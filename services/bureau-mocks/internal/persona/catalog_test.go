package persona

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeedScenarios(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "seed", "personas.yaml")
	catalog, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, inn := range []string{"7707083893", "7810755500", "5401010012", "6671160003", "1234567894", "1111111117", "9876543210", "500100732259"} {
		if !ValidINN(inn) {
			t.Fatalf("seed INN %s is invalid", inn)
		}
		if _, ok := catalog.Lookup(inn); !ok {
			t.Fatalf("seed INN %s is missing", inn)
		}
	}
	if _, ok := catalog.Lookup("1234567809"); ok {
		t.Fatal("unknown valid INN should not be synthesized")
	}
	zero, _ := catalog.Lookup("9876543210")
	if zero.Credit.TotalDebt == nil || zero.Credit.OverdueCount_12M == nil || zero.Credit.MaxOverdueDays == nil ||
		zero.Credit.TotalDebt.AmountKopecks != 0 || *zero.Credit.OverdueCount_12M != 0 || *zero.Credit.MaxOverdueDays != 0 {
		t.Fatal("explicit zero values lost")
	}
	missing, _ := catalog.Lookup("500100732259")
	if missing.Credit.TotalDebt != nil || missing.Credit.OverdueCount_12M != nil || missing.Credit.MaxOverdueDays != nil {
		t.Fatal("missing bureau values became known zero")
	}
}

func TestRejectInvalidSeed(t *testing.T) {
	for _, input := range []string{
		"version: 2\npersonas: []\n",
		"version: 1\npersonas:\n  - inn: '7707083894'\n",
		"version: 1\npersonas:\n  - inn: '7707083893'\n    unexpected: true\n",
	} {
		path := filepath.Join(t.TempDir(), "personas.yaml")
		if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("accepted invalid seed: %s", strings.ReplaceAll(input, "\n", " "))
		}
	}
}
