package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"panelnest/internal/layout"
	"panelnest/internal/ledger"
	"panelnest/internal/model"
)

const defaultLedgerPath = ".panelnest.json"

func Run(args []string, out, errOut io.Writer) error {
	args, ledgerPath, err := extractLedgerPath(args)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New("a command is required; use stock-add, job-open, piece-add, preview, commit, show, stock-list, or smoke")
	}
	switch args[0] {
	case "stock-add":
		return runStockAdd(args[1:], ledgerPath, out)
	case "job-open":
		return runJobOpen(args[1:], ledgerPath, out)
	case "piece-add":
		return runPieceAdd(args[1:], ledgerPath, out)
	case "preview":
		return runPreview(args[1:], ledgerPath, out)
	case "commit":
		return runCommit(args[1:], ledgerPath, out)
	case "show":
		return runShow(args[1:], ledgerPath, out)
	case "stock-list":
		return runStockList(args[1:], ledgerPath, out)
	case "smoke":
		return runSmoke(out)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func extractLedgerPath(args []string) ([]string, string, error) {
	path := defaultLedgerPath
	cleaned := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "--ledger" {
			if index+1 >= len(args) {
				return nil, "", errors.New("--ledger requires a path")
			}
			path = args[index+1]
			index++
			continue
		}
		if strings.HasPrefix(argument, "--ledger=") {
			path = strings.TrimPrefix(argument, "--ledger=")
			if path == "" {
				return nil, "", errors.New("--ledger requires a path")
			}
			continue
		}
		cleaned = append(cleaned, argument)
	}
	if strings.TrimSpace(path) == "" {
		return nil, "", errors.New("--ledger requires a path")
	}
	return cleaned, path, nil
}

func newFlagSet(name string, out io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return flags
}

func runStockAdd(args []string, path string, out io.Writer) error {
	flags := newFlagSet("stock-add", out)
	id := flags.String("id", "", "unique panel id")
	material := flags.String("material", "", "material label")
	width := flags.Int("width", 0, "panel width in millimetres")
	height := flags.Int("height", 0, "panel height in millimetres")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireFlags(map[string]string{"id": *id, "material": *material}); err != nil {
		return err
	}
	value, err := ledger.Load(path)
	if err != nil {
		return err
	}
	if err := ledger.AddPanel(&value, model.Panel{ID: *id, Material: *material, Width: *width, Height: *height}); err != nil {
		return err
	}
	if err := ledger.Save(path, &value); err != nil {
		return err
	}
	fmt.Fprintf(out, "stock added: id=%s material=%s size=%dx%dmm status=%s\n", *id, *material, *width, *height, model.Available)
	return nil
}

func runJobOpen(args []string, path string, out io.Writer) error {
	flags := newFlagSet("job-open", out)
	id := flags.String("id", "", "unique job id")
	panelID := flags.String("panel", "", "source panel id")
	kerf := flags.Int("kerf", 0, "kerf in millimetres")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireFlags(map[string]string{"id": *id, "panel": *panelID}); err != nil {
		return err
	}
	value, err := ledger.Load(path)
	if err != nil {
		return err
	}
	if err := ledger.OpenJob(&value, model.Job{ID: *id, SourcePanelID: *panelID, Kerf: *kerf}); err != nil {
		return err
	}
	if err := ledger.Save(path, &value); err != nil {
		return err
	}
	fmt.Fprintf(out, "job opened: id=%s panel=%s kerf=%dmm status=%s\n", *id, *panelID, *kerf, model.Draft)
	return nil
}

func runPieceAdd(args []string, path string, out io.Writer) error {
	flags := newFlagSet("piece-add", out)
	jobID := flags.String("job", "", "job id")
	label := flags.String("label", "", "piece label")
	quantity := flags.Int("quantity", 0, "piece quantity")
	width := flags.Int("width", 0, "piece width in millimetres")
	height := flags.Int("height", 0, "piece height in millimetres")
	grain := flags.String("grain", "", "optional grain constraint: lengthwise or crosswise")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireFlags(map[string]string{"job": *jobID, "label": *label}); err != nil {
		return err
	}
	var constraint *model.GrainConstraint
	if supplied := flagWasSet(flags, "grain"); supplied {
		parsed := model.GrainConstraint(*grain)
		constraint = &parsed
	}
	value, err := ledger.Load(path)
	if err != nil {
		return err
	}
	piece := model.PieceRequirement{Label: *label, Quantity: *quantity, Width: *width, Height: *height, Grain: constraint}
	if err := ledger.AddPiece(&value, *jobID, piece); err != nil {
		return err
	}
	if err := ledger.Save(path, &value); err != nil {
		return err
	}
	grainText := "omitted"
	if constraint != nil {
		grainText = string(*constraint)
	}
	fmt.Fprintf(out, "piece added: job=%s label=%s quantity=%d size=%dx%dmm grain=%s\n", *jobID, *label, *quantity, *width, *height, grainText)
	return nil
}

