// Package statement parses the versioned Smart Credit demo TXT statement format.
// It does not interpret account turnover as revenue, profit, or obligations.
package statement

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	FormatDemoTXTV1 = "SMART_CREDIT_DEMO_STATEMENT_TXT_V1"
	DocumentType    = "BANK_STATEMENT"
	CurrencyRUB     = "RUB"
	MaxFileSize     = 5 * 1024 * 1024
	MaxOperations   = 10000
)

const heading = "ДЕМОНСТРАЦИОННАЯ ВЫПИСКА ПО СЧЁТУ"

type Direction string

const (
	Credit Direction = "CREDIT"
	Debit  Direction = "DEBIT"
)

type Operation struct {
	ID            string    `json:"id"`
	Date          time.Time `json:"date"`
	Direction     Direction `json:"direction"`
	AmountKopecks int64     `json:"amount_kopecks"`
	Counterparty  string    `json:"counterparty"`
	Purpose       string    `json:"purpose"`
}

type Result struct {
	Format                string      `json:"format"`
	DocumentType          string      `json:"document_type"`
	AccountOwner          string      `json:"account_owner"`
	AccountID             string      `json:"account_id"`
	PeriodStart           time.Time   `json:"period_start"`
	PeriodEnd             time.Time   `json:"period_end"`
	Currency              string      `json:"currency"`
	OpeningBalanceKopecks int64       `json:"opening_balance_kopecks"`
	TotalCreditsKopecks   int64       `json:"total_credits_kopecks"`
	TotalDebitsKopecks    int64       `json:"total_debits_kopecks"`
	ClosingBalanceKopecks int64       `json:"closing_balance_kopecks"`
	Operations            []Operation `json:"operations"`
}

type ErrorCode string

const (
	UnsupportedFormat  ErrorCode = "UNSUPPORTED_FORMAT"
	InvalidField       ErrorCode = "INVALID_FIELD"
	InconsistentTotals ErrorCode = "INCONSISTENT_TOTALS"
	LimitExceeded      ErrorCode = "LIMIT_EXCEEDED"
)

type ParseError struct {
	Code   ErrorCode
	Line   int
	Detail string
}

func (e *ParseError) Error() string {
	if e.Line == 0 {
		return fmt.Sprintf("%s: %s", e.Code, e.Detail)
	}
	return fmt.Sprintf("%s at line %d: %s", e.Code, e.Line, e.Detail)
}

type line struct {
	number int
	text   string
}

type parser struct {
	lines []line
	index int
}

