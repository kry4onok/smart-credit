package profilefixtures

import "testing"

func TestFixturePresenceAndIsolation(t *testing.T) {
	data, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 16 {
		t.Fatalf("want 16 scenarios, got %d", len(data.Cases))
	}
	found := map[string]bool{}
	for _, scenario := range data.Cases {
		switch scenario.ID {
		case "missing_profit":
			if scenario.Profile.Profit12m != nil {
				t.Fatal("missing profit became a value")
			}
			found[scenario.ID] = true
		case "zero_profit":
			if scenario.Profile.Profit12m == nil || scenario.Profile.Profit12m.AmountKopecks != 0 {
				t.Fatal("known zero lost")
			}
			found[scenario.ID] = true
		case "loss":
			if scenario.Profile.Profit12m == nil || scenario.Profile.Profit12m.AmountKopecks >= 0 {
				t.Fatal("loss lost")
			}
			found[scenario.ID] = true
		}
	}
	if len(found) != 3 {
		t.Fatal("presence scenarios missing")
	}
	data.Cases[0].Profile.Name = "mutated"
	again, err := Load()
	if err != nil || again.Cases[0].Profile.Name == "mutated" {
		t.Fatal("fixture state shared between calls")
	}
}
