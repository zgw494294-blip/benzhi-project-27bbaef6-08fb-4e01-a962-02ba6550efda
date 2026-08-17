package cli

import (
	"bytes"
	"strings"
	"testing"

	"panelnest/internal/ledger"
)

func TestRunWorkflowUsesReloadableLedgerAndDistinguishesGrainInput(t *testing.T) {
	path := t.TempDir() + "/panelnest.json"
	var output bytes.Buffer
	run := func(args ...string) error {
		output.Reset()
		return Run(append(args, "--ledger", path), &output, &output)
	}
	if err := run("stock-add", "--id", "panel-1", "--material", "oak", "--width", "100", "--height", "100"); err != nil {
		t.Fatalf("stock-add: %v", err)
	}
	if err := run("job-open", "--id", "job-1", "--panel", "panel-1", "--kerf", "1"); err != nil {
		t.Fatalf("job-open: %v", err)
	}
	if err := run("piece-add", "--job", "job-1", "--label", "free", "--quantity", "1", "--width", "20", "--height", "20"); err != nil {
		t.Fatalf("piece-add omitted grain: %v", err)
	}
	if err := run("piece-add", "--job", "job-1", "--label", "invalid", "--quantity", "1", "--width", "20", "--height", "20", "--grain", ""); err == nil || !strings.Contains(err.Error(), "invalid grain") {
		t.Fatalf("explicit empty grain should fail: %v", err)
	}
	value, err := ledger.Load(path)
	if err != nil {
		t.Fatalf("load after failed piece: %v", err)
	}
	if len(value.Jobs["job-1"].Pieces) != 1 || value.Jobs["job-1"].Pieces[0].Grain != nil {
		t.Fatalf("failed grain input changed persisted job: %#v", value.Jobs["job-1"])
	}
	if err := run("preview", "--job", "job-1"); err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !strings.Contains(output.String(), "placed=1 unplaced=0") {
		t.Fatalf("preview output missing placement summary: %s", output.String())
	}
	if err := run("commit", "--job", "job-1"); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := run("show", "--job", "job-1"); err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(output.String(), "receipt: id=receipt-job-1") {
		t.Fatalf("show output missing receipt: %s", output.String())
	}
}
