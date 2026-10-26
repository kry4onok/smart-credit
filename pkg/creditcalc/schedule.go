// Package creditcalc implements fixed-rate RUB annuities in integer kopecks.
package creditcalc

import (
	"errors"
	"math/big"
	"time"
)

const MaxMoney int64 = 9_007_199_254_740_991

type Terms struct {
	AmountKopecks int64 `json:"amount_kopecks"`
	TermMonths    int   `json:"term_months"`
	RateBPS       int   `json:"rate_bps"`
}

type Payment struct {
	Number           int    `json:"number"`
	Period           string `json:"period"`
	PaymentKopecks   int64  `json:"payment_kopecks"`
	PrincipalKopecks int64  `json:"principal_kopecks"`
	InterestKopecks  int64  `json:"interest_kopecks"`
	BalanceKopecks   int64  `json:"balance_kopecks"`
}

type Schedule struct {
	Terms
	Currency             string    `json:"currency"`
	Method               string    `json:"method"`
	PaymentKopecks       int64     `json:"payment_kopecks"`
	TotalPaymentKopecks  int64     `json:"total_payment_kopecks"`
	TotalInterestKopecks int64     `json:"total_interest_kopecks"`
	Payments             []Payment `json:"payments"`
}

// Round rounds a nonnegative rational to kopecks, with ties away from zero.
func Round(v *big.Rat) int64 {
	n := new(big.Int).Mul(v.Num(), big.NewInt(2))
	n.Add(n, v.Denom())
	return n.Quo(n, new(big.Int).Mul(v.Denom(), big.NewInt(2))).Int64()
}

func BuildSchedule(t Terms, asOf time.Time) (Schedule, error) {
	if t.AmountKopecks <= 0 || t.AmountKopecks > MaxMoney || t.TermMonths < 1 || t.TermMonths > 120 || t.RateBPS < 0 || t.RateBPS > 10000 || asOf.IsZero() {
		return Schedule{}, errors.New("invalid fixed-rate RUB terms")
	}
	rate := big.NewRat(int64(t.RateBPS), 120000)
	annuity := big.NewRat(t.AmountKopecks, int64(t.TermMonths))
	if t.RateBPS > 0 {
		onePlus := new(big.Rat).Add(big.NewRat(1, 1), rate)
		power := big.NewRat(1, 1)
		for range t.TermMonths {
			power.Mul(power, onePlus)
		}
		annuity.Mul(big.NewRat(t.AmountKopecks, 1), rate)
		annuity.Mul(annuity, power)
		annuity.Quo(annuity, new(big.Rat).Sub(power, big.NewRat(1, 1)))
	}
	payment := Round(annuity)
	if payment <= 0 {
		return Schedule{}, errors.New("amount is too small for monthly rounding")
	}
	s := Schedule{Terms: t, Currency: "RUB", Method: "annuity-monthly-half-up-v1", PaymentKopecks: payment, Payments: make([]Payment, 0, t.TermMonths)}
	balance := t.AmountKopecks
	month := time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= t.TermMonths; i++ {
		interest := Round(new(big.Rat).Mul(big.NewRat(balance, 1), rate))
		principal := payment - interest
		if i == t.TermMonths || principal > balance {
			principal = balance
		}
		if principal < 0 {
			return Schedule{}, errors.New("negative amortization after rounding")
		}
		balance -= principal
		actual := principal + interest
		if s.TotalPaymentKopecks > MaxMoney-actual {
			return Schedule{}, errors.New("total exceeds safe integer range")
		}
		s.TotalPaymentKopecks += actual
		s.TotalInterestKopecks += interest
		s.Payments = append(s.Payments, Payment{Number: i, Period: month.AddDate(0, i, 0).Format("2006-01"), PaymentKopecks: actual, PrincipalKopecks: principal, InterestKopecks: interest, BalanceKopecks: balance})
	}
	return s, nil
}
