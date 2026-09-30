#!/usr/bin/env python3
"""Render saved JevBench summaries as a standalone HTML report and CSV."""

import argparse
import csv
import html
import json
import os
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("run", type=Path)
    parser.add_argument("--baseline", type=Path, help="reuse other models from an earlier run of the same dataset")
    args = parser.parse_args()
    root = args.run
    manifest = json.loads((root / "manifest.json").read_text())
    summaries = json.loads((root / "summary.json").read_text())
    sources = {model: (root, manifest) for model in summaries}
    if args.baseline:
        baseline_manifest = json.loads((args.baseline / "manifest.json").read_text())
        for key in ("dataset_hash", "benchmark_commit", "scope", "n_planned_per_model"):
            if baseline_manifest[key] != manifest[key]:
                raise ValueError(f"baseline differs in {key}")
        baseline = json.loads((args.baseline / "summary.json").read_text())
        for model, summary in baseline.items():
            if model not in summaries:
                summaries[model] = summary
                sources[model] = (args.baseline, baseline_manifest)
    rows = []
    for model, summary in summaries.items():
        rows.append({
            "model": model,
            "reasoning": sources[model][1].get("reasoning", {}).get(model, "default (omitted)" if model == "gpt-6-luna" else "not applicable"),
            "source": ", ".join(summary["probability_sources"]),
            "attempted": summary["n_attempted"],
            "correct": summary["n_correct"],
            "accuracy": summary["accuracy"],
            "original_accuracy": summary["cohorts"]["original"]["accuracy"],
            "easy_accuracy": summary["cohorts"]["easy"]["accuracy"],
            "hard_accuracy": summary["cohorts"]["hard"]["accuracy"],
            "validity": summary["schema_validity"],
            "p50_seconds": summary["latency"]["p50_s"],
            "p95_seconds": summary["latency"]["p95_s"],
            "brier": summary["brier_mean"],
            "ece": summary["ece"]["ece"],
            "input_tokens": summary["usage_totals"]["input_tokens"],
            "output_tokens": summary["usage_totals"]["output_tokens"],
            "api_failures": len(summary["errors"]),
            "resolved_models": ", ".join(summary["model_identities"]),
            "source_manifest": os.path.relpath(sources[model][0] / "manifest.json", root),
        })
    rows.sort(key=lambda row: row["accuracy"], reverse=True)
    with (root / "comparison.csv").open("w", newline="") as stream:
        writer = csv.DictWriter(stream, fieldnames=list(rows[0]))
        writer.writeheader()
        writer.writerows(rows)

    columns = [
        ("model", "Model"), ("source", "Probability source"), ("reasoning", "Reasoning"),
        ("accuracy", "Accuracy"), ("original_accuracy", "Original (72)"),
        ("easy_accuracy", "Easy (48)"), ("hard_accuracy", "Hard (111)"),
        ("validity", "Valid"), ("p50_seconds", "Median seconds"),
        ("p95_seconds", "p95 seconds"), ("brier", "Brier ↓"), ("ece", "ECE ↓"),
        ("input_tokens", "Input tokens"), ("output_tokens", "Output tokens"),
        ("api_failures", "API failures"),
    ]
    percentages = {"accuracy", "original_accuracy", "easy_accuracy", "hard_accuracy", "validity"}
    table_rows = []
    for row in rows:
        cells = []
        for key, _ in columns:
            value = row[key]
            if key in percentages:
                label = f"{100 * value:.1f}%"
            elif key in {"p50_seconds", "p95_seconds", "brier", "ece"}:
                label = f"{value:.3f}"
            elif isinstance(value, int):
                label = f"{value:,}"
            else:
                label = str(value)
            cells.append(f'<td data-value="{html.escape(str(value), quote=True)}">{html.escape(label)}</td>')
        table_rows.append("<tr>" + "".join(cells) + "</tr>")
    headers = "".join(f'<th onclick="sortRows({i})">{html.escape(label)}</th>' for i, (_, label) in enumerate(columns))
    error_sections = []
    for model, summary in summaries.items():
        source_root, source_manifest = sources[model]
        evidence = html.escape(os.path.relpath(source_root / model / "results.jsonl", root), quote=True)
        source_link = html.escape(os.path.relpath(source_root / "manifest.json", root), quote=True)
        error_sections.append(
            "<details><summary>" + html.escape(model) + " · resolved as " +
            html.escape(", ".join(summary["model_identities"])) + "</summary><p>" +
            '<a href="' + evidence + '">Per-item evidence</a> · <a href="' + source_link + '">Source manifest</a>' +
            " · Wingman commit: <code>" + html.escape(source_manifest["wingman_commit"]) + "</code></p><pre>" +
            html.escape(json.dumps(summary["errors"], indent=2)) + "</pre></details>"
        )
    dataset_hash = html.escape(manifest["dataset_hash"])
    benchmark_commit = html.escape(manifest["benchmark_commit"])
    wingman_commit = html.escape(manifest["wingman_commit"])
    report = """<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Wingman · JevBench public comparison</title>
<style>
body{font:15px system-ui,sans-serif;margin:36px auto;max-width:1550px;padding:0 24px;color:#172334;background:#f7f9fc}
h1{font-size:30px;letter-spacing:-.5px}p{max-width:1000px;line-height:1.65}
.scroll{overflow:auto;background:white;border:1px solid #dbe2ec;border-radius:12px}
table{border-collapse:collapse;white-space:nowrap;width:100%}th,td{text-align:right;padding:15px 13px;border-bottom:1px solid #edf0f5}
th{font-size:12px;color:#596a82;cursor:pointer}td:first-child,th:first-child{text-align:left;font-weight:600}
td:nth-child(2),th:nth-child(2){text-align:left}tr:last-child td{border-bottom:0}tbody tr:hover{background:#f1f5fb}
code,pre{font:12px ui-monospace,monospace}pre{overflow:auto;background:#eef2f7;padding:14px;border-radius:8px}
details{margin:12px 0}a{color:#3156c9}.note{color:#536079}
</style>
<h1>Wingman · JevBench public comparison</h1>
<p>231 public decisions per model: 72 original, 48 easy, and 111 hard. All models ran serially through
<code>/v1/systemone</code> on the same machine and task order. This is a comparison of Wingman deployments,
not an official JevBench leaderboard score; private and sealed decisions are unavailable.</p>
<div class="scroll"><table id="comparison"><thead><tr>""" + headers + "</tr></thead><tbody>" + "".join(table_rows) + """</tbody></table></div>
<p class="note">Each model's source run is linked below. The reasoning column records the GPT setting.
Click a column header to sort. Accuracy uses JevBench's argmax scoring over the exact label set;
score questions also use argmax for headline accuracy. Invalid answers count as incorrect.
Calibration uses the returned option probabilities. The native models, embedding similarities,
and GPT's stated estimates have different probability sources.
Latency is observed wall time including Wingman and the upstream provider, with no production-load adjustment.
Three smoke requests per model preceded the scored runs.</p>
<p class="note">Costs were not measured. Token counts are retained. Dollar entries in
<code>ledger.jsonl</code> are benchmark reservations for unknown prices and do not represent measured billing.</p>
<p><a href="comparison.csv">Comparison CSV</a> · <a href="comparison-summary.json">Full comparison metrics JSON</a> ·
<a href="manifest.json">Reproduction manifest</a> ·
<a href="https://github.com/fstandhartinger/jevbench">Upstream benchmark</a></p>
<h2>Model identities and API errors</h2>""" + "".join(error_sections) + """
<h2>Provenance</h2><pre>Benchmark commit: """ + benchmark_commit + """
Wingman commit: """ + wingman_commit + """
Public dataset hash: """ + dataset_hash + """</pre>
<script>
let direction={};
function sortRows(index){
 const body=document.querySelector('#comparison tbody'), rows=[...body.rows];
 direction[index]=!(direction[index]??false);
 rows.sort((a,b)=>{
  const x=a.cells[index].dataset.value,y=b.cells[index].dataset.value;
  const result=(x!==''&&y!==''&&Number.isFinite(+x)&&Number.isFinite(+y))?+x-+y:x.localeCompare(y);
  return direction[index]?-result:result;
 });
 rows.forEach(row=>body.appendChild(row));
}
</script></html>
"""
    (root / "report.html").write_text(report)
    (root / "comparison-summary.json").write_text(json.dumps({
        "summaries": summaries,
        "source_manifests": {row["model"]: row["source_manifest"] for row in rows},
    }, indent=2, sort_keys=True, allow_nan=False) + "\n")
    print(root / "report.html")
    for row in rows:
        print(f"{row['model']}: {row['correct']}/{row['attempted']} correct, "
              f"{100 * row['accuracy']:.1f}%, hard {100 * row['hard_accuracy']:.1f}%, "
              f"median {row['p50_seconds']:.3f}s, p95 {row['p95_seconds']:.3f}s, "
              f"failures {row['api_failures']}")


if __name__ == "__main__":
    main()
