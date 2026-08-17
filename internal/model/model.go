package model

import (
	"fmt"
	"strings"
)

const LedgerVersion = 1

type Availability string

const (
	Available Availability = "available"
	Consumed  Availability = "consumed"
)

type PanelKind string

const (
	StockPanel PanelKind = "stock"
	Offcut     PanelKind = "offcut"
)

type GrainConstraint string

const (
	GrainLengthwise GrainConstraint = "lengthwise"
	GrainCrosswise  GrainConstraint = "crosswise"
)

type JobStatus string

const (
	Draft     JobStatus = "draft"
	Committed JobStatus = "committed"
)

type Panel struct {
	ID            string       `json:"id"`
	Material      string       `json:"material"`
	Width         int          `json:"width_mm"`
	Height        int          `json:"height_mm"`
	Status        Availability `json:"status"`
	Kind          PanelKind    `json:"kind"`
	SourceJobID   string       `json:"source_job_id,omitempty"`
	SourcePanelID string       `json:"source_panel_id,omitempty"`
}

type PieceRequirement struct {
	Label    string           `json:"label"`
	Quantity int              `json:"quantity"`
	Width    int              `json:"width_mm"`
	Height   int              `json:"height_mm"`
	Grain    *GrainConstraint `json:"grain,omitempty"`
}

type Job struct {
	ID            string             `json:"id"`
	SourcePanelID string             `json:"source_panel_id"`
	Kerf          int                `json:"kerf_mm"`
	Pieces        []PieceRequirement `json:"pieces"`
	Status        JobStatus          `json:"status"`
	ReceiptID     string             `json:"receipt_id,omitempty"`
}

type Placement struct {
	PieceIndex    int    `json:"piece_index"`
	Label         string `json:"label"`
	QuantityIndex int    `json:"quantity_index"`
	X             int    `json:"x_mm"`
	Y             int    `json:"y_mm"`
	Width         int    `json:"width_mm"`
	Height        int    `json:"height_mm"`
	FootprintW    int    `json:"footprint_width_mm"`
	FootprintH    int    `json:"footprint_height_mm"`
	Rotated       bool   `json:"rotated"`
}

type Receipt struct {
	ID             string      `json:"id"`
	JobID          string      `json:"job_id"`
	SourcePanelID  string      `json:"source_panel_id"`
	SourceWidth    int         `json:"source_width_mm"`
	SourceHeight   int         `json:"source_height_mm"`
	Kerf           int         `json:"kerf_mm"`
	Placements     []Placement `json:"placements"`
	OffcutPanelIDs []string    `json:"offcut_panel_ids"`
}

type Ledger struct {
	Version  int                `json:"version"`
	Stock    map[string]Panel   `json:"stock"`
	Jobs     map[string]Job     `json:"jobs"`
	Receipts map[string]Receipt `json:"receipts"`
}

func NewLedger() Ledger {
	return Ledger{
		Version:  LedgerVersion,
		Stock:    make(map[string]Panel),
		Jobs:     make(map[string]Job),
		Receipts: make(map[string]Receipt),
	}
}

func (l *Ledger) Normalize() {
	if l.Version == 0 {
		l.Version = LedgerVersion
	}
	if l.Stock == nil {
		l.Stock = make(map[string]Panel)
	}
	if l.Jobs == nil {
		l.Jobs = make(map[string]Job)
	}
	if l.Receipts == nil {
		l.Receipts = make(map[string]Receipt)
	}
}

func (p Panel) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("panel id is required")
	}
	if strings.TrimSpace(p.Material) == "" {
		return fmt.Errorf("panel %q material is required", p.ID)
	}
	if p.Width <= 0 || p.Height <= 0 {
		return fmt.Errorf("panel %q dimensions must be positive", p.ID)
	}
	if p.Status != Available && p.Status != Consumed {
		return fmt.Errorf("panel %q has invalid status %q", p.ID, p.Status)
	}
	if p.Kind != StockPanel && p.Kind != Offcut {
		return fmt.Errorf("panel %q has invalid kind %q", p.ID, p.Kind)
	}
	if p.Kind == Offcut && p.SourceJobID == "" {
		return fmt.Errorf("offcut %q is missing its source job", p.ID)
	}
	return nil
}

func (p PieceRequirement) Validate() error {
	if strings.TrimSpace(p.Label) == "" {
		return fmt.Errorf("piece label is required")
	}
	if p.Quantity <= 0 {
		return fmt.Errorf("piece %q quantity must be positive", p.Label)
	}
	if p.Width <= 0 || p.Height <= 0 {
		return fmt.Errorf("piece %q dimensions must be positive", p.Label)
	}
	if p.Grain != nil && *p.Grain != GrainLengthwise && *p.Grain != GrainCrosswise {
		return fmt.Errorf("piece %q has invalid grain %q", p.Label, *p.Grain)
	}
	return nil
}

func (j Job) Validate() error {
	if strings.TrimSpace(j.ID) == "" {
		return fmt.Errorf("job id is required")
	}
	if strings.TrimSpace(j.SourcePanelID) == "" {
		return fmt.Errorf("job %q source panel is required", j.ID)
	}
	if j.Kerf < 0 {
		return fmt.Errorf("job %q kerf must be nonnegative", j.ID)
	}
	if j.Status != Draft && j.Status != Committed {
		return fmt.Errorf("job %q has invalid status %q", j.ID, j.Status)
	}
	for i, piece := range j.Pieces {
		if err := piece.Validate(); err != nil {
			return fmt.Errorf("job %q piece %d: %w", j.ID, i+1, err)
		}
	}
	if j.Status == Committed && j.ReceiptID == "" {
		return fmt.Errorf("committed job %q is missing its receipt", j.ID)
	}
	return nil
}

func (l *Ledger) Validate() error {
	l.Normalize()
	if l.Version != LedgerVersion {
		return fmt.Errorf("unsupported ledger version %d", l.Version)
	}
	for id, panel := range l.Stock {
		if id != panel.ID {
			return fmt.Errorf("stock key %q does not match panel id %q", id, panel.ID)
		}
		if err := panel.Validate(); err != nil {
			return err
		}
	}
	for id, job := range l.Jobs {
		if id != job.ID {
			return fmt.Errorf("job key %q does not match job id %q", id, job.ID)
		}
		if err := job.Validate(); err != nil {
			return err
		}
	}
	for id, receipt := range l.Receipts {
		if id != receipt.ID {
			return fmt.Errorf("receipt key %q does not match receipt id %q", id, receipt.ID)
		}
		if receipt.JobID == "" || receipt.SourcePanelID == "" {
			return fmt.Errorf("receipt %q is missing its job or source panel", id)
		}
		if receipt.SourceWidth <= 0 || receipt.SourceHeight <= 0 || receipt.Kerf < 0 {
			return fmt.Errorf("receipt %q has invalid source dimensions or kerf", id)
		}
	}
	return nil
}
