package challenge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/amboss-mededu/go-coding-challenge/model"
)

func TestPayloads(t *testing.T) {
	tests := []struct {
		file string
		into any
	}{
		{"1.json", &model.Product{}},
		{"2.json", &model.Product{}},
		{"3.json", &model.Product{}},
		{"4.json", &model.Category{}},
		{"5.json", &model.Category{}},
		{"6.json", &model.Category{}},
		{"7.json", &model.Article{}},
		{"8.json", &model.Article{}},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tt.file))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, tt.into); err != nil {
				t.Fatal(err)
			}
		})
	}
}
