# PanelNest

PanelNest is a standard-library Go CLI for deterministic rectangular cut planning from sheet goods. A local JSON ledger holds source panels, draft jobs, committed receipts, and usable offcuts.

## Usage

The ledger defaults to `.panelnest.json`. Use `--ledger PATH` before or after the command to select another file.

```text
panelnest stock-add --id panel-1 --material birch-ply --width 2440 --height 1220
panelnest job-open --id job-1 --panel panel-1 --kerf 3
panelnest piece-add --job job-1 --label side --quantity 2 --width 700 --height 400
panelnest piece-add --job job-1 --label rail --quantity 1 --width 500 --height 100 --grain lengthwise
panelnest preview --job job-1
panelnest commit --job job-1
panelnest show --job job-1
panelnest stock-list
```

An omitted grain constraint allows either orientation. `lengthwise` keeps the entered orientation and `crosswise` requires the rotated orientation. Kerf is included in each placement footprint. Preview never changes the ledger; commit consumes the source panel and records the immutable placement receipt.

The bounded complete-workflow check is available with:

```text
go run ./cmd/panelnest smoke
```

The project uses Go 1.22.0 and only the Go standard library.
