package creditcalc

import (
	"testing"
	"time"
)

func TestScheduleConservationAndRounding(t *testing.T) {
	for _, terms := range []Terms{{100_000_000, 12, 1800}, {100_000_000, 24, 1800}, {10_001, 12, 0}, {100_000_001, 36, 9999}} {
		s, err := BuildSchedule(terms, time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		var principal, total, interest int64
		for i, p := range s.Payments {
			principal += p.PrincipalKopecks
			total += p.PaymentKopecks
			interest += p.InterestKopecks
			if p.PaymentKopecks != p.PrincipalKopecks+p.InterestKopecks || p.BalanceKopecks < 0 {
				t.Fatal(p)
			}
			if i == 0 && p.Period != "2026-02" {
				t.Fatal(p.Period)
			}
		}
		if principal != terms.AmountKopecks || total != s.TotalPaymentKopecks || interest != s.TotalInterestKopecks || s.Payments[len(s.Payments)-1].BalanceKopecks != 0 {
			t.Fatal(s)
		}
	}
	a, _ := BuildSchedule(Terms{100_000_000, 12, 1800}, time.Now())
	b, _ := BuildSchedule(Terms{100_000_000, 24, 1800}, time.Now())
	if a.PaymentKopecks != 9167999 || b.PaymentKopecks >= a.PaymentKopecks || b.TotalInterestKopecks <= a.TotalInterestKopecks {
		t.Fatal(a.PaymentKopecks, b.PaymentKopecks)
	}
}
func TestScheduleRejectsBounds(t *testing.T) {
	for _, v := range []Terms{{0, 12, 1800}, {100, 0, 1800}, {100, 121, 1800}, {100, 12, -1}, {MaxMoney + 1, 12, 0}} {
		if _, err := BuildSchedule(v, time.Now()); err == nil {
			t.Fatal(v)
		}
	}
}
