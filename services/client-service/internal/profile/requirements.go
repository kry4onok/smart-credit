package profile

import (
	"strings"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func CoverageHTTP(from, to *time.Time) *common.DatePeriod {
	if from == nil || to == nil {
		return nil
	}
	return &common.DatePeriod{From: openapi_types.Date{Time: *from}, To: openapi_types.Date{Time: *to}}
}
func RequirementHTTP(r *commonpb.Requirement) common.Requirement {
	v := common.Requirement{Id: r.GetId(), Code: r.GetCode(), Label: r.GetLabel(), Reason: r.GetReason(), Kind: common.RequirementKind(strings.TrimPrefix(r.GetKind().String(), "REQUIREMENT_KIND_")), RequiredAt: common.RequirementRequiredAt(strings.TrimPrefix(r.GetRequiredAt().String(), "REQUIREMENT_STAGE_")), Status: common.RequirementStatus(strings.TrimPrefix(r.GetStatus().String(), "REQUIREMENT_STATUS_")), AcceptedDocumentTypes: []common.DocumentType{}, DocumentIds: []common.Id{}}
	if r.GetFieldPath() != "" {
		v.FieldPath = Ptr(r.GetFieldPath())
	}
	if r.GetPeriod() != nil {
		v.Period = &common.DatePeriod{From: date(r.Period.From), To: date(r.Period.To)}
	}
	if r.GetTaxRegime() != 0 {
		v.TaxRegime = Ptr(common.TaxRegime(strings.TrimPrefix(r.GetTaxRegime().String(), "TAX_REGIME_")))
	}
	for _, t := range r.GetAcceptedDocumentTypes() {
		v.AcceptedDocumentTypes = append(v.AcceptedDocumentTypes, common.DocumentType(strings.TrimPrefix(t.String(), "DOCUMENT_TYPE_")))
	}
	for _, id := range r.GetDocumentIds() {
		v.DocumentIds = append(v.DocumentIds, uuid.MustParse(id))
	}
	return v
}
func RequirementsHTTP(items []*commonpb.Requirement) *[]common.Requirement {
	if len(items) == 0 {
		return nil
	}
	v := make([]common.Requirement, 0, len(items))
	for _, r := range items {
		v = append(v, RequirementHTTP(r))
	}
	return &v
}
func PlanHTTP(p *commonpb.RequirementsPlan) *common.RequirementsPlan {
	if p == nil {
		return nil
	}
	v := &common.RequirementsPlan{CanSubmit: p.CanSubmit, PolicyVersion: common.RequirementsPlanPolicyVersion(p.PolicyVersion), ReferenceDate: date(p.ReferenceDate), Route: common.RequirementsPlanRoute(strings.TrimPrefix(p.Route.String(), "REQUIREMENTS_ROUTE_")), ReasonCodes: append([]string{}, p.ReasonCodes...), Items: []common.Requirement{}}
	for _, r := range p.Items {
		v.Items = append(v.Items, RequirementHTTP(r))
	}
	return v
}
