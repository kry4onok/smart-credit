package get

import (
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func toResponse(value model.Application) generated.ClientApplication {
	response := generated.ClientApplication{
		Id:             common.Id(value.ID),
		ProductId:      common.Id(value.ProductID),
		ProductVersion: value.ProductVersion,
		Company:        companyProfile(value.Company),
		Status:         common.ClientStatus(value.Status),
		StageLabel:     value.StageLabel,
		NextAction:     value.NextAction,
		AllowedActions: allowedActions(value.AllowedActions),
		Documents:      documents(value.Documents),
		CreatedAt:      value.CreatedAt,
		UpdatedAt:      value.UpdatedAt,
		Version:        value.Version,
	}
	if value.Form != nil {
		response.Form = &generated.ApplicationForm{
			Amount:     common.Money{AmountKopecks: value.Form.AmountKopecks, Currency: common.MoneyCurrencyRUB},
			TermMonths: value.Form.TermMonths,
			Purpose:    common.PurposeTag(value.Form.Purpose),
		}
	}
	if value.PendingAction != nil {
		pending := generated.ClientApplicationPendingAction(*value.PendingAction)
		response.PendingAction = &pending
	}
	if value.PublicReason != nil {
		response.PublicReason = value.PublicReason
	}
	if value.InfoRequest != nil {
		response.InfoRequest = infoRequest(value.InfoRequest)
	}
	if value.Offer != nil {
		response.Offer = offer(value.Offer)
	}
	return response
}

func companyProfile(value model.Company) common.CompanyProfile {
	return common.CompanyProfile{
		Inn: value.INN, Name: value.Name, Okved: value.OKVED,
		RegisteredAt: openapi_types.Date{Time: value.RegisteredAt},
		Revenue12m:   common.Money{AmountKopecks: value.Revenue12mKopecks, Currency: common.MoneyCurrencyRUB},
		Profit12m:    common.SignedMoney{AmountKopecks: value.Profit12mKopecks, Currency: common.SignedMoneyCurrencyRUB},
		Obligations:  common.Money{AmountKopecks: value.ObligationsKopecks, Currency: common.MoneyCurrencyRUB},
	}
}

func allowedActions(values []string) []generated.ClientApplicationAllowedActions {
	result := make([]generated.ClientApplicationAllowedActions, len(values))
	for i, value := range values {
		result[i] = generated.ClientApplicationAllowedActions(value)
	}
	return result
}

func documents(values []model.ApplicationDocument) []common.DocumentMetadata {
	result := make([]common.DocumentMetadata, len(values))
	for i, value := range values {
		result[i] = common.DocumentMetadata{
			Id:           common.Id(value.ID),
			DocumentType: common.DocumentType(value.DocumentType),
			FileName:     value.FileName,
			Sha256:       value.Sha256,
			IsMock:       value.IsMock,
			UploadedAt:   value.UploadedAt,
		}
	}
	return result
}

func infoRequest(value *model.ApplicationInfoRequest) *common.InfoRequest {
	fields := make([]common.InfoField, len(value.RequiredFields))
	for i, field := range value.RequiredFields {
		fields[i] = common.InfoField(field)
	}
	documentTypes := make([]common.DocumentType, len(value.RequiredDocumentTypes))
	for i, documentType := range value.RequiredDocumentTypes {
		documentTypes[i] = common.DocumentType(documentType)
	}
	return &common.InfoRequest{
		Id:                    common.Id(value.ID),
		Message:               value.Message,
		RequiredFields:        fields,
		RequiredDocumentTypes: documentTypes,
		CreatedAt:             value.CreatedAt,
	}
}

func offer(value *model.ApplicationOffer) *common.Offer {
	return &common.Offer{
		Id:             common.Id(value.ID),
		Kind:           common.OfferKind(value.Kind),
		ProductId:      common.Id(value.ProductID),
		ProductVersion: value.ProductVersion,
		ProductName:    value.ProductName,
		Amount:         common.Money{AmountKopecks: value.AmountKopecks, Currency: common.MoneyCurrencyRUB},
		TermMonths:     value.TermMonths,
		RateBps:        value.RateBps,
		Explanation:    value.Explanation,
		ExpiresAt:      value.ExpiresAt,
	}
}
