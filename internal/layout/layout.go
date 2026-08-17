package layout

import (
	"fmt"
	"math"

	"panelnest/internal/model"
)

type Rectangle struct {
	X      int `json:"x_mm"`
	Y      int `json:"y_mm"`
	Width  int `json:"width_mm"`
	Height int `json:"height_mm"`
}

type Unplaced struct {
	PieceIndex    int    `json:"piece_index"`
	Label         string `json:"label"`
	QuantityIndex int    `json:"quantity_index"`
	Reason        string `json:"reason"`
}

type Preview struct {
	JobID          string            `json:"job_id"`
	PanelID        string            `json:"panel_id"`
	Placements     []model.Placement `json:"placements"`
	Unplaced       []Unplaced        `json:"unplaced"`
	FreeRectangles []Rectangle       `json:"free_rectangles"`
}

type orientation struct {
	width   int
	height  int
	rotated bool
}

func PreviewJob(job model.Job, panel model.Panel) (Preview, error) {
	if err := job.Validate(); err != nil {
		return Preview{}, err
	}
	if err := panel.Validate(); err != nil {
		return Preview{}, err
	}
	free := []Rectangle{{Width: panel.Width, Height: panel.Height}}
	result := Preview{JobID: job.ID, PanelID: panel.ID}

	for pieceIndex, requirement := range job.Pieces {
		for quantityIndex := 1; quantityIndex <= requirement.Quantity; quantityIndex++ {
			placed := false
			for freeIndex, candidate := range free {
				for _, orientation := range orientations(requirement) {
					footprintWidth, ok := addKerf(orientation.width, job.Kerf)
					if !ok {
						return Preview{}, fmt.Errorf("piece %q footprint exceeds integer range", requirement.Label)
					}
					footprintHeight, ok := addKerf(orientation.height, job.Kerf)
					if !ok {
						return Preview{}, fmt.Errorf("piece %q footprint exceeds integer range", requirement.Label)
					}
					if footprintWidth > candidate.Width || footprintHeight > candidate.Height {
						continue
					}
					result.Placements = append(result.Placements, model.Placement{
						PieceIndex:    pieceIndex + 1,
						Label:         requirement.Label,
						QuantityIndex: quantityIndex,
						X:             candidate.X,
						Y:             candidate.Y,
						Width:         orientation.width,
						Height:        orientation.height,
						FootprintW:    footprintWidth,
						FootprintH:    footprintHeight,
						Rotated:       orientation.rotated,
					})
					free = replaceFreeRectangle(free, freeIndex, candidate, footprintWidth, footprintHeight)
					placed = true
					break
				}
				if placed {
					break
				}
			}
			if !placed {
				result.Unplaced = append(result.Unplaced, Unplaced{
					PieceIndex:    pieceIndex + 1,
					Label:         requirement.Label,
					QuantityIndex: quantityIndex,
					Reason:        "no remaining rectangle can fit the piece with its kerf",
				})
			}
		}
	}
	result.FreeRectangles = free
	return result, nil
}

func orientations(requirement model.PieceRequirement) []orientation {
	if requirement.Grain == nil {
		return distinctOrientations(orientation{width: requirement.Width, height: requirement.Height}, orientation{width: requirement.Height, height: requirement.Width, rotated: true})
	}
	if *requirement.Grain == model.GrainLengthwise {
		return []orientation{{width: requirement.Width, height: requirement.Height}}
	}
	return []orientation{{width: requirement.Height, height: requirement.Width, rotated: true}}
}

func distinctOrientations(first, second orientation) []orientation {
	if first.width == second.width && first.height == second.height {
		return []orientation{first}
	}
	return []orientation{first, second}
}

func replaceFreeRectangle(free []Rectangle, index int, source Rectangle, usedWidth, usedHeight int) []Rectangle {
	fragments := make([]Rectangle, 0, 2)
	if remainingWidth := source.Width - usedWidth; remainingWidth > 0 {
		fragments = append(fragments, Rectangle{X: source.X + usedWidth, Y: source.Y, Width: remainingWidth, Height: usedHeight})
	}
	if remainingHeight := source.Height - usedHeight; remainingHeight > 0 {
		fragments = append(fragments, Rectangle{X: source.X, Y: source.Y + usedHeight, Width: source.Width, Height: remainingHeight})
	}

	next := make([]Rectangle, 0, len(free)-1+len(fragments))
	next = append(next, free[:index]...)
	next = append(next, fragments...)
	next = append(next, free[index+1:]...)
	return next
}

func addKerf(value, kerf int) (int, bool) {
	if kerf > math.MaxInt-value {
		return 0, false
	}
	return value + kerf, true
}
