package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"panelnest/internal/model"
)

func TestLoadInitializesMissingStockMapWithoutDroppingJobs(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "ledger.json")
	content := `{"version":1,"jobs":{"job-1":{"id":"job-1","source_panel_id":"panel-1","kerf_mm":0,"pieces":[],"status":"draft"}},"receipts":{}}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	value, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if value.Stock == nil || value.Jobs["job-1"].ID != "job-1" {
		t.Fatalf("normalization lost data: %#v", value)
	}
	if err := AddPanel(&value, model.Panel{ID: "panel-1", Material: "MDF", Width: 100, Height: 100}); err != nil {
		t.Fatalf("add after normalization: %v", err)
	}
	if value.Jobs["job-1"].ID != "job-1" || value.Stock["panel-1"].ID != "panel-1" {
		t.Fatal("existing and inserted records were not retained")
	}
}

func TestCommitPersistsReceiptConsumesOnceAndRegistersOffcuts(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "ledger.json")
	value := model.NewLedger()
	if err := AddPanel(&value, model.Panel{ID: "panel-1", Material: "plywood", Width: 100, Height: 100}); err != nil {
		t.Fatalf("add panel: %v", err)
	}
	if err := OpenJob(&value, model.Job{ID: "job-1", SourcePanelID: "panel-1", Kerf: 2}); err != nil {
		t.Fatalf("open job: %v", err)
	}
	if err := AddPiece(&value, "job-1", model.PieceRequirement{Label: "door", Quantity: 1, Width: 40, Height: 30}); err != nil {
		t.Fatalf("add piece: %v", err)
	}
	if err := Save(path, &value); err != nil {
		t.Fatalf("save draft: %v", err)
	}
	value, err := Load(path)
	if err != nil {
		t.Fatalf("reload draft: %v", err)
	}
	receipt, err := Commit(&value, "job-1")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(receipt.Placements) != 1 || len(receipt.OffcutPanelIDs) != 2 {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
	if value.Stock["panel-1"].Status != model.Consumed || value.Jobs["job-1"].Status != model.Committed {
		t.Fatalf("commit did not transition source and job: %#v %#v", value.Stock["panel-1"], value.Jobs["job-1"])
	}
	offcutsBefore := len(value.Stock)
	if _, err := Commit(&value, "job-1"); err == nil || !strings.Contains(err.Error(), "already committed") {
		t.Fatalf("second commit should fail: %v", err)
	}
	if len(value.Stock) != offcutsBefore || len(value.Receipts) != 1 {
		t.Fatal("second commit changed inventory or receipts")
	}
	if err := Save(path, &value); err != nil {
		t.Fatalf("save committed ledger: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload committed ledger: %v", err)
	}
	if _, exists := reloaded.Receipts[receipt.ID]; !exists {
		t.Fatalf("receipt missing after reload: %#v", reloaded.Receipts)
	}
}

func TestFailedReplacementRemovesTemporaryFileAndLeavesExistingLedgerReadable(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "ledger.json")
	value := model.NewLedger()
	if err := AddPanel(&value, model.Panel{ID: "panel-1", Material: "plywood", Width: 20, Height: 20}); err != nil {
		t.Fatalf("add panel: %v", err)
	}
	if err := Save(path, &value); err != nil {
		t.Fatalf("initial save: %v", err)
	}
	failedTarget := filepath.Join(directory, "target-directory")
	if err := os.Mkdir(failedTarget, 0o755); err != nil {
		t.Fatalf("make failed target: %v", err)
	}
	if err := Save(failedTarget, &value); err == nil {
		t.Fatal("save to directory should fail during replacement")
	}
	matches, err := filepath.Glob(filepath.Join(directory, ".target-directory.tmp-*"))
	if err != nil {
		t.Fatalf("glob temporary files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files were not removed: %v", matches)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("existing ledger became unreadable: %v", err)
	}
	if _, exists := reloaded.Stock["panel-1"]; !exists {
		t.Fatal("existing ledger lost its panel")
	}
}
