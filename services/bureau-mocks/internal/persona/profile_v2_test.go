package persona

import (
	"path/filepath"
	"testing"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	profilefixtures "github.com/alfabank-hackathon-2026/smart-credit/seed/profile-requirements"
)

func TestProfileFixturesResolveInBureau(t *testing.T) {
	data, err := profilefixtures.Load()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := Load(filepath.Join("..", "..", "..", "..", "seed", "personas.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range data.Cases {
		scenario, ok := catalog.Lookup(item.Profile.Inn)
		if !ok {
			t.Fatalf("missing bureau entry for %s", item.ID)
		}
		want := commonpb.LegalForm_LEGAL_FORM_LLC
		if string(*item.Profile.LegalForm) == "IP" {
			want = commonpb.LegalForm_LEGAL_FORM_IP
		}
		if scenario.Profile.LegalForm != want {
			t.Fatalf("legal form mismatch: %s", item.ID)
		}
	}
}

func TestExplicitLegalFormValidation(t *testing.T) {
	for _, test := range []struct {
		inn, form string
		fail      bool
	}{
		{"7707083893", "LLC", false}, {"500100732259", "IP", false},
		{"7707083893", "", false}, {"7707083893", "IP", true},
		{"500100732259", "LLC", true}, {"7707083893", "UNKNOWN", true},
	} {
		_, err := convert(personaSeed{INN: test.inn, Profile: profileSeed{LegalForm: test.form, Status: "ACTIVE", RegisteredAt: "2020-01-01", OKVED: "47.11"}})
		if (err != nil) != test.fail {
			t.Fatalf("form=%s inn=%s error=%v", test.form, test.inn, err)
		}
	}
}
