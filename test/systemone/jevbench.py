#!/usr/bin/env python3
"""Run the upstream JevBench public tasks through Wingman's System One API.

Start Wingman with task server, clone the benchmark, then run:
  python3 test/systemone/jevbench.py --benchmark /path/to/jevbench --output /path/to/new-run

Uses upstream task loading, scoring, metrics, durable evidence and budget ledger.
Only the probability-source metadata is adapted to describe Wingman's backends.
"""

import argparse
import datetime
import hashlib
import json
import platform
import subprocess
import sys
from pathlib import Path


MODELS = {
    "nimble": "native",
    "jev-1.13": "native",
    "text-embedding-3-large": "embedding_similarity",
    "gpt-6-luna": "verbalized",
}


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2, sort_keys=True, allow_nan=False) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--benchmark", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--endpoint", default="http://localhost:4242")
    parser.add_argument("--models", nargs="+", choices=MODELS, default=list(MODELS))
    parser.add_argument("--gpt-reasoning", choices=("default", "none"), help="record the running server's setting; does not change requests")
    parser.add_argument("--smoke", action="store_true", help="one example per question type, excluded from the full run")
    args = parser.parse_args()

    root = args.benchmark.resolve()
    sys.path.insert(0, str(root))
    from jevbench.adapters.typesafe import TypeSafeAdapter
    from jevbench.budget import Ledger
    from jevbench.runner import Runner
    from jevbench.summarize import summarize
    from jevbench.tasks import dataset_hash, load_jsonl

    class WingmanAdapter(TypeSafeAdapter):
        def run(self, task):
            result = super().run(task)
            result.probs_source = MODELS[self.model]
            return result

    cohorts = {
        name: load_jsonl(str(root / "datasets" / "public" / (name + ".jsonl")))
        for name in ("original", "easy", "hard")
    }
    tasks = [task for cohort in cohorts.values() for task in cohort]
    if args.smoke:
        tasks = [
            next(task for task in tasks if task.question["type"] == kind)
            for kind in ("noul", "choice", "score")
        ]
    if len({task.id for task in tasks}) != len(tasks):
        raise ValueError("duplicate benchmark task IDs")

    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    source = root / "datasets" / "public"
    write_json(output / "manifest.json", {
        "benchmark": "https://github.com/fstandhartinger/jevbench",
        "benchmark_commit": subprocess.check_output(["git", "-C", str(root), "rev-parse", "HEAD"], text=True).strip(),
        "wingman_commit": subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(),
        "endpoint": args.endpoint.rstrip("/") + "/v1/systemone",
        "requested_models": args.models,
        "probability_sources": {model: MODELS[model] for model in args.models},
        "reasoning": {model: (args.gpt_reasoning or "not recorded") if model == "gpt-6-luna" else "not applicable" for model in args.models},
        "decision_adapter_sha256": hashlib.sha256(Path("pkg/provider/adapter/decider/completer.go").read_bytes()).hexdigest(),
        "dataset_hash": dataset_hash(tasks),
        "dataset_file_hashes": {
            name: hashlib.sha256((source / (name + ".jsonl")).read_bytes()).hexdigest()
            for name in cohorts
        },
        "n_planned_per_model": len(tasks),
        "scope": "smoke" if args.smoke else "public datasets only; no private or sealed items",
        "official_leaderboard_score": False,
        "concurrency": 1,
        "latency": "observed wall time through Wingman; no production-load adjustment",
        "wingman_retries": "configured provider defaults; benchmark runner adds no retries",
        "pricing": "not measured; usage recorded; paid calls reserve $0.02 each in upstream ledger, shared cap $15",
        "platform": platform.platform(),
        "started_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    })
    ledger = Ledger(str(output / "ledger.jsonl"), cap_usd=15)
    all_summaries = {}
    incomplete = False
    for model in args.models:
        model_dir = output / model
        model_dir.mkdir()
        adapter = WingmanAdapter(endpoint=args.endpoint, model=model, key_env="", timeout_s=180)
        adapter.cost_basis = "local_compute_not_priced" if model == "nimble" else "provider_tariff_not_measured"
        reserve = 0 if model == "nimble" else 0.02
        runner = Runner(adapter, ledger, raw_dir=str(model_dir / "raw"), default_reserve_usd=reserve)
        print(f"START {model}: {len(tasks)} tasks", flush=True)
        records = runner.run_all(tasks, results_path=str(model_dir / "results.jsonl"))
        summary = summarize(tasks, records)
        summary["requested_model"] = model
        summary["usage_totals"] = {
            key: sum(record.get("usage", {}).get(key, 0) for record in records)
            for key in ("input_tokens", "output_tokens")
        }
        summary["errors"] = [
            {"task_id": record["task_id"], "status": record["status_code"], "error": record["error"]}
            for record in records if not record["ok"]
        ]
        if not args.smoke:
            summary["cohorts"] = {
                name: summarize(cohort, [record for record in records if record["task_id"] in {task.id for task in cohort}])
                for name, cohort in cohorts.items()
            }
        write_json(model_dir / "summary.json", summary)
        all_summaries[model] = summary
        write_json(output / "summary.json", all_summaries)
        print(f"DONE {model}: {summary['n_attempted']}/{len(tasks)} attempted, "
              f"accuracy={summary['accuracy']}, valid={summary['schema_validity']}, "
              f"p50={summary['latency']['p50_s']}s", flush=True)
        incomplete |= not summary["complete"]
    manifest = json.loads((output / "manifest.json").read_text())
    manifest["finished_utc"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
    manifest["complete"] = not incomplete
    write_json(output / "manifest.json", manifest)
    return 1 if incomplete else 0


if __name__ == "__main__":
    sys.exit(main())
