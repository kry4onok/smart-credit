package summary

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/google/uuid"
)

type repositoryStub struct {
	caseItem model.Case
	found    bool
}

func (r repositoryStub) GetCase(context.Context, uuid.UUID) (model.Case, bool, error) {
	return r.caseItem, r.found, nil
}

type assistantStub struct {
	facts  Facts
	text   string
	source string
	err    error
}

func (a *assistantStub) Summarize(_ context.Context, facts Facts) (string, string, error) {
	a.facts = facts
	return a.text, a.source, a.err
}

func TestSummaryUsesMinimalCaseFacts(t *testing.T) {
	id := uuid.New()
	caseItem := model.Case{ID: id, ProductName: "Оборотный кредит", Status: model.CaseStatusManualReview,
		Application:    model.SubmittedApplication{Purpose: "WORKING_CAPITAL"},
		Score:          &model.ScoreResult{AssessmentStatus: "ASSESSED", Grade: "B", Reasons: []model.ScoreReason{{Code: "NO_OVERDUE", Description: "Нет просрочек"}}},
		TriggeredRules: []string{"REVIEW_REQUIRED"},
		Company:        model.Company{INN: "7707083893", Name: "Internal company name"}}
	assistant := &assistantStub{text: "  Краткое резюме.  ", source: "assistant"}
	result, err := New(repositoryStub{caseItem: caseItem, found: true}, assistant).Execute(context.Background(), id)
	if err != nil || result.Text != "Краткое резюме." || result.Source != "assistant" {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if assistant.facts.ApplicationID != id.String() || assistant.facts.ProductName != "Оборотный кредит" ||
		assistant.facts.Purpose != "WORKING_CAPITAL" || assistant.facts.Grade != "B" ||
		assistant.facts.PurposeDescription != "Пополнение оборотных средств" ||
		assistant.facts.Status != "MANUAL_REVIEW" || assistant.facts.StatusDescription != "Ручное рассмотрение" ||
		len(assistant.facts.ReasonCodes) != 1 || assistant.facts.ReasonCodes[0] != "NO_OVERDUE" ||
		len(assistant.facts.ReasonFacts) != 1 || assistant.facts.ReasonFacts[0].Description != "Нет просрочек" ||
		len(assistant.facts.TriggeredRuleCodes) != 1 || assistant.facts.TriggeredRuleCodes[0] != "REVIEW_REQUIRED" ||
		len(assistant.facts.RuleFacts) != 1 || assistant.facts.RuleFacts[0].Description != "Необходим ручной разбор; конкретная причина этим кодом не указана" {
		t.Fatalf("facts = %+v", assistant.facts)
	}
}

func TestSummaryFallsBackWhenAssistantFails(t *testing.T) {
	caseItem := model.Case{ID: uuid.New(), ProductName: "Оборотный кредит", Status: model.CaseStatusScoring,
		Application: model.SubmittedApplication{Purpose: "WORKING_CAPITAL"},
		Score:       &model.ScoreResult{AssessmentStatus: "ASSESSED", Grade: "B"},
		Company:     model.Company{INN: "7707083893"}}
	assistant := &assistantStub{err: errors.New("unavailable")}
	result, err := New(repositoryStub{caseItem: caseItem, found: true}, assistant).Execute(context.Background(), caseItem.ID)
	if err != nil || result.Source != "template" || !strings.Contains(result.Text, "Скоринговый грейд — B.") ||
		!strings.Contains(result.Text, "Текущий этап — «Расчёт скоринговой оценки».") ||
		strings.Contains(result.Text, "Продукт:") {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	if strings.Contains(result.Text, caseItem.Company.INN) || strings.Contains(result.Text, "одобр") {
		t.Fatalf("fallback claims extra facts: %q", result.Text)
	}
}

func TestUnknownRuleDoesNotLeakCode(t *testing.T) {
	caseItem := model.Case{ID: uuid.New(), Status: model.CaseStatusManualReview,
		TriggeredRules: []string{"NEW_RULE"}}
	facts := factsFromCase(caseItem)
	text := template(facts)
	if len(facts.RuleFacts) != 1 || facts.RuleFacts[0].Description == "" ||
		strings.Contains(text, "NEW_RULE") || !strings.Contains(text, "проверки аналитиком") {
		t.Fatalf("unexpected facts or template: %+v, %q", facts, text)
	}
}

func TestSummaryReturnsNotFoundWithoutAssistantCall(t *testing.T) {
	assistant := &assistantStub{}
	_, err := New(repositoryStub{}, assistant).Execute(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) || assistant.facts.ApplicationID != "" {
		t.Fatalf("error = %v, assistant facts = %+v", err, assistant.facts)
	}
}
