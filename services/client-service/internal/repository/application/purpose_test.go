package application

import (
	"errors"
	"testing"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

func TestOtherPurposePersistenceAndSubmissionIntegration(t *testing.T) {
	f := readyDraft(t)
	purpose, description := "OTHER", "Открываем точку: ремонт, оборудование и сырьё"
	if _, err := f.db.Exec(t.Context(), `UPDATE applications SET product_snapshot=jsonb_set(product_snapshot,'{purpose_tags}','["OTHER"]') WHERE id=$1`, f.draft.ID); err != nil {
		t.Fatal(err)
	}
	saved, err := f.repo.Update(t.Context(), model.UpdateApplicationParams{OrgID: f.params.OrgID, ApplicationID: f.draft.ID, ExpectedVersion: 1, Patch: model.ApplicationPatch{Purpose: &purpose}})
	if err != nil {
		t.Fatal(err)
	}
	submit := model.SubmitApplicationParams{OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID, IdempotencyID: uuid.New()}
	if _, err = f.repo.Submit(t.Context(), submit); !errors.Is(err, domain.ErrInvalidApplication) {
		t.Fatalf("OTHER without description: %v", err)
	}
	saved, err = f.repo.Update(t.Context(), model.UpdateApplicationParams{OrgID: f.params.OrgID, ApplicationID: f.draft.ID, ExpectedVersion: saved.Version, Patch: model.ApplicationPatch{PurposeDescription: &description}})
	if err != nil || saved.PurposeDescription != description || saved.Form.PurposeDescription != description {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
	got, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
	if err != nil || got.Form.PurposeDescription != description {
		t.Fatalf("reload=%+v err=%v", got, err)
	}
	if _, err = f.repo.Submit(t.Context(), submit); err != nil {
		t.Fatal(err)
	}
	form := f.event(t, "ApplicationSubmitted").GetApplicationSubmitted().GetForm()
	if form.GetPurpose() != commonpb.PurposeTag_PURPOSE_TAG_OTHER || form.GetPurposeDescription() != description {
		t.Fatalf("event form=%v", form)
	}
}
func TestUnsupportedPurposeCannotSubmitIntegration(t *testing.T) {
	f := readyDraft(t)
	purpose := "EQUIPMENT"
	if _, err := f.repo.Update(t.Context(), model.UpdateApplicationParams{OrgID: f.params.OrgID, ApplicationID: f.draft.ID, ExpectedVersion: 1, Patch: model.ApplicationPatch{Purpose: &purpose}}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Submit(t.Context(), model.SubmitApplicationParams{OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID, IdempotencyID: uuid.New()}); !errors.Is(err, domain.ErrInvalidApplication) {
		t.Fatalf("unsupported purpose submitted: %v", err)
	}
}
func TestPurposeDescriptionPartialDraftIntegration(t *testing.T) {
	f := newApplicationFixture(t)
	description := "Несколько расходов"
	got, err := f.repo.Update(t.Context(), model.UpdateApplicationParams{OrgID: f.params.OrgID, ApplicationID: f.draft.ID, ExpectedVersion: 1, Patch: model.ApplicationPatch{PurposeDescription: &description}})
	if err != nil || got.PurposeDescription != description || got.Form != nil {
		t.Fatalf("partial draft=%+v err=%v", got, err)
	}
	empty := ""
	got, err = f.repo.Update(t.Context(), model.UpdateApplicationParams{OrgID: f.params.OrgID, ApplicationID: f.draft.ID, ExpectedVersion: got.Version, Patch: model.ApplicationPatch{PurposeDescription: &empty}})
	if err != nil || got.PurposeDescription != "" {
		t.Fatalf("cleared draft=%+v err=%v", got, err)
	}
}
