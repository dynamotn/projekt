package cli

import (
	"bytes"
	"testing"

	"github.com/jedib0t/go-pretty/v6/table"
)

func TestEncodeTable(t *testing.T) {
	tests := []struct {
		name    string
		noColor bool
		rows    []table.Row
		wantErr bool
	}{
		{
			name:    "table with color",
			noColor: false,
			rows: []table.Row{
				{"Item 1", "Value 1"},
				{"Item 2", "Value 2"},
			},
			wantErr: false,
		},
		{
			name:    "table without color",
			noColor: true,
			rows: []table.Row{
				{"Item 1", "Value 1"},
				{"Item 2", "Value 2"},
			},
			wantErr: false,
		},
		{
			name:    "empty table",
			noColor: false,
			rows:    []table.Row{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tw := table.NewWriter()
			tw.AppendHeader(table.Row{"Key", "Value"})
			for _, row := range tt.rows {
				tw.AppendRow(row)
			}

			err := EncodeTable(&buf, tw, tt.noColor)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeTable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && buf.Len() == 0 {
				t.Error("EncodeTable() produced no output")
			}
		})
	}
}

func TestEncodeTable_Sorting(t *testing.T) {
	var buf bytes.Buffer
	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Name", "Age"})
	tw.AppendRow(table.Row{"Bob", 30})
	tw.AppendRow(table.Row{"Alice", 25})
	tw.AppendRow(table.Row{"Charlie", 35})

	err := EncodeTable(&buf, tw, true)
	if err != nil {
		t.Errorf("EncodeTable() error = %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("EncodeTable() produced no output")
	}

	// Table should be sorted by first column
	alicePos := bytes.Index([]byte(output), []byte("Alice"))
	bobPos := bytes.Index([]byte(output), []byte("Bob"))
	charliePos := bytes.Index([]byte(output), []byte("Charlie"))

	if alicePos == -1 || bobPos == -1 || charliePos == -1 {
		t.Error("EncodeTable() missing expected row data")
	}
}
