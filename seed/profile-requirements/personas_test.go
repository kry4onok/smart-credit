package profilefixtures

import "testing"

func TestPersonaLinks(t *testing.T) {
	catalog, err := LoadPersonas()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Personas) != 16 {
		t.Fatal("missing scenarios")
	}
	catalog.Personas[0].BureauINN = "mutated"
	fresh, err := LoadPersonas()
	if err != nil || fresh.Personas[0].BureauINN == "mutated" {
		t.Fatal("shared mutable persona catalog")
	}
}
