package gethandler

import (
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	getcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/get"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func toResponse(output getcase.Output) generated.GetCase200JSONResponse {
	etag := fmt.Sprintf("\"%d\"", output.Case.Version)

	return generated.GetCase200JSONResponse{
		Body:    ToBankCase(output),
		Headers: generated.GetCase200ResponseHeaders{ETag: &etag},
	}
}

func ToBankCase(output getcase.Output) generated.BankCase {
	caseItem := output.Case
	actions := make([]generated.BankCaseAllowedActions, len(output.AllowedActions))
	for index, action := range output.AllowedActions {
		actions[index] = generated.BankCaseAllowedActions(action)
	}
	documentTypes := make([]common.DocumentType, len(caseItem.Application.DocumentTypes))
	for index, documentType := range caseItem.Application.DocumentTypes {
		documentTypes[index] = common.DocumentType(documentType)
	}
	triggeredRules := append([]string{}, caseItem.TriggeredRules...)

	result := generated.BankCase{
		Id:             caseItem.ID,
		Status:         common.BankStatus(caseItem.Status),
		ProductId:      caseItem.ProductID,
		ProductName:    caseItem.ProductName,
		ProductVersion: caseItem.ProductVersion,
		Company:        toCompany(caseItem.Company),
		Application: generated.SubmittedApplication{
			Amount:        rub(caseItem.Application.AmountKopecks),
			TermMonths:    caseItem.Application.TermMonths,
			Purpose:       common.PurposeTag(caseItem.Application.Purpose),
			DocumentTypes: documentTypes,
		},
		TriggeredRules: &triggeredRules,
		AssigneeId:     caseItem.AssigneeID,
		PublicReason:   caseItem.PublicReason,
		AllowedActions: actions,
		AuditVersion:   caseItem.AuditVersion,
		CreatedAt:      caseItem.CreatedAt,
		UpdatedAt:      caseItem.UpdatedAt,
		Version:        caseItem.Version,
	}

	if caseItem.Checks != nil {
		result.Checks = toChecks(*caseItem.Checks)
	}
	if caseItem.Score != nil {
		result.Score = toScore(*caseItem.Score)
	}
	if caseItem.Recommendation != nil {
		value := generated.BankCaseRecommendation(*caseItem.Recommendation)
		result.Recommendation = &value
	}
	if caseItem.InfoRequest != nil {
		result.InfoRequest = toInfoRequest(*caseItem.InfoRequest)
	}
	if caseItem.Offer != nil {
		result.Offer = toOffer(*caseItem.Offer)
	}

	return result
}

func toCompany(company model.Company) common.CompanyProfile {
	return common.CompanyProfile{
		Inn:          company.INN,
		Name:         company.Name,
		Okved:        company.OKVED,
		RegisteredAt: openapi_types.Date{Time: company.RegisteredAt},
		Revenue12m:   rub(company.Revenue12MKopecks),
		Profit12m: common.SignedMoney{
			AmountKopecks: company.Profit12MKopecks,
			Currency:      common.SignedMoneyCurrencyRUB,
		},
		Obligations: rub(company.ObligationsKopecks),
	}
}

func toChecks(checks model.Checks) *generated.Checks {
	result := &generated.Checks{
		CompanyStatus:   generated.ChecksCompanyStatus(checks.CompanyStatus),
		Okved:           checks.OKVED,
		TotalDebt:       rub(checks.TotalDebtKopecks),
		OverdueCount12m: checks.OverdueCount12M,
		MaxOverdueDays:  checks.MaxOverdueDays,
		BureauError:     checks.BureauError,
	}
	if checks.RegisteredAt != nil {
		result.RegisteredAt = &openapi_types.Date{Time: *checks.RegisteredAt}
	}

	return result
}

func toScore(score model.ScoreResult) *generated.ScoreResult {
	reasons := make([]generated.ScoreReason, len(score.Reasons))
	for index, reason := range score.Reasons {
		reasons[index] = generated.ScoreReason{
			Code:         generated.ScoreReasonCode(reason.Code),
			Description:  reason.Description,
			Direction:    generated.ScoreReasonDirection(reason.Direction),
			Contribution: reason.Contribution,
		}
	}

	return &generated.ScoreResult{
		Score:                score.Score,
		Grade:                generated.ScoreResultGrade(score.Grade),
		Reasons:              reasons,
		ModelVersion:         score.ModelVersion,
		FeatureSchemaVersion: score.FeatureSchemaVersion,
	}
}

func toInfoRequest(infoRequest model.InfoRequest) *common.InfoRequest {
	fields := make([]common.InfoField, len(infoRequest.RequiredFields))
	for index, field := range infoRequest.RequiredFields {
		fields[index] = common.InfoField(field)
	}
	documentTypes := make([]common.DocumentType, len(infoRequest.RequiredDocumentTypes))
	for index, documentType := range infoRequest.RequiredDocumentTypes {
		documentTypes[index] = common.DocumentType(documentType)
	}

	return &common.InfoRequest{
		Id:                    infoRequest.ID,
		Message:               infoRequest.Message,
		RequiredFields:        fields,
		RequiredDocumentTypes: documentTypes,
		CreatedAt:             infoRequest.CreatedAt,
	}
}

func toOffer(offer model.Offer) *common.Offer {
	return &common.Offer{
		Id:             offer.ID,
		Kind:           common.OfferKind(offer.Kind),
		ProductId:      offer.ProductID,
		ProductVersion: offer.ProductVersion,
		ProductName:    offer.ProductName,
		Amount:         rub(offer.AmountKopecks),
		TermMonths:     offer.TermMonths,
		RateBps:        offer.RateBPS,
		ExpiresAt:      offer.ExpiresAt,
		Explanation:    offer.Explanation,
	}
}

func rub(amountKopecks int64) common.Money {
	return common.Money{
		AmountKopecks: amountKopecks,
		Currency:      common.MoneyCurrencyRUB,
	}
}
