package decide

import (
	"errors"
	"testing"

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
