package problemgenerator

import (
	"context"
	"encoding/csv"
	"os"
	"strconv"
)

type CSVRepo struct {
	templates []Template
}

// NewCSVRepo loads ./data/problem_sets.csv at program start.
func NewCSVRepo(path string) (*CSVRepo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = 3 // id,operation,template
	r.TrimLeadingSpace = true

	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	// skip header
	var tpl []Template
	for _, rec := range records[1:] {
		id, _ := strconv.Atoi(rec[0])
		tpl = append(tpl, Template{
			ID: id, Operation: rec[1], Template: rec[2],
		})
	}
	return &CSVRepo{templates: tpl}, nil
}

// ListByOperation satisfies TemplateRepo.
func (r *CSVRepo) ListByOperation(_ context.Context, op string) ([]Template, error) {
	var out []Template
	for _, t := range r.templates {
		if t.Operation == op {
			out = append(out, t)
		}
	}
	return out, nil
}
