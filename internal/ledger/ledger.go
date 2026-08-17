package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"panelnest/internal/layout"
	"panelnest/internal/model"
)

func Load(path string) (model.Ledger, error) {
	if strings.TrimSpace(path) == "" {
		return model.Ledger{}, errors.New("ledger path is required")
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return model.NewLedger(), nil
	}
	if err != nil {
		return model.Ledger{}, fmt.Errorf("read ledger: %w", err)
	}
	var result model.Ledger
	if err := json.Unmarshal(data, &result); err != nil {
		return model.Ledger{}, fmt.Errorf("decode ledger: %w", err)
	}
	result.Normalize()
	if err := result.Validate(); err != nil {
		return model.Ledger{}, fmt.Errorf("validate ledger: %w", err)
	}
	return result, nil
}

func Save(path string, value *model.Ledger) error {
	if value == nil {
		return errors.New("ledger is required")
	}
	value.Normalize()
	if err := value.Validate(); err != nil {
		return fmt.Errorf("validate ledger: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode ledger: %w", err)
	}
	data = append(data, '\n')

	directory := filepath.Dir(path)
	base := filepath.Base(path)
	temporary, err := os.CreateTemp(directory, "."+base+".tmp-*")
	if err != nil {
		return fmt.Errorf("create ledger temporary file: %w", err)
	}
	temporaryName := temporary.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temporary.Close()
		}
		_ = os.Remove(temporaryName)
	}()

	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write ledger temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync ledger temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		closed = true
		return fmt.Errorf("close ledger temporary file: %w", err)
	}
	closed = true
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace ledger: %w", err)
	}
	return nil
}

func AddPanel(value *model.Ledger, panel model.Panel) error {
	if value == nil {
		return errors.New("ledger is required")
	}
	value.Normalize()
	panel.ID = strings.TrimSpace(panel.ID)
	panel.Material = strings.TrimSpace(panel.Material)
	if panel.Status == "" {
		panel.Status = model.Available
	}
	if panel.Kind == "" {
		panel.Kind = model.StockPanel
	}
	if err := panel.Validate(); err != nil {
		return err
	}
	if _, exists := value.Stock[panel.ID]; exists {
		return fmt.Errorf("panel %q already exists", panel.ID)
	}
	value.Stock[panel.ID] = panel
	return nil
}

func OpenJob(value *model.Ledger, job model.Job) error {
	if value == nil {
		return errors.New("ledger is required")
	}
	value.Normalize()
	job.ID = strings.TrimSpace(job.ID)
	job.SourcePanelID = strings.TrimSpace(job.SourcePanelID)
	if job.Status == "" {
		job.Status = model.Draft
	}
	if err := job.Validate(); err != nil {
		return err
	}
	if _, exists := value.Jobs[job.ID]; exists {
		return fmt.Errorf("job %q already exists", job.ID)
	}
	panel, exists := value.Stock[job.SourcePanelID]
	if !exists {
		return fmt.Errorf("source panel %q does not exist", job.SourcePanelID)
	}
	if panel.Status != model.Available {
		return fmt.Errorf("source panel %q is not available", job.SourcePanelID)
	}
	value.Jobs[job.ID] = job
	return nil
}

func AddPiece(value *model.Ledger, jobID string, piece model.PieceRequirement) error {
	if value == nil {
		return errors.New("ledger is required")
	}
	value.Normalize()
	job, exists := value.Jobs[jobID]
	if !exists {
		return fmt.Errorf("job %q does not exist", jobID)
	}
	if job.Status != model.Draft {
		return fmt.Errorf("job %q is already committed", jobID)
	}
	piece.Label = strings.TrimSpace(piece.Label)
	if err := piece.Validate(); err != nil {
		return err
	}
	job.Pieces = append(job.Pieces, piece)
	value.Jobs[jobID] = job
	return nil
}

func PreviewJob(value model.Ledger, jobID string) (layout.Preview, error) {
	job, exists := value.Jobs[jobID]
	if !exists {
		return layout.Preview{}, fmt.Errorf("job %q does not exist", jobID)
	}
	panel, exists := value.Stock[job.SourcePanelID]
	if !exists {
		return layout.Preview{}, fmt.Errorf("source panel %q does not exist", job.SourcePanelID)
	}
	return layout.PreviewJob(job, panel)
}

