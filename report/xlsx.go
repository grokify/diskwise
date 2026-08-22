package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/grokify/diskwise/detect"
)

var xlsxHeader = []string{
	"Tier", "Kind", "Allocated Bytes", "Allocated", "Logical Bytes", "Logical",
	"Confidence", "Path", "Path Count", "Reason", "Scenarios",
}

const xlsxSheet = "Opportunities"

// WriteXLSX renders rows as a single-sheet .xlsx workbook: a frozen
// header row and an autofilter over the data range, so Excel/Numbers'
// native column sort and filter UI works immediately on open — no
// macros, no extra sheets.
func WriteXLSX(w io.Writer, rows []Row) error {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	if err := f.SetSheetName("Sheet1", xlsxSheet); err != nil {
		return fmt.Errorf("report: xlsx: rename sheet: %w", err)
	}

	for col, name := range xlsxHeader {
		cell, err := excelize.CoordinatesToCellName(col+1, 1)
		if err != nil {
			return fmt.Errorf("report: xlsx: header cell: %w", err)
		}
		if err := f.SetCellValue(xlsxSheet, cell, name); err != nil {
			return fmt.Errorf("report: xlsx: set header: %w", err)
		}
	}

	for i, r := range rows {
		row := i + 2 // header occupies row 1
		values := []any{
			string(r.Tier), string(r.Kind),
			r.AllocatedSize, humanBytes(r.AllocatedSize),
			r.LogicalSize, humanBytes(r.LogicalSize),
			r.Confidence, r.Path, r.PathCount, r.Reason,
			scenariosText(r.Scenarios),
		}
		for col, v := range values {
			cell, err := excelize.CoordinatesToCellName(col+1, row)
			if err != nil {
				return fmt.Errorf("report: xlsx: row %d cell: %w", row, err)
			}
			if err := f.SetCellValue(xlsxSheet, cell, v); err != nil {
				return fmt.Errorf("report: xlsx: set row %d: %w", row, err)
			}
		}
	}

	lastCol, err := excelize.CoordinatesToCellName(len(xlsxHeader), len(rows)+1)
	if err != nil {
		return fmt.Errorf("report: xlsx: filter range: %w", err)
	}
	if err := f.AutoFilter(xlsxSheet, "A1:"+lastCol, nil); err != nil {
		return fmt.Errorf("report: xlsx: autofilter: %w", err)
	}
	if err := f.SetPanes(xlsxSheet, &excelize.Panes{Freeze: true, Split: false, XSplit: 0, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return fmt.Errorf("report: xlsx: freeze header: %w", err)
	}
	for col := 1; col <= len(xlsxHeader); col++ {
		colName, err := excelize.ColumnNumberToName(col)
		if err != nil {
			return fmt.Errorf("report: xlsx: column width: %w", err)
		}
		if err := f.SetColWidth(xlsxSheet, colName, colName, 18); err != nil {
			return fmt.Errorf("report: xlsx: column width: %w", err)
		}
	}

	if _, err := f.WriteTo(w); err != nil {
		return fmt.Errorf("report: xlsx: write: %w", err)
	}
	return nil
}

// scenariosText formats a finding's scenarios as one semicolon-
// separated line, since a spreadsheet cell has no room for the HTML
// report's nested list.
func scenariosText(scenarios []detect.Scenario) string {
	if len(scenarios) == 0 {
		return ""
	}
	parts := make([]string, len(scenarios))
	for i, sc := range scenarios {
		parts[i] = fmt.Sprintf("[%s] %s — %s", sc.Name, humanBytes(sc.ReclaimableBytes), sc.Description)
	}
	return strings.Join(parts, "; ")
}
