package application

import (
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/requirements"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/profile"
)

func referenceDateString(d *time.Time) string {
	if d == nil {
		return ""
	}
	return d.UTC().Format(time.DateOnly)
}
func coverageProto(from, to *time.Time) *commonpb.DatePeriod {
	if from == nil || to == nil {
		return nil
	}
	return &commonpb.DatePeriod{From: referenceDateString(from), To: referenceDateString(to)}
}
func buildPlan(c model.Company, p productSnapshotJSON, f *model.ApplicationForm, docs []model.ApplicationDocument, ref *time.Time) (*commonpb.RequirementsPlan, error) {
	if p.RequirementsPolicyVersion == nil {
		return nil, nil
	}
	if ref == nil || c.ProfileSchemaVersion == nil || *c.ProfileSchemaVersion != 2 || *p.RequirementsPolicyVersion != "sme-profile-v1" {
		return nil, &requirements.RuleError{Code: requirements.InvalidProfile, Issues: []requirements.Issue{{Path: "profile", Code: requirements.InvalidProfile, Message: "Неизвестная политика, версия профиля или дата требований"}}}
	}
	if err := domain.ValidateCompanyAt(c, *ref); err != nil {
		return nil, err
	}
	product := &commonpb.Product{ProductId: p.ID.String(), Version: p.Version, RequirementsPolicyVersion: *p.RequirementsPolicyVersion, MinAmount: moneyProto(p.MinAmount), MaxAmount: moneyProto(p.MaxAmount), MinTermMonths: int32(p.MinTermMonths), MaxTermMonths: int32(p.MaxTermMonths)}
	for _, t := range p.PurposeTags {
		product.PurposeTags = append(product.PurposeTags, commonpb.PurposeTag(commonpb.PurposeTag_value["PURPOSE_TAG_"+t]))
	}
	var form *commonpb.ApplicationForm
	if f != nil {
		form = formProto(*f)
		if f.AmountKopecks == 0 {
			form.Amount = nil
		}
	}
	return requirements.Build(requirements.Input{Company: profile.ToProto(c), Product: product, Form: form, Documents: documentMetadataProto(docs), ReferenceDate: *ref})
}