func Parse(content []byte) (Result, error) {
	if len(content) > MaxFileSize {
		return Result{}, &ParseError{Code: LimitExceeded, Detail: "statement exceeds 5 MiB"}
	}
	if !utf8.Valid(content) || strings.ContainsRune(string(content), '\x00') {
		return Result{}, &ParseError{Code: UnsupportedFormat, Detail: "statement must be UTF-8 text"}
	}
	text := strings.TrimPrefix(string(content), "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if strings.ContainsRune(text, '\r') {
		return Result{}, &ParseError{Code: UnsupportedFormat, Detail: "unsupported line ending"}
	}
	p := parser{lines: nonemptyLines(text)}
	if !p.hasNext() || p.next().text != heading {
		return Result{}, &ParseError{Code: UnsupportedFormat, Detail: "missing demo statement heading"}
	}
	for p.hasNext() && !strings.HasPrefix(p.peek().text, "Тип документа:") {
		if p.peek().text == "ОСТАТКИ И ОБОРОТЫ" || p.peek().text == "ОПЕРАЦИИ" {
			return Result{}, invalid(p.peek().number, "missing document type")
		}
		p.next()
	}
	if !p.hasNext() {
		return Result{}, invalid(0, "missing document type")
	}
	value, err := p.field("Тип документа")
	if err != nil {
		return Result{}, err
	}
	if value != DocumentType {
		return Result{}, &ParseError{Code: UnsupportedFormat, Detail: "only BANK_STATEMENT is supported"}
	}
	result := Result{Format: FormatDemoTXTV1, DocumentType: DocumentType, Currency: CurrencyRUB,
		Operations: make([]Operation, 0)}
	if result.AccountOwner, err = p.field("Владелец счёта"); err != nil {
		return Result{}, err
	}
	if result.AccountID, err = p.field("Идентификатор счёта"); err != nil {
		return Result{}, err
	}
	if value, err = p.field("Начало периода"); err != nil {
		return Result{}, err
	}
	if result.PeriodStart, err = parseDate(value, p.previousLine()); err != nil {
		return Result{}, err
	}
	if value, err = p.field("Конец периода"); err != nil {
		return Result{}, err
	}
	if result.PeriodEnd, err = parseDate(value, p.previousLine()); err != nil {
		return Result{}, err
	}
	if result.PeriodEnd.Before(result.PeriodStart) {
		return Result{}, invalid(p.previousLine(), "period end precedes start")
	}
	if value, err = p.field("Валюта"); err != nil {
		return Result{}, err
	}
	if value != CurrencyRUB {
		return Result{}, &ParseError{Code: UnsupportedFormat, Line: p.previousLine(), Detail: "only RUB is supported"}
	}
	if p.hasNext() && strings.HasPrefix(p.peek().text, "Суммы указаны в ") {
		if p.next().text != "Суммы указаны в рублях; десятичный разделитель — точка." {
			return Result{}, invalid(p.previousLine(), "unsupported amount notation")
		}
	}
	if err := p.expect("ОСТАТКИ И ОБОРОТЫ"); err != nil {
		return Result{}, err
	}
	if result.OpeningBalanceKopecks, err = p.moneyField("Входящий остаток"); err != nil {
		return Result{}, err
	}
	if result.TotalCreditsKopecks, err = p.moneyField("Всего поступлений"); err != nil {
		return Result{}, err
	}
	if result.TotalDebitsKopecks, err = p.moneyField("Всего списаний"); err != nil {
		return Result{}, err
	}
	if result.ClosingBalanceKopecks, err = p.moneyField("Исходящий остаток"); err != nil {
		return Result{}, err
	}
	if err := p.expect("ОПЕРАЦИИ"); err != nil {
		return Result{}, err
	}
	seen := make(map[string]struct{})
	var credits, debits int64
	for p.hasNext() && p.peek().text != "ПРИМЕЧАНИЕ" {
		if len(result.Operations) >= MaxOperations {
			return Result{}, &ParseError{Code: LimitExceeded, Line: p.peek().number, Detail: "too many operations"}
		}
		operationLine := p.peek().number
		operation, parseErr := p.operation(result.PeriodStart, result.PeriodEnd)
		if parseErr != nil {
			return Result{}, parseErr
		}
		if _, duplicate := seen[operation.ID]; duplicate {
			return Result{}, invalid(operationLine, "duplicate operation id")
		}
		seen[operation.ID] = struct{}{}
		if operation.Direction == Credit {
			credits, err = addMoney(credits, operation.AmountKopecks)
		} else {
			debits, err = addMoney(debits, operation.AmountKopecks)
		}
		if err != nil {
			return Result{}, invalid(p.previousLine(), "operation total overflows")
		}
		result.Operations = append(result.Operations, operation)
	}
	if credits != result.TotalCreditsKopecks || debits != result.TotalDebitsKopecks {
		return Result{}, &ParseError{Code: InconsistentTotals, Detail: "operation sums differ from stated turnovers"}
	}
	available, err := addMoney(result.OpeningBalanceKopecks, result.TotalCreditsKopecks)
	if err != nil || result.TotalDebitsKopecks > available ||
		available-result.TotalDebitsKopecks != result.ClosingBalanceKopecks {
		return Result{}, &ParseError{Code: InconsistentTotals, Detail: "closing balance does not match opening balance and turnovers"}
	}
	return result, nil
}

