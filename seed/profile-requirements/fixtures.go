// Package profilefixtures shares deterministic profile examples between services and tools.
// Expected outcomes describe the next implementation stage; Load does not run scoring or policy.
package profilefixtures

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
)

// Files contains contract examples, including deliberately invalid HTTP requests.
//
//go:embed *.json *.textproto
var Files embed.FS

type Dataset struct {
	SchemaVersion int        `json:"schema_version"`
	Synthetic     bool       `json:"synthetic"`
	Status        string     `json:"status"`
	Cases         []Scenario `json:"cases"`
}

type Scenario struct {
	ID            string                    `json:"scenario_id"`
	ReferenceDate string                    `json:"reference_date"`
	Profile       common.CompanyProfile     `json:"profile"`
	Product       common.Product            `json:"product"`
	Form          json.RawMessage           `json:"form"`
	Documents     []common.DocumentMetadata `json:"documents"`
	Expected      Expectation               `json:"expected"`
	Note          string                    `json:"note"`
}

type Expectation struct {
	Route           *string  `json:"route"`
	CanSubmit       bool     `json:"can_submit"`
	ValidationError *string  `json:"validation_error"`
	AssessmentCodes []string `json:"assessment_codes"`
}

// Load returns independent typed values on each call; no database or external sources are used.
func Load() (Dataset, error) {
	data, err := Files.ReadFile("requirements-cases.json")
	if err != nil {
		return Dataset{}, err
	}
	var dataset Dataset
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dataset); err != nil {
		return Dataset{}, fmt.Errorf("decode profile fixtures: %w", err)
	}
	if dataset.SchemaVersion != 1 || !dataset.Synthetic || len(dataset.Cases) == 0 {
		return Dataset{}, fmt.Errorf("invalid profile fixture header")
	}
	seen := make(map[string]bool)
	for _, scenario := range dataset.Cases {
		if scenario.ID == "" || seen[scenario.ID] {
			return Dataset{}, fmt.Errorf("empty or duplicate scenario ID")
		}
		seen[scenario.ID] = true
	}
	return dataset, nil
}
