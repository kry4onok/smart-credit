package validation

import (
	"encoding/json"
	"errors"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/requirements"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func evaluateV2(snapshot Snapshot, product Product) Outcome {
	manual := func(code, reason string) Outcome {
		return Outcome{Status: model.CaseStatusManualReview, Reason: code, PublicReason: reason}
	}
	if !requirements.SupportedPolicy(product.PolicyVersion) {
		return manual("UNKNOWN_REQUIREMENTS_POLICY", "Версия правил продукта требует проверки сотрудником")
	}
	if len(product.RequiredDocumentTypes) > 0 {
		return manual("INVALID_PRODUCT_POLICY", "Набор правил продукта требует проверки сотрудником")
	}
	if snapshot.ReferenceDate == nil {
		return manual("INVALID_PROFILE", "Отсутствует закреплённая дата требований")
	}
	company := new(commonpb.CompanySnapshot)
	if err := protojson.Unmarshal(snapshot.CompanyJSON, company); err != nil || company.GetProfileSchemaVersion() != 2 {
		return manual("INVALID_PROFILE", "Не удалось прочитать профиль v2")
	}
	var stored []receive.DocumentSnapshot
	if err := json.Unmarshal(snapshot.DocumentsJSON, &stored); err != nil {
		return manual("INVALID_DOCUMENT_COVERAGE", "Не удалось прочитать документы")
	}
	documents := make([]*commonpb.DocumentMetadata, 0, len(stored))
	for _, d := range stored {
		value, ok := commonpb.DocumentType_value["DOCUMENT_TYPE_"+d.DocumentType]
		if d.DocumentID == "" || !ok || value == 0 || d.UploadedAt.IsZero() {
			return manual("INVALID_DOCUMENT_COVERAGE", "Некорректные метаданные документа")
		}
		documents = append(documents, &commonpb.DocumentMetadata{DocumentId: d.DocumentID, DocumentType: commonpb.DocumentType(value), FileName: d.FileName, Sha256: d.SHA256, UploadedAt: timestamppb.New(d.UploadedAt), IsMock: d.IsMock, RequirementId: d.RequirementID, CoveragePeriod: d.CoveragePeriod})
	}
	purpose, ok := commonpb.PurposeTag_value["PURPOSE_TAG_"+snapshot.PurposeTag]
	if !ok {
		return manual("INVALID_PROFILE", "Неизвестная цель кредита")
	}
	form := &commonpb.ApplicationForm{Amount: &commonpb.Money{AmountKopecks: snapshot.AmountKopecks, Currency: "RUB"}, TermMonths: snapshot.TermMonths, Purpose: commonpb.PurposeTag(purpose), PurposeDescription: snapshot.PurposeDescription}
	pbProduct := &commonpb.Product{RequirementsPolicyVersion: product.PolicyVersion, MinAmount: &commonpb.Money{AmountKopecks: product.MinAmountKopecks, Currency: "RUB"}, MaxAmount: &commonpb.Money{AmountKopecks: product.MaxAmountKopecks, Currency: "RUB"}, MinTermMonths: product.MinTermMonths, MaxTermMonths: product.MaxTermMonths}
	for _, tag := range product.PurposeTags {
		value, ok := commonpb.PurposeTag_value["PURPOSE_TAG_"+tag]
		if !ok {
			return manual("INVALID_PROFILE", "Неизвестная цель продукта")
		}
		pbProduct.PurposeTags = append(pbProduct.PurposeTags, commonpb.PurposeTag(value))
	}
	plan, err := requirements.Build(requirements.Input{Company: company, Product: pbProduct, Form: form, Documents: documents, ReferenceDate: *snapshot.ReferenceDate})
	if err != nil {
		code := "INVALID_PROFILE"
		var ruleError *requirements.RuleError
		if errors.As(err, &ruleError) {
			code = ruleError.Code
		}
		return manual(code, "Профиль содержит противоречивые данные")
	}
	if plan == nil {
		return manual("INVALID_PROFILE", "Не удалось вычислить требования")
	}
	if plan.GetRoute() == commonpb.RequirementsRoute_REQUIREMENTS_ROUTE_MANUAL_REVIEW {
		code := "PROFILE_V2_REQUIRES_REVIEW"
		if len(plan.ReasonCodes) > 0 {
			code = plan.ReasonCodes[0]
		}
		result := manual(code, "Профиль требует проверки сотрудником")
		result.Plan = plan
		return result
	}
	result := Outcome{Status: model.CaseStatusEnrichment, Plan: plan, RequestedRequirements: []*commonpb.Requirement{}}
	for _, item := range plan.GetItems() {
		if item.GetStatus() == commonpb.RequirementStatus_REQUIREMENT_STATUS_MISSING &&
			(item.GetRequiredAt() == commonpb.RequirementStage_REQUIREMENT_STAGE_SUBMIT || product.PolicyVersion != requirements.SourceFirstPolicyVersion && item.GetRequiredAt() == commonpb.RequirementStage_REQUIREMENT_STAGE_ASSESSMENT) {
			result.RequestedRequirements = append(result.RequestedRequirements, item)
		}
	}
	if len(result.RequestedRequirements) > 0 {
		result.Status = model.CaseStatusNeedsInfo
		result.Reason = "REQUIREMENTS_NOT_MET"
		result.PublicReason = "Для продолжения рассмотрения дополните данные заявки."
		return result
	}
	if !plan.CanSubmit {
		return manual("REQUIREMENTS_NOT_MET", "Параметры заявки не соответствуют продукту")
	}
	return result
}