func runPreview(args []string, path string, out io.Writer) error {
	flags := newFlagSet("preview", out)
	jobID := flags.String("job", "", "job id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireFlags(map[string]string{"job": *jobID}); err != nil {
		return err
	}
	value, err := ledger.Load(path)
	if err != nil {
		return err
	}
	preview, err := ledger.PreviewJob(value, *jobID)
	if err != nil {
		return err
	}
	printPreview(out, preview)
	return nil
}

func runCommit(args []string, path string, out io.Writer) error {
	flags := newFlagSet("commit", out)
	jobID := flags.String("job", "", "job id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireFlags(map[string]string{"job": *jobID}); err != nil {
		return err
	}
	value, err := ledger.Load(path)
	if err != nil {
		return err
	}
	receipt, err := ledger.Commit(&value, *jobID)
	if err != nil {
		return err
	}
	if err := ledger.Save(path, &value); err != nil {
		return err
	}
	fmt.Fprintf(out, "job committed: id=%s receipt=%s placements=%d offcuts=%d\n", receipt.JobID, receipt.ID, len(receipt.Placements), len(receipt.OffcutPanelIDs))
	return nil
}

func runShow(args []string, path string, out io.Writer) error {
	flags := newFlagSet("show", out)
	jobID := flags.String("job", "", "job id")
	receiptID := flags.String("receipt", "", "receipt id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *jobID == "" && *receiptID == "" {
		return errors.New("show requires --job or --receipt")
	}
	if *jobID != "" && *receiptID != "" {
		return errors.New("show accepts only one of --job or --receipt")
	}
	value, err := ledger.Load(path)
	if err != nil {
		return err
	}
	if *receiptID != "" {
		receipt, exists := value.Receipts[*receiptID]
		if !exists {
			return fmt.Errorf("receipt %q does not exist", *receiptID)
		}
		printReceipt(out, receipt)
		return nil
	}
	job, exists := value.Jobs[*jobID]
	if !exists {
		return fmt.Errorf("job %q does not exist", *jobID)
	}
	fmt.Fprintf(out, "job: id=%s status=%s panel=%s kerf=%dmm pieces=%d\n", job.ID, job.Status, job.SourcePanelID, job.Kerf, len(job.Pieces))
	for index, piece := range job.Pieces {
		grainText := "omitted"
		if piece.Grain != nil {
			grainText = string(*piece.Grain)
		}
		fmt.Fprintf(out, "piece: index=%d label=%s quantity=%d size=%dx%dmm grain=%s\n", index+1, piece.Label, piece.Quantity, piece.Width, piece.Height, grainText)
	}
	if job.ReceiptID != "" {
		receipt, exists := value.Receipts[job.ReceiptID]
		if !exists {
			return fmt.Errorf("job %q references missing receipt %q", job.ID, job.ReceiptID)
		}
		printReceipt(out, receipt)
	}
	return nil
}

func runStockList(args []string, path string, out io.Writer) error {
	flags := newFlagSet("stock-list", out)
	if err := flags.Parse(args); err != nil {
		return err
	}
	value, err := ledger.Load(path)
	if err != nil {
		return err
	}
	ids := ledger.SortedPanelIDs(value)
	if len(ids) == 0 {
		fmt.Fprintln(out, "stock: empty")
		return nil
	}
	for _, id := range ids {
		panel := value.Stock[id]
		fmt.Fprintf(out, "stock: id=%s material=%s size=%dx%dmm status=%s kind=%s", panel.ID, panel.Material, panel.Width, panel.Height, panel.Status, panel.Kind)
		if panel.SourceJobID != "" {
			fmt.Fprintf(out, " source_job=%s", panel.SourceJobID)
		}
		fmt.Fprintln(out)
	}
	return nil
}

