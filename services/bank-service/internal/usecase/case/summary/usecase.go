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
	Grade              string
	ReasonCodes        []string
	TriggeredRuleCodes []string
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
		TriggeredRuleCodes: append([]string(nil), caseItem.TriggeredRules...),
	}
	if caseItem.Score != nil && caseItem.Score.AssessmentStatus == "ASSESSED" {
		facts.Grade = string(caseItem.Score.Grade)
		for _, reason := range caseItem.Score.Reasons {
			facts.ReasonCodes = append(facts.ReasonCodes, string(reason.Code))
		}
	}
	return facts
}

func template(facts Facts) string {
	parts := make([]string, 0, 5)
	if facts.ProductName != "" {
		parts = append(parts, "Продукт: "+facts.ProductName+".")
	}
	if facts.Purpose != "" {
		parts = append(parts, "Цель: "+facts.Purpose+".")
	}
	if facts.Grade != "" {
		parts = append(parts, "Грейд скоринга: "+facts.Grade+".")
	}
	if len(facts.ReasonCodes) > 0 {
		parts = append(parts, "Коды причин: "+strings.Join(facts.ReasonCodes, ", ")+".")
	}
	if len(facts.TriggeredRuleCodes) > 0 {
		parts = append(parts, "Коды сработавших правил: "+strings.Join(facts.TriggeredRuleCodes, ", ")+".")
	}
	if len(parts) == 0 {
		return "Данные для резюме не переданы."
	}
	return strings.Join(parts, " ")
}
