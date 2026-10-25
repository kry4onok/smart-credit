package statement

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func example(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile("testdata/example.txt")
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestParseExample(t *testing.T) {
	result, err := Parse([]byte(example(t)))
	if err != nil {
		t.Fatal(err)
	}
	if result.Format != FormatDemoTXTV1 || result.DocumentType != DocumentType ||
		result.AccountOwner != "ООО «Демо-поставка»" || result.AccountID != "DEMO-ACCOUNT-001" ||
		result.PeriodStart.Format("2006-01-02") != "2026-10-01" ||
		result.PeriodEnd.Format("2006-01-02") != "2026-10-24" || result.Currency != CurrencyRUB {
		t.Fatalf("statement identity = %+v", result)
	}
	if result.OpeningBalanceKopecks != 50000000 || result.TotalCreditsKopecks != 250000000 ||
		result.TotalDebitsKopecks != 90000000 || result.ClosingBalanceKopecks != 210000000 {
		t.Fatalf("statement amounts = %+v", result)
	}
	if len(result.Operations) != 6 || result.Operations[0].ID != "DEMO-001" ||
		result.Operations[0].Direction != Credit || result.Operations[0].AmountKopecks != 120000000 ||
		result.Operations[5].Direction != Debit || result.Operations[5].AmountKopecks != 10000000 {
		t.Fatalf("statement operations = %+v", result.Operations)
	}
}

func TestParseAcceptsBOMAndCRLF(t *testing.T) {
	content := "\ufeff" + strings.ReplaceAll(example(t), "\n", "\r\n")
	if _, err := Parse([]byte(content)); err != nil {
		t.Fatal(err)
	}
}

func TestParseRejectsInvalidOrInconsistentStatements(t *testing.T) {
	base := example(t)
	for _, test := range []struct {
		name    string
		content []byte
		code    ErrorCode
		line    bool
	}{
		{"unsupported header", []byte(strings.Replace(base, "ДЕМОНСТРАЦИОННАЯ ВЫПИСКА ПО СЧЁТУ", "BANK STATEMENT", 1)), UnsupportedFormat, false},
		{"invalid UTF-8", []byte{0xff, 0xfe}, UnsupportedFormat, false},
		{"unsupported currency", []byte(strings.Replace(base, "Валюта: RUB", "Валюта: USD", 1)), UnsupportedFormat, true},
		{"missing field", []byte(strings.Replace(base, "Начало периода: 2026-10-01", "", 1)), InvalidField, true},
		{"reversed period", []byte(strings.Replace(base, "Конец периода: 2026-10-24", "Конец периода: 2026-09-30", 1)), InvalidField, true},
		{"bad amount", []byte(strings.Replace(base, "Входящий остаток: 500000.00", "Входящий остаток: 500000,00", 1)), InvalidField, true},
		{"amount overflow", []byte(strings.Replace(base, "Входящий остаток: 500000.00", "Входящий остаток: 92233720368547759.99", 1)), InvalidField, true},
		{"operation outside period", []byte(strings.Replace(base, "Дата: 2026-10-02", "Дата: 2026-09-30", 1)), InvalidField, true},
		{"unknown direction", []byte(strings.Replace(base, "Направление: Поступление", "Направление: Перевод", 1)), InvalidField, true},
		{"duplicate operation", []byte(strings.Replace(base, "Операция: DEMO-002", "Операция: DEMO-001", 1)), InvalidField, true},
		{"turnover mismatch", []byte(strings.Replace(base, "Всего поступлений: 2500000.00", "Всего поступлений: 2400000.00", 1)), InconsistentTotals, false},
		{"balance mismatch", []byte(strings.Replace(base, "Исходящий остаток: 2100000.00", "Исходящий остаток: 2000000.00", 1)), InconsistentTotals, false},
		{"too large", []byte(strings.Repeat("x", MaxFileSize+1)), LimitExceeded, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(test.content)
			var parseErr *ParseError
			if !errors.As(err, &parseErr) || parseErr.Code != test.code {
				t.Fatalf("error = %v, want %s", err, test.code)
			}
			if test.line && parseErr.Line == 0 {
				t.Fatalf("missing source line in %v", parseErr)
			}
		})
	}
}