func nonemptyLines(text string) []line {
	parts := strings.Split(text, "\n")
	result := make([]line, 0, len(parts))
	for index, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			result = append(result, line{number: index + 1, text: value})
		}
	}
	return result
}

func (p *parser) hasNext() bool { return p.index < len(p.lines) }
func (p *parser) peek() line    { return p.lines[p.index] }
func (p *parser) next() line {
	value := p.peek()
	p.index++
	return value
}
func (p *parser) previousLine() int { return p.lines[p.index-1].number }

func (p *parser) field(label string) (string, error) {
	if !p.hasNext() {
		return "", invalid(0, "missing "+label)
	}
	current := p.next()
	prefix := label + ": "
	if !strings.HasPrefix(current.text, prefix) {
		return "", invalid(current.number, "expected "+label)
	}
	value := strings.TrimSpace(strings.TrimPrefix(current.text, prefix))
	if value == "" {
		return "", invalid(current.number, "empty "+label)
	}
	return value, nil
}

func (p *parser) expect(value string) error {
	if !p.hasNext() {
		return invalid(0, "missing "+value)
	}
	current := p.next()
	if current.text != value {
		return invalid(current.number, "expected "+value)
	}
	return nil
}

func (p *parser) moneyField(label string) (int64, error) {
	value, err := p.field(label)
	if err != nil {
		return 0, err
	}
	return parseMoney(value, p.previousLine())
}

func (p *parser) operation(start, end time.Time) (Operation, error) {
	var result Operation
	var err error
	if result.ID, err = p.field("Операция"); err != nil {
		return Operation{}, err
	}
	value, err := p.field("Дата")
	if err != nil {
		return Operation{}, err
	}
	result.Date, err = parseDate(value, p.previousLine())
	if err != nil {
		return Operation{}, err
	}
	if result.Date.Before(start) || result.Date.After(end) {
		return Operation{}, invalid(p.previousLine(), "operation date is outside statement period")
	}
	value, err = p.field("Направление")
	if err != nil {
		return Operation{}, err
	}
	switch value {
	case "Поступление":
		result.Direction = Credit
	case "Списание":
		result.Direction = Debit
	default:
		return Operation{}, invalid(p.previousLine(), "unknown operation direction")
	}
	if result.AmountKopecks, err = p.moneyField("Сумма"); err != nil {
		return Operation{}, err
	}
	if result.AmountKopecks == 0 {
		return Operation{}, invalid(p.previousLine(), "operation amount must be positive")
	}
	if result.Counterparty, err = p.field("Контрагент"); err != nil {
		return Operation{}, err
	}
	if result.Purpose, err = p.field("Назначение"); err != nil {
		return Operation{}, err
	}
	return result, nil
}

func parseDate(value string, lineNumber int) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return time.Time{}, invalid(lineNumber, "date must be YYYY-MM-DD")
	}
	return parsed, nil
}

func parseMoney(value string, lineNumber int) (int64, error) {
	whole, fraction, ok := strings.Cut(value, ".")
	if !ok || whole == "" || len(fraction) != 2 {
		return 0, invalid(lineNumber, "amount must use rubles with two decimal places")
	}
	for _, character := range whole + fraction {
		if character < '0' || character > '9' {
			return 0, invalid(lineNumber, "amount must contain only digits and a dot")
		}
	}
	rubles, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, invalid(lineNumber, "amount overflows")
	}
	kopecks, _ := strconv.ParseInt(fraction, 10, 64)
	if rubles > (math.MaxInt64-kopecks)/100 {
		return 0, invalid(lineNumber, "amount overflows")
	}
	return rubles*100 + kopecks, nil
}

func addMoney(left, right int64) (int64, error) {
	if right > math.MaxInt64-left {
		return 0, fmt.Errorf("money overflow")
	}
	return left + right, nil
}

func invalid(lineNumber int, detail string) *ParseError {
	return &ParseError{Code: InvalidField, Line: lineNumber, Detail: detail}
}
