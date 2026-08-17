package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"

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

	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open ledger lock: %w", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock ledger: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	stored, err := Load(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		stored, err = mergeLedgers(stored, *value)
		if err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat ledger: %w", err)
	} else {
		stored = *value
	}
	if err := stored.Validate(); err != nil {
		return fmt.Errorf("validate merged ledger: %w", err)
	}

	data, err := json.MarshalIndent(&stored, "", "  ")
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
	*value = stored
	return nil
}

func mergeLedgers(stored, proposed model.Ledger) (model.Ledger, error) {
	for id, panel := range proposed.Stock {
		current, exists := stored.Stock[id]
		if !exists {
			stored.Stock[id] = panel
			continue
		}
		merged, err := mergePanel(current, panel)
		if err != nil {
			return model.Ledger{}, fmt.Errorf("merge panel %q: %w", id, err)
		}
		stored.Stock[id] = merged
	}
	for id, job := range proposed.Jobs {
		current, exists := stored.Jobs[id]
		if !exists {
			stored.Jobs[id] = job
			continue
		}
		merged, err := mergeJob(current, job)
		if err != nil {
			return model.Ledger{}, fmt.Errorf("merge job %q: %w", id, err)
		}
		stored.Jobs[id] = merged
	}
	for id, receipt := range proposed.Receipts {
		current, exists := stored.Receipts[id]
		if exists && !reflect.DeepEqual(current, receipt) {
			return model.Ledger{}, fmt.Errorf("merge receipt %q: conflicting updates", id)
		}
		stored.Receipts[id] = receipt
	}

	committedPanels := make(map[string]string)
	for id, job := range stored.Jobs {
		if job.Status != model.Committed {
			continue
		}
		if other, exists := committedPanels[job.SourcePanelID]; exists && other != id {
			return model.Ledger{}, fmt.Errorf("source panel %q was committed by both %q and %q", job.SourcePanelID, other, id)
		}
		committedPanels[job.SourcePanelID] = id
	}
	return stored, nil
}

func mergePanel(stored, proposed model.Panel) (model.Panel, error) {
	if stored == proposed {
		return stored, nil
	}
	storedStatus := stored.Status
	proposedStatus := proposed.Status
	stored.Status = ""
	proposed.Status = ""
	if stored != proposed {
		return model.Panel{}, errors.New("conflicting updates")
	}
	stored.Status = storedStatus
	if proposedStatus == model.Consumed {
		stored.Status = model.Consumed
	}
	return stored, nil
}

func mergeJob(stored, proposed model.Job) (model.Job, error) {
	if stored.ID != proposed.ID || stored.SourcePanelID != proposed.SourcePanelID || stored.Kerf != proposed.Kerf {
		return model.Job{}, errors.New("conflicting updates")
	}
	if stored.Status == model.Committed || proposed.Status == model.Committed {
		if !reflect.DeepEqual(stored.Pieces, proposed.Pieces) {
			return model.Job{}, errors.New("pieces changed while the job was committed")
		}
		if stored.Status == model.Committed && proposed.Status == model.Committed && stored.ReceiptID != proposed.ReceiptID {
			return model.Job{}, errors.New("conflicting receipts")
		}
		if proposed.Status == model.Committed {
			return proposed, nil
		}
		return stored, nil
	}
	if stored.ReceiptID != proposed.ReceiptID {
		return model.Job{}, errors.New("conflicting receipts")
	}
	if piecePrefix(stored.Pieces, proposed.Pieces) {
		return proposed, nil
	}
	if piecePrefix(proposed.Pieces, stored.Pieces) {
		return stored, nil
	}
	return model.Job{}, errors.New("conflicting piece updates")
}

func piecePrefix(prefix, pieces []model.PieceRequirement) bool {
	if len(prefix) > len(pieces) {
		return false
	}
	for index := range prefix {
		if !reflect.DeepEqual(prefix[index], pieces[index]) {
			return false
		}
	}
	return true
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
	for index, rectangle := range preview.FreeRectangles {
		if rectangle.Width <= 0 || rectangle.Height <= 0 {
			continue
		}
		offcutID := fmt.Sprintf("offcut-%s-%d", job.ID, index+1)
		if _, exists := value.Stock[offcutID]; exists {
			return model.Receipt{}, fmt.Errorf("offcut panel %q already exists", offcutID)
		}
		offcutIDs = append(offcutIDs, offcutID)
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
