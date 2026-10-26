package summary

import (
	"context"
	"errors"
	"fmt"
	"regexp"
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
		description := ruleDescriptions[code]
		if description == "" {
			description = "Сработало правило, требующее проверки аналитиком"
		}
		facts.RuleFacts = append(facts.RuleFacts, DescribedCode{Code: code, Description: description})
	}
	if caseItem.Score != nil && caseItem.Score.AssessmentStatus == "ASSESSED" {
		facts.Grade = string(caseItem.Score.Grade)
		for _, reason := range caseItem.Score.Reasons {
			code := string(reason.Code)
			facts.ReasonCodes = append(facts.ReasonCodes, code)
			description := strings.TrimSpace(reason.Description)
			if description == "" || internalCode.MatchString(description) {
				description = "Фактор скоринга без опубликованного описания"
			}
			facts.ReasonFacts = append(facts.ReasonFacts, DescribedCode{Code: code, Description: description})
		}
	}
	return facts
}

func template(facts Facts) string {
	parts := make([]string, 0, 6)
	purpose := displayValue(facts.Purpose, facts.PurposeDescription, "цель уточняется")
	status := displayValue(facts.Status, facts.StatusDescription, "уточняется")
	if facts.ProductName != "" {
		opening := "Рассматривается заявка на продукт «" + facts.ProductName + "»"
		if purpose != "" {
			opening += " с целью «" + purpose + "»"
		}
		parts = append(parts, opening+".")
	} else if purpose != "" {
		parts = append(parts, "В заявке указана цель «"+purpose+"».")
	}
	if status != "" {
		parts = append(parts, "Текущий этап — «"+status+"».")
	}
	if facts.Grade != "" {
		parts = append(parts, "Скоринговый грейд — "+facts.Grade+".")
	}
	if len(facts.ReasonCodes) > 0 {
		parts = append(parts, "В оценке отмечены "+formatFacts(facts.ReasonFacts, facts.ReasonCodes, "Фактор скоринга без опубликованного описания")+".")
	}
	if len(facts.TriggeredRuleCodes) > 0 {
		parts = append(parts, "По банковским правилам также отмечено: "+formatFacts(facts.RuleFacts, facts.TriggeredRuleCodes, "Сработало правило, требующее проверки аналитиком")+".")
	}
	if len(parts) == 0 {
		return "Данные для резюме не переданы."
	}
	return strings.Join(parts, " ")
}

var internalCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func displayValue(value, description, generic string) string {
	if description != "" {
		return description
	}
	if internalCode.MatchString(value) {
		return generic
	}
	return value
}

func formatFacts(described []DescribedCode, codes []string, generic string) string {
	items := make([]string, 0, len(codes))
	for i, code := range codes {
		item := generic
		if i < len(described) && described[i].Code == code && described[i].Description != "" && !internalCode.MatchString(described[i].Description) {
			item = described[i].Description
		}
		items = append(items, strings.TrimRight(item, "."))
	}
	return strings.Join(items, "; ")
}
