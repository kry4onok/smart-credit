package create

import (
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func toResponse(value model.Application) generated.ClientApplication {
	actions := make([]generated.ClientApplicationAllowedActions, len(value.AllowedActions))
	for i, action := range value.AllowedActions {
		actions[i] = generated.ClientApplicationAllowedActions(action)
	}
	return generated.ClientApplication{
		Id:             common.Id(value.ID),
		ProductId:      common.Id(value.ProductID),
		ProductVersion: value.ProductVersion,
		Company:        companyProfile(value.Company),
		Status:         common.ClientStatus(value.Status),
		StageLabel:     value.StageLabel,
		NextAction:     value.NextAction,
		AllowedActions: actions,
		Documents:      make([]common.DocumentMetadata, 0),
		CreatedAt:      value.CreatedAt,
		UpdatedAt:      value.UpdatedAt,
		Version:        value.Version,
	}
}

func companyProfile(value model.Company) common.CompanyProfile {
	return common.CompanyProfile{
		Inn: value.INN, Name: value.Name, Okved: value.OKVED,
		RegisteredAt: openapi_types.Date{Time: value.RegisteredAt},
		Revenue12m:   &common.Money{AmountKopecks: value.Revenue12mKopecks, Currency: common.MoneyCurrencyRUB},
		Profit12m:    &common.SignedMoney{AmountKopecks: value.Profit12mKopecks, Currency: common.SignedMoneyCurrencyRUB},
		Obligations:  &common.Money{AmountKopecks: value.ObligationsKopecks, Currency: common.MoneyCurrencyRUB},
	}
}
