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

	tasks := placementTasks(job)
	for _, task := range tasks {
		if _, ok := taskFootprints(task, job.Kerf); !ok {
			return Preview{}, fmt.Errorf("piece %q footprint exceeds integer range", task.label)
		}
	}

	result := Preview{JobID: job.ID, PanelID: panel.ID}
	initialFree := []Rectangle{{Width: panel.Width, Height: panel.Height}}

	if placements, free, ok := placeAll(tasks, job.Kerf, initialFree); ok {
		result.Placements = placements
		result.FreeRectangles = free
		return result, nil
	}

	// No complete packing exists. Fall back to the greedy best-effort result so
	// the preview still reports which pieces could not be placed.
	free := initialFree
	for _, task := range tasks {
		placement, nextFree, placed := placeGreedy(task, job.Kerf, free)
		if !placed {
			result.Unplaced = append(result.Unplaced, Unplaced{
				PieceIndex:    task.pieceIndex,
				Label:         task.label,
				QuantityIndex: task.quantityIndex,
				Reason:        "no remaining rectangle can fit the piece with its kerf",
			})
			continue
		}
		result.Placements = append(result.Placements, placement)
		free = nextFree
	}
	result.FreeRectangles = free
	return result, nil
}

type placementTask struct {
	pieceIndex    int
	label         string
	quantityIndex int
	requirement   model.PieceRequirement
}

func placementTasks(job model.Job) []placementTask {
	tasks := make([]placementTask, 0)
	for pieceIndex, requirement := range job.Pieces {
		for quantityIndex := 1; quantityIndex <= requirement.Quantity; quantityIndex++ {
			tasks = append(tasks, placementTask{
				pieceIndex:    pieceIndex + 1,
				label:         requirement.Label,
				quantityIndex: quantityIndex,
				requirement:   requirement,
			})
		}
	}
	return tasks
}

// placementOption is an orientation together with its kerf-aware footprint.
type placementOption struct {
	width      int
	height     int
	footprintW int
	footprintH int
	rotated    bool
}

// taskFootprints returns the kerf-aware footprint for each viable orientation
// of a task. The second return is false when the footprint would overflow the
// integer range.
func taskFootprints(task placementTask, kerf int) ([]placementOption, bool) {
	options := make([]placementOption, 0, 2)
	for _, orientation := range orientations(task.requirement) {
		footprintWidth, ok := addKerf(orientation.width, kerf)
		if !ok {
			return nil, false
		}
		footprintHeight, ok := addKerf(orientation.height, kerf)
		if !ok {
			return nil, false
		}
		options = append(options, placementOption{
			width:      orientation.width,
			height:     orientation.height,
			footprintW: footprintWidth,
			footprintH: footprintHeight,
			rotated:    orientation.rotated,
		})
	}
	return options, true
}

// placeAll performs a depth-first search over the placement tasks, trying free
// rectangles and orientations in the same canonical order as the greedy placer.
// It returns the first complete packing it finds. The search order guarantees
// that when the greedy strategy succeeds the result matches it exactly, while
// still recovering packings that require rotating an earlier piece.
func placeAll(tasks []placementTask, kerf int, free []Rectangle) ([]model.Placement, []Rectangle, bool) {
	if len(tasks) == 0 {
		return nil, append([]Rectangle(nil), free...), true
	}
	task := tasks[0]
	options, ok := taskFootprints(task, kerf)
	if !ok {
		return nil, nil, false
	}
	for freeIndex, candidate := range free {
		for _, option := range options {
			if option.footprintW > candidate.Width || option.footprintH > candidate.Height {
				continue
			}
			placement := model.Placement{
				PieceIndex:    task.pieceIndex,
				Label:         task.label,
				QuantityIndex: task.quantityIndex,
				X:             candidate.X,
				Y:             candidate.Y,
				Width:         option.width,
				Height:        option.height,
				FootprintW:    option.footprintW,
				FootprintH:    option.footprintH,
				Rotated:       option.rotated,
			}
			nextFree := replaceFreeRectangle(free, freeIndex, candidate, option.footprintW, option.footprintH)
			if placements, remaining, ok := placeAll(tasks[1:], kerf, nextFree); ok {
				return append([]model.Placement{placement}, placements...), remaining, true
			}
		}
	}
	return nil, nil, false
}

// placeGreedy places a single task using the first free rectangle and
// orientation that fits, mirroring the original best-effort behavior used when
// no complete packing exists.
func placeGreedy(task placementTask, kerf int, free []Rectangle) (model.Placement, []Rectangle, bool) {
	options, ok := taskFootprints(task, kerf)
	if !ok {
		return model.Placement{}, free, false
	}
	for freeIndex, candidate := range free {
		for _, option := range options {
			if option.footprintW > candidate.Width || option.footprintH > candidate.Height {
				continue
			}
			placement := model.Placement{
				PieceIndex:    task.pieceIndex,
				Label:         task.label,
				QuantityIndex: task.quantityIndex,
				X:             candidate.X,
				Y:             candidate.Y,
				Width:         option.width,
				Height:        option.height,
				FootprintW:    option.footprintW,
				FootprintH:    option.footprintH,
				Rotated:       option.rotated,
			}
			nextFree := replaceFreeRectangle(free, freeIndex, candidate, option.footprintW, option.footprintH)
			return placement, nextFree, true
		}
	}
	return model.Placement{}, free, false
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
