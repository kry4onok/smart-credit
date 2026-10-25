package summary

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("case not found")

type Repository interface {
	GetCase(context.Context, uuid.UUID) (model.Case, bool, error)
}

type Facts struct {
	ApplicationID      string
	ProductName        string
	Purpose            string
	PurposeDescription string
	Status             string
	StatusDescription  string
	Grade              string
	ReasonCodes        []string
	TriggeredRuleCodes []string
	ReasonFacts        []DescribedCode
	RuleFacts          []DescribedCode
}

type DescribedCode struct {
	Code        string
	Description string
}

type Assistant interface {
	Summarize(context.Context, Facts) (text, source string, err error)
}

type Output struct {
	Text   string
	Source string
}

type UseCase struct {
	repository Repository
	assistant  Assistant
}

func New(repository Repository, assistant Assistant) *UseCase {
	return &UseCase{repository: repository, assistant: assistant}
}

func (uc *UseCase) Execute(ctx context.Context, caseID uuid.UUID) (Output, error) {
	caseItem, found, err := uc.repository.GetCase(ctx, caseID)
	if err != nil {
		return Output{}, fmt.Errorf("get case for summary: %w", err)
	}
	if !found {
		return Output{}, ErrNotFound
	}
	facts := factsFromCase(caseItem)
	if uc.assistant != nil {
		text, source, err := uc.assistant.Summarize(ctx, facts)
		if err == nil && strings.TrimSpace(text) != "" && (source == "assistant" || source == "template") {
			return Output{Text: strings.TrimSpace(text), Source: source}, nil
		}
	}
	return Output{Text: template(facts), Source: "template"}, nil
}

func factsFromCase(caseItem model.Case) Facts {
	facts := Facts{
		ApplicationID: caseItem.ID.String(), ProductName: caseItem.ProductName,
		Purpose:            string(caseItem.Application.Purpose),
		PurposeDescription: purposeDescriptions[string(caseItem.Application.Purpose)],
		Status:             string(caseItem.Status),
		StatusDescription:  statusDescriptions[caseItem.Status],
		TriggeredRuleCodes: append([]string(nil), caseItem.TriggeredRules...),
	}
	for _, code := range facts.TriggeredRuleCodes {
		facts.RuleFacts = append(facts.RuleFacts, DescribedCode{Code: code, Description: ruleDescriptions[code]})
	}
	if caseItem.Score != nil && caseItem.Score.AssessmentStatus == "ASSESSED" {
		facts.Grade = string(caseItem.Score.Grade)
		for _, reason := range caseItem.Score.Reasons {
			code := string(reason.Code)
			facts.ReasonCodes = append(facts.ReasonCodes, code)
			facts.ReasonFacts = append(facts.ReasonFacts, DescribedCode{Code: code, Description: strings.TrimSpace(reason.Description)})
		}
	}
	return facts
}

func template(facts Facts) string {
	parts := make([]string, 0, 6)
	if facts.ProductName != "" {
		parts = append(parts, "Продукт: "+facts.ProductName+".")
	}
	if facts.Purpose != "" {
		purpose := facts.Purpose
		if facts.PurposeDescription != "" {
			purpose = facts.PurposeDescription
		}
		parts = append(parts, "Цель: "+purpose+".")
	}
	if facts.StatusDescription != "" {
		parts = append(parts, "Статус: "+facts.StatusDescription+".")
	} else if facts.Status != "" {
		parts = append(parts, "Статус: "+facts.Status+".")
	}
	if facts.Grade != "" {
		parts = append(parts, "Грейд скоринга: "+facts.Grade+".")
	}
	if len(facts.ReasonCodes) > 0 {
		parts = append(parts, "Факторы скоринга: "+formatFacts(facts.ReasonFacts, facts.ReasonCodes)+".")
	}
	if len(facts.TriggeredRuleCodes) > 0 {
		parts = append(parts, "Правила банка: "+formatFacts(facts.RuleFacts, facts.TriggeredRuleCodes)+".")
	}
	if len(parts) == 0 {
		return "Данные для резюме не переданы."
	}
	return strings.Join(parts, " ")
}

func formatFacts(described []DescribedCode, codes []string) string {
	items := make([]string, 0, len(codes))
	for i, code := range codes {
		item := code
		if i < len(described) && described[i].Code == code && described[i].Description != "" {
			item = described[i].Description
		}
		items = append(items, strings.TrimRight(item, "."))
	}
	return strings.Join(items, "; ")
}