func runSmoke(out io.Writer) error {
	directory, err := os.MkdirTemp("", "panelnest-smoke-")
	if err != nil {
		return fmt.Errorf("create smoke directory: %w", err)
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "ledger.json")
	value, err := ledger.Load(path)
	if err != nil {
		return err
	}
	if err := ledger.AddPanel(&value, model.Panel{ID: "panel-smoke", Material: "birch-ply", Width: 1200, Height: 600}); err != nil {
		return err
	}
	if err := ledger.OpenJob(&value, model.Job{ID: "job-smoke", SourcePanelID: "panel-smoke", Kerf: 3}); err != nil {
		return err
	}
	if err := ledger.AddPiece(&value, "job-smoke", model.PieceRequirement{Label: "sign-face", Quantity: 1, Width: 500, Height: 300}); err != nil {
		return err
	}
	if err := ledger.Save(path, &value); err != nil {
		return err
	}
	value, err = ledger.Load(path)
	if err != nil {
		return err
	}
	preview, err := ledger.PreviewJob(value, "job-smoke")
	if err != nil {
		return err
	}
	if len(preview.Unplaced) != 0 {
		return errors.New("smoke preview unexpectedly has unplaced pieces")
	}
	receipt, err := ledger.Commit(&value, "job-smoke")
	if err != nil {
		return err
	}
	if err := ledger.Save(path, &value); err != nil {
		return err
	}
	value, err = ledger.Load(path)
	if err != nil {
		return err
	}
	if value.Stock["panel-smoke"].Status != model.Consumed {
		return errors.New("smoke source panel was not consumed")
	}
	if _, exists := value.Receipts[receipt.ID]; !exists {
		return errors.New("smoke receipt was not persisted")
	}
	fmt.Fprintf(out, "smoke: workflow complete receipt=%s placements=%d offcuts=%d\n", receipt.ID, len(receipt.Placements), len(receipt.OffcutPanelIDs))
	return nil
}

func printPreview(out io.Writer, preview layout.Preview) {
	fmt.Fprintf(out, "preview: job=%s panel=%s placed=%d unplaced=%d\n", preview.JobID, preview.PanelID, len(preview.Placements), len(preview.Unplaced))
	for _, placement := range preview.Placements {
		fmt.Fprintf(out, "placed: piece=%d label=%s quantity_index=%d at=%d,%dmm size=%dx%dmm footprint=%dx%dmm rotated=%t\n", placement.PieceIndex, placement.Label, placement.QuantityIndex, placement.X, placement.Y, placement.Width, placement.Height, placement.FootprintW, placement.FootprintH, placement.Rotated)
	}
	for _, item := range preview.Unplaced {
		fmt.Fprintf(out, "unplaced: piece=%d label=%s quantity_index=%d reason=%s\n", item.PieceIndex, item.Label, item.QuantityIndex, item.Reason)
	}
	fmt.Fprintf(out, "free: rectangles=%d\n", len(preview.FreeRectangles))
	for index, rectangle := range preview.FreeRectangles {
		fmt.Fprintf(out, "free-rectangle: index=%d at=%d,%dmm size=%dx%dmm\n", index+1, rectangle.X, rectangle.Y, rectangle.Width, rectangle.Height)
	}
}

func printReceipt(out io.Writer, receipt model.Receipt) {
	fmt.Fprintf(out, "receipt: id=%s job=%s panel=%s source_size=%dx%dmm kerf=%dmm placements=%d offcuts=%d\n", receipt.ID, receipt.JobID, receipt.SourcePanelID, receipt.SourceWidth, receipt.SourceHeight, receipt.Kerf, len(receipt.Placements), len(receipt.OffcutPanelIDs))
	for _, placement := range receipt.Placements {
		fmt.Fprintf(out, "receipt-placement: piece=%d label=%s quantity_index=%d at=%d,%dmm size=%dx%dmm rotated=%t\n", placement.PieceIndex, placement.Label, placement.QuantityIndex, placement.X, placement.Y, placement.Width, placement.Height, placement.Rotated)
	}
	for index, offcutID := range receipt.OffcutPanelIDs {
		fmt.Fprintf(out, "receipt-offcut: index=%d id=%s\n", index+1, offcutID)
	}
}

func requireFlags(values map[string]string) error {
	for name, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("--%s is required", name)
		}
	}
	return nil
}

func flagWasSet(flags *flag.FlagSet, name string) bool {
	set := false
	flags.Visit(func(flag *flag.Flag) {
		if flag.Name == name {
			set = true
		}
	})
	return set
}
