package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGrainConstraintPreservesOmissionAndExplicitEmptyValue(t *testing.T) {
	var ledger Ledger
	input := `{
  "version": 1,
  "stock": {},
  "jobs": {
    "job-1": {
      "id": "job-1",
      "source_panel_id": "panel-1",
      "kerf_mm": 0,
      "status": "draft",
      "pieces": [
        {"label": "omitted", "quantity": 1, "width_mm": 10, "height_mm": 10},
        {"label": "explicit-empty", "quantity": 1, "width_mm": 10, "height_mm": 10, "grain": ""}
      ]
    }
  },
  "receipts": {}
}`
	if err := json.Unmarshal([]byte(input), &ledger); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	pieces := ledger.Jobs["job-1"].Pieces
	if pieces[0].Grain != nil {
		t.Fatal("omitted grain should remain nil")
	}
	if pieces[1].Grain == nil || *pieces[1].Grain != "" {
		t.Fatalf("explicit empty grain should remain a non-nil empty value, got %#v", pieces[1].Grain)
	}
	if err := ledger.Validate(); err == nil || !strings.Contains(err.Error(), "invalid grain") {
		t.Fatalf("expected explicit empty grain to be rejected, got %v", err)
	}
	encoded, err := json.Marshal(ledger.Jobs["job-1"].Pieces)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"grain":""`) {
		t.Fatalf("explicit empty grain was not retained during encoding: %s", encoded)
	}
}
