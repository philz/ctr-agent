package main

import (
	"reflect"
	"testing"
)

func TestParsePorts(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []int
		wantErr bool
	}{
		{
			name:  "single port",
			input: "8000",
			want:  []int{8000},
		},
		{
			name:  "multiple ports comma-separated",
			input: "8000,8001,8002",
			want:  []int{8000, 8001, 8002},
		},
		{
			name:  "port range",
			input: "8000-8003",
			want:  []int{8000, 8001, 8002, 8003},
		},
		{
			name:  "mixed: ports and ranges",
			input: "8000,8010-8012,8020",
			want:  []int{8000, 8010, 8011, 8012, 8020},
		},
		{
			name:  "mixed: multiple ranges and ports",
			input: "7000,8000-8002,9000-9001,9999",
			want:  []int{7000, 8000, 8001, 8002, 9000, 9001, 9999},
		},
		{
			name:  "with spaces",
			input: "8000, 8001 , 8002",
			want:  []int{8000, 8001, 8002},
		},
		{
			name:  "range with spaces",
			input: "8000 - 8002",
			want:  []int{8000, 8001, 8002},
		},
		{
			name:  "mixed with spaces",
			input: "8000, 8010 - 8012, 8020",
			want:  []int{8000, 8010, 8011, 8012, 8020},
		},
		{
			name:  "duplicate ports removed",
			input: "8000,8000,8001",
			want:  []int{8000, 8001},
		},
		{
			name:  "overlapping ranges",
			input: "8000-8002,8001-8003",
			want:  []int{8000, 8001, 8002, 8003},
		},
		{
			name:    "invalid port",
			input:   "not-a-port",
			wantErr: true,
		},
		{
			name:    "invalid range format",
			input:   "8000-8001-8002",
			wantErr: true,
		},
		{
			name:    "invalid range start",
			input:   "abc-8002",
			wantErr: true,
		},
		{
			name:    "invalid range end",
			input:   "8000-xyz",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePorts(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePorts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePorts() = %v, want %v", got, tt.want)
			}
		})
	}
}
