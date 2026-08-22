package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/grokify/diskwise/detect"
	"github.com/grokify/diskwise/entity"
	"github.com/grokify/diskwise/policy"
)

func TestWriteXLSX_HeaderAndRows(t *testing.T) {
	rows := []Row{
		{Tier: policy.SafeDelete, Kind: entity.KindCache, Path: "/a", AllocatedSize: 100, LogicalSize: 100, Confidence: 0.9, Reason: "cache"},
		{
			Tier: policy.LikelySafe, Kind: entity.KindArtifactFamily, Path: "/b", AllocatedSize: 200, PathCount: 2,
			Scenarios: []detect.Scenario{{Name: "keep-newest", Description: "keep latest", ReclaimableBytes: 150}},
		},
	}

	var buf bytes.Buffer
	if err := WriteXLSX(&buf, rows); err != nil {
		t.Fatalf("WriteXLSX: %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("re-opening generated xlsx: %v", err)
	}
	defer func() { _ = f.Close() }()

	sheetRows, err := f.GetRows(xlsxSheet)
	if err != nil {
		t.Fatalf("GetRows: %v", err)
	}
	if len(sheetRows) != 3 { // header + 2 data rows
		t.Fatalf("len(sheetRows) = %d, want 3", len(sheetRows))
	}
	if sheetRows[0][0] != "Tier" {
		t.Errorf("header[0] = %q, want %q", sheetRows[0][0], "Tier")
	}
	if got, want := sheetRows[1][0], string(policy.SafeDelete); got != want {
		t.Errorf("row 1 Tier = %q, want %q", got, want)
	}
	if got, want := sheetRows[1][7], "/a"; got != want {
		t.Errorf("row 1 Path = %q, want %q", got, want)
	}

	found := false
	for _, cell := range sheetRows[2] {
		if strings.Contains(cell, "keep-newest") {
			found = true
		}
	}
	if !found {
		t.Error("row 2 is missing scenario summary text")
	}

	panes, err := f.GetPanes(xlsxSheet)
	if err != nil {
		t.Fatalf("GetPanes: %v", err)
	}
	if !panes.Freeze || panes.TopLeftCell != "A2" || panes.YSplit != 1 {
		t.Errorf("panes = %+v, want a header row frozen at A2 (YSplit=1)", panes)
	}
}
