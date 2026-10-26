package profilefixtures

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Persona struct {
	ScenarioID    string `json:"scenario_id"`
	BureauINN     string `json:"bureau_inn"`
	LegalForm     string `json:"legal_form"`
	ProfileSource string `json:"profile_source"`
	MockMode      string `json:"mock_mode"`
}
type PersonaCatalog struct {
	SchemaVersion  int       `json:"schema_version"`
	Synthetic      bool      `json:"synthetic"`
	ProfilesSource string    `json:"profiles_source"`
	BureauSource   string    `json:"bureau_source"`
	Personas       []Persona `json:"personas"`
}

// LoadPersonas validates aliases used by client, bureau and future web fixtures.
// Tax periods and finances stay in the client profile, never in registry responses.
func LoadPersonas() (PersonaCatalog, error) {
	raw, err := Files.ReadFile("personas.json")
	if err != nil {
		return PersonaCatalog{}, err
	}
	var catalog PersonaCatalog
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&catalog); err != nil {
		return catalog, err
	}
	if !catalog.Synthetic || catalog.SchemaVersion != 1 || catalog.ProfilesSource != "requirements-cases.json" || catalog.BureauSource != "../personas.yaml" {
		return catalog, fmt.Errorf("invalid persona catalog metadata")
	}
	data, err := Load()
	if err != nil {
		return catalog, err
	}
	byID := map[string]Scenario{}
	for _, s := range data.Cases {
		byID[s.ID] = s
	}
	seen := map[string]bool{}
	for _, p := range catalog.Personas {
		s, ok := byID[p.ScenarioID]
		if !ok || seen[p.ScenarioID] || s.Profile.LegalForm == nil || p.BureauINN != s.Profile.Inn || p.LegalForm != string(*s.Profile.LegalForm) || p.ProfileSource != "CLIENT_DECLARED" || p.MockMode != "LOCAL_ONLY" {
			return catalog, fmt.Errorf("invalid persona link: %s", p.ScenarioID)
		}
		seen[p.ScenarioID] = true
	}
	if len(seen) != len(byID) {
		return catalog, fmt.Errorf("persona catalog does not cover profile scenarios")
	}
	return catalog, nil
}
