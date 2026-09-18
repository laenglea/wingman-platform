package harness

import (
	"context"
	"crypto/rand"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

//go:embed testdata/project/*
var projectFiles embed.FS

const ProjectTestCommand = "python3 -m unittest -v"

// ProjectFixture is shared by the CLI suites so they repair the same bugs and
// are judged by the same independent tests, regardless of their tool protocol.
type ProjectFixture struct {
	files map[string][]byte
}

func NewProjectFixture(t *testing.T) ProjectFixture {
	t.Helper()
	orderID := "order-" + rand.Text()
	p := ProjectFixture{files: map[string][]byte{
		"order.json":    fmt.Appendf(nil, `{"order_id":%q,"items":[{"quantity":3,"unit_price_cents":250,"discount_cents":25},{"quantity":2,"unit_price_cents":125},{"quantity":9,"unit_price_cents":900,"cancelled":true}]}`, orderID),
		"expected.json": fmt.Appendf(nil, `{"order_id":%q,"item_count":5,"subtotal_cents":925}`, orderID),
	}}
	for _, name := range []string{"pricing.py", "invoice.py", "test_invoice.py"} {
		data, err := projectFiles.ReadFile("testdata/project/" + name)
		if err != nil {
			t.Fatal(err)
		}
		p.files[name] = data
	}
	return p
}

func (p ProjectFixture) Setup(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Fatal("python3 is required for the project repair scenario")
	}
	for name, data := range p.files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// Verify checks the protected fixtures and runs the tests independently of
// the CLI's claims. Keep its output next to the other retained artifacts.
func (p ProjectFixture) Verify(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"test_invoice.py", "order.json", "expected.json"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != string(p.files[name]) {
			t.Fatalf("protected fixture %s changed: %v", name, err)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "-m", "unittest", "-v")
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "PYTHONDONTWRITEBYTECODE=1"}
	output, runErr := cmd.CombinedOutput()
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "verification.log"), output, 0600); err != nil {
		t.Fatal(err)
	}
	if runErr != nil || !strings.Contains(string(output), "Ran 7 tests") || !strings.Contains(string(output), "\nOK") {
		t.Fatalf("independent project tests failed: %v\n%s", runErr, output)
	}
}