func Commit(value *model.Ledger, jobID string) (model.Receipt, error) {
	if value == nil {
		return model.Receipt{}, errors.New("ledger is required")
	}
	value.Normalize()
	job, exists := value.Jobs[jobID]
	if !exists {
		return model.Receipt{}, fmt.Errorf("job %q does not exist", jobID)
	}
	if job.Status != model.Draft {
		return model.Receipt{}, fmt.Errorf("job %q is already committed", jobID)
	}
	if len(job.Pieces) == 0 {
		return model.Receipt{}, fmt.Errorf("job %q has no pieces", jobID)
	}
	panel, exists := value.Stock[job.SourcePanelID]
	if !exists {
		return model.Receipt{}, fmt.Errorf("source panel %q does not exist", job.SourcePanelID)
	}
	if panel.Status != model.Available {
		return model.Receipt{}, fmt.Errorf("source panel %q is not available", job.SourcePanelID)
	}
	preview, err := layout.PreviewJob(job, panel)
	if err != nil {
		return model.Receipt{}, fmt.Errorf("preview job: %w", err)
	}
	if len(preview.Unplaced) != 0 {
		return model.Receipt{}, fmt.Errorf("job %q does not fit: %d piece(s) unplaced", jobID, len(preview.Unplaced))
	}

	receiptID := "receipt-" + job.ID
	if _, exists := value.Receipts[receiptID]; exists {
		return model.Receipt{}, fmt.Errorf("receipt %q already exists", receiptID)
	}
	offcutIDs := make([]string, 0, len(preview.FreeRectangles))
	reservedOffcutIDs := make(map[string]struct{}, len(preview.FreeRectangles))
	for index, rectangle := range preview.FreeRectangles {
		if rectangle.Width <= 0 || rectangle.Height <= 0 {
			continue
		}
		offcutNumber := index + 1
		offcutID := fmt.Sprintf("offcut-%s-%d", job.ID, offcutNumber)
		for {
			_, stockExists := value.Stock[offcutID]
			_, reserved := reservedOffcutIDs[offcutID]
			if !stockExists && !reserved {
				break
			}
			offcutNumber++
			offcutID = fmt.Sprintf("offcut-%s-%d", job.ID, offcutNumber)
		}
		offcutIDs = append(offcutIDs, offcutID)
		reservedOffcutIDs[offcutID] = struct{}{}
	}

	receipt := model.Receipt{
		ID:             receiptID,
		JobID:          job.ID,
		SourcePanelID:  panel.ID,
		SourceWidth:    panel.Width,
		SourceHeight:   panel.Height,
		Kerf:           job.Kerf,
		Placements:     append([]model.Placement(nil), preview.Placements...),
		OffcutPanelIDs: append([]string(nil), offcutIDs...),
	}
	panel.Status = model.Consumed
	value.Stock[panel.ID] = panel
	for index, rectangle := range preview.FreeRectangles {
		if rectangle.Width <= 0 || rectangle.Height <= 0 {
			continue
		}
		offcutID := offcutIDs[indexOfPositiveRectangle(preview.FreeRectangles, index)]
		value.Stock[offcutID] = model.Panel{
			ID:            offcutID,
			Material:      panel.Material,
			Width:         rectangle.Width,
			Height:        rectangle.Height,
			Status:        model.Available,
			Kind:          model.Offcut,
			SourceJobID:   job.ID,
			SourcePanelID: panel.ID,
		}
	}
	job.Status = model.Committed
	job.ReceiptID = receiptID
	value.Jobs[job.ID] = job
	value.Receipts[receipt.ID] = receipt
	return receipt, nil
}

func indexOfPositiveRectangle(rectangles []layout.Rectangle, current int) int {
	positiveIndex := 0
	for index := 0; index < current; index++ {
		if rectangles[index].Width > 0 && rectangles[index].Height > 0 {
			positiveIndex++
		}
	}
	return positiveIndex
}

func SortedPanelIDs(value model.Ledger) []string {
	ids := make([]string, 0, len(value.Stock))
	for id := range value.Stock {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
