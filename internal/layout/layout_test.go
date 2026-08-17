package layout

import (
	"testing"

	"panelnest/internal/model"
)

func TestPreviewExpandsQuantitiesAndKeepsFragmentsDistinct(t *testing.T) {
	job := model.Job{
		ID:            "job-1",
		SourcePanelID: "panel-1",
		Kerf:          1,
		Status:        model.Draft,
		Pieces: []model.PieceRequirement{{
			Label: "shelf", Quantity: 2, Width: 4, Height: 4,
		}},
	}
	preview, err := PreviewJob(job, model.Panel{ID: "panel-1", Material: "plywood", Width: 10, Height: 10, Status: model.Available, Kind: model.StockPanel})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(preview.Placements) != 2 || len(preview.Unplaced) != 0 {
		t.Fatalf("unexpected preview result: %#v", preview)
	}
	if preview.Placements[0].X != 0 || preview.Placements[0].Y != 0 || preview.Placements[0].FootprintW != 5 || preview.Placements[0].FootprintH != 5 {
		t.Fatalf("unexpected first placement: %#v", preview.Placements[0])
	}
	if preview.Placements[1].X != 5 || preview.Placements[1].Y != 0 {
		t.Fatalf("second placement did not use the first replacement fragment: %#v", preview.Placements[1])
	}
	if len(preview.FreeRectangles) != 1 || preview.FreeRectangles[0] != (Rectangle{X: 0, Y: 5, Width: 10, Height: 5}) {
		t.Fatalf("free fragments were skipped or reused: %#v", preview.FreeRectangles)
	}
	assertNoOverlap(t, preview.Placements)
}

func TestPreviewGrainControlsRotation(t *testing.T) {
	base := model.Job{ID: "job-1", SourcePanelID: "panel-1", Status: model.Draft, Pieces: []model.PieceRequirement{{Label: "grain-piece", Quantity: 1, Width: 6, Height: 4}}}
	rotated, err := PreviewJob(base, model.Panel{ID: "panel-1", Material: "board", Width: 4, Height: 6, Status: model.Available, Kind: model.StockPanel})
	if err != nil {
		t.Fatalf("unconstrained preview: %v", err)
	}
	if len(rotated.Placements) != 1 || !rotated.Placements[0].Rotated {
		t.Fatalf("unconstrained piece should rotate: %#v", rotated)
	}
	lengthwise := base
	constraint := model.GrainLengthwise
	lengthwise.Pieces[0].Grain = &constraint
	fixed, err := PreviewJob(lengthwise, model.Panel{ID: "panel-1", Material: "board", Width: 4, Height: 6, Status: model.Available, Kind: model.StockPanel})
	if err != nil {
		t.Fatalf("lengthwise preview: %v", err)
	}
	if len(fixed.Placements) != 0 || len(fixed.Unplaced) != 1 {
		t.Fatalf("lengthwise piece should remain unplaced: %#v", fixed)
	}
}

func TestPreviewReportsEveryUnplacedQuantity(t *testing.T) {
	job := model.Job{ID: "job-1", SourcePanelID: "panel-1", Status: model.Draft, Pieces: []model.PieceRequirement{{Label: "large", Quantity: 3, Width: 4, Height: 4}}}
	preview, err := PreviewJob(job, model.Panel{ID: "panel-1", Material: "board", Width: 5, Height: 5, Status: model.Available, Kind: model.StockPanel})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(preview.Placements) != 1 || len(preview.Unplaced) != 2 {
		t.Fatalf("expected one placement and two unplaced pieces: %#v", preview)
	}
	if preview.Unplaced[0].QuantityIndex != 2 || preview.Unplaced[1].QuantityIndex != 3 {
		t.Fatalf("unplaced quantities were not stable: %#v", preview.Unplaced)
	}
}

func assertNoOverlap(t *testing.T, placements []model.Placement) {
	t.Helper()
	for i, first := range placements {
		for j := i + 1; j < len(placements); j++ {
			second := placements[j]
			if first.X < second.X+second.FootprintW && second.X < first.X+first.FootprintW && first.Y < second.Y+second.FootprintH && second.Y < first.Y+first.FootprintH {
				t.Fatalf("placements overlap: %#v and %#v", first, second)
			}
		}
	}
}
