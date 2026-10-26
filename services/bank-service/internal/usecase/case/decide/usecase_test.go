package decide

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/google/uuid"
)

func TestValidateDecisionShapes(t *testing.T) {
	base := Input{CaseID: uuid.New(), AnalystID: uuid.New(), IdempotencyKey: uuid.New(), ExpectedVersion: 2, Comment: "Internal review", Outcome: "APPROVE", Offer: &Offer{ProductID: uuid.New(), AmountKopecks: 100000000, TermMonths: 12, RateBPS: 1600}}
	tests := []struct {
		name   string
		change func(*Input)
		valid  bool
	}{
		{"approve", func(*Input) {}, true},
		{"missing comment", func(i *Input) { i.Comment = " " }, false},
		{"counter requires public reason", func(i *Input) { i.Outcome = "COUNTER" }, false},
		{"counter", func(i *Input) { i.Outcome = "COUNTER"; i.PublicReason = "Изменены условия" }, true},
		{"request info requires fields", func(i *Input) {
			i.Outcome = "REQUEST_INFO"
			i.Offer = nil
			i.InfoRequest = &InfoRequest{Message: "Дополните заявку"}
		}, false},
		{"request info", func(i *Input) {
			i.Outcome = "REQUEST_INFO"
			i.Offer = nil
			i.InfoRequest = &InfoRequest{Message: "Дополните заявку", RequiredDocumentTypes: []string{"BANK_STATEMENT"}}
		}, true},
		{"v2 request", func(i *Input) {
			i.Outcome = "REQUEST_INFO"
			i.Offer = nil
			i.InfoRequest = &InfoRequest{Message: "Дополните заявку", V2: true, RequirementIDs: []string{"id"}}
		}, true},
		{"v2 mixed request", func(i *Input) {
			i.Outcome = "REQUEST_INFO"
			i.Offer = nil
			i.InfoRequest = &InfoRequest{Message: "Дополните заявку", V2: true, RequirementIDs: []string{"id"}, RequiredFields: []string{"COMPANY_REVENUE_12M"}}
		}, false},
		{"reject requires reason", func(i *Input) { i.Outcome = "REJECT"; i.Offer = nil }, false},
		{"reject", func(i *Input) {
			i.Outcome = "REJECT"
			i.Offer = nil
			i.PublicReason = "Не соответствует условиям"
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := base
			tt.change(&in)
			err := Validate(in)
			if tt.valid && err != nil || !tt.valid && !errors.Is(err, ErrValidation) {
				t.Fatalf("validation error = %v, valid = %v", err, tt.valid)
			}
		})
	}
}

type decisionRepositoryStub struct {
	input    Input
	preview  Preview
	ready    bool
	prepared bool
}

func (s *decisionRepositoryStub) PrepareDecision(_ context.Context, _ Input) (Preview, bool) {
	s.prepared = true
	return s.preview, s.ready
}

func (s *decisionRepositoryStub) Decide(_ context.Context, input Input) (model.Case, error) {
	s.input = input
	return model.Case{}, nil
}

type decisionTransactionStub struct{ active bool }

func (s *decisionTransactionStub) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	s.active = true
	defer func() { s.active = false }()
	return fn(ctx)
}

type explanationStub struct {
	facts  Facts
	text   string
	err    error
	called bool
	inTx   *decisionTransactionStub
}

func (s *explanationStub) Explain(_ context.Context, facts Facts) (string, error) {
	if s.inTx.active {
		return "", errors.New("assistant called inside transaction")
	}
	s.called = true
	s.facts = facts
	return s.text, s.err
}

func TestExecuteExplainsOutsideTransactionAndKeepsFallback(t *testing.T) {
	for _, tt := range []struct {
		name     string
		text     string
		err      error
		ready    bool
		expected string
		called   bool
	}{
		{name: "assistant", text: "  Текст клиенту.  ", ready: true, expected: "Текст клиенту.", called: true},
		{name: "unavailable", err: errors.New("unavailable"), ready: true, called: true},
		{name: "empty", text: "   ", ready: true, called: true},
		{name: "oversized", text: strings.Repeat("а", 1001), ready: true, called: true},
		{name: "invalid or replay", text: "Текст", ready: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &decisionRepositoryStub{preview: Preview{RequestedProductName: "Оборотный кредит", OfferedProductName: "Гибкий кредит"}, ready: tt.ready}
			tx := &decisionTransactionStub{}
			assistant := &explanationStub{text: tt.text, err: tt.err, inTx: tx}
			input := Input{CaseID: uuid.New(), AnalystID: uuid.New(), IdempotencyKey: uuid.New(), ExpectedVersion: 1,
				Outcome: "COUNTER", Comment: "Внутренний комментарий", PublicReason: "Предлагаем другие условия.",
				Offer: &Offer{ProductID: uuid.New(), AmountKopecks: 100000000, TermMonths: 12, RateBPS: 1600}}
			if _, err := New(repo, tx, assistant).Execute(context.Background(), input); err != nil {
				t.Fatal(err)
			}
			if assistant.called != tt.called || repo.input.ClientText != tt.expected {
				t.Fatalf("called=%t, client text=%q", assistant.called, repo.input.ClientText)
			}
			if tt.called && (assistant.facts.ApplicationID != input.CaseID.String() || assistant.facts.Outcome != "COUNTER" ||
				assistant.facts.ProductName != "Оборотный кредит" || assistant.facts.PublicReason != input.PublicReason ||
				assistant.facts.Offer == nil || assistant.facts.Offer.ProductName != "Гибкий кредит" ||
				assistant.facts.Offer.AmountKopecks != input.Offer.AmountKopecks) {
				t.Fatalf("assistant facts = %+v", assistant.facts)
			}
		})
	}
}

func TestExecuteApproveHasTextWithoutAssistant(t *testing.T) {
	repo := &decisionRepositoryStub{}
	tx := &decisionTransactionStub{}
	input := Input{CaseID: uuid.New(), AnalystID: uuid.New(), IdempotencyKey: uuid.New(), ExpectedVersion: 1,
		Outcome: "APPROVE", Comment: "Внутренний комментарий",
		Offer: &Offer{ProductID: uuid.New(), AmountKopecks: 100000000, TermMonths: 12, RateBPS: 1600}}
	if _, err := New(repo, tx, nil).Execute(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if repo.prepared || repo.input.ClientText != "По вашей заявке принято решение об одобрении." {
		t.Fatalf("prepared=%t client text=%q", repo.prepared, repo.input.ClientText)
	}
}
