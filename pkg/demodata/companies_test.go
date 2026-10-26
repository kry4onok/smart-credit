package demodata

import "testing"

func TestCompanyCatalogue(t *testing.T) {
	seenNumber, seenINN, seenEmail := map[int]bool{}, map[string]bool{}, map[string]bool{}
	for _, c := range Companies() {
		if c.Number < 1 || seenNumber[c.Number] || seenINN[c.INN] || seenEmail[c.Email] {
			t.Fatal("duplicate or invalid demo identity", c.Number)
		}
		seenNumber[c.Number], seenINN[c.INN], seenEmail[c.Email] = true, true, true
		if c.MonthlyReceiptsKopecks <= 0 || c.InitialCashKopecks < 0 || c.LargestCustomerShareBPS < 0 || c.LargestCustomerShareBPS > 10000 || c.SuggestedAmountKopecks <= 0 || c.SuggestedTermMonths < 1 || c.Title == "" || c.Description == "" {
			t.Fatal("incomplete demo scenario", c.Email)
		}
		if c.MonthlyReceiptsKopecks-c.MonthlyReceiptsKopecks*int64(c.MarginBPS)/10000-25000000 < 0 {
			t.Fatal("demo cash components would be negative", c.Email)
		}
	}
	copies := Companies()
	original := copies[0].Name
	copies[0].Name = "changed"
	again, ok := Lookup(copies[0].INN)
	if !ok || again.Name != original {
		t.Fatal("catalogue was mutated")
	}
	if _, ok := Lookup("unknown"); ok {
		t.Fatal("unknown company synthesized")
	}
}
