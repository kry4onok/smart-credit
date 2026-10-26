package product

import (
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

func ToAPI(value model.Product) common.Product {
	return common.Product{
		RequirementsPolicyVersion: policyVersion(value.RequirementsPolicyVersion),
		Id:                        common.Id(value.ID),
		Version:                   value.Version,
		Code:                      value.Code,
		Name:                      value.Name,
		Description:               value.Description,
		PurposeTags:               purposeTags(value.PurposeTags),
		MinAmount:                 common.Money{AmountKopecks: value.MinAmountKopecks, Currency: common.MoneyCurrencyRUB},
		MaxAmount:                 common.Money{AmountKopecks: value.MaxAmountKopecks, Currency: common.MoneyCurrencyRUB},
		MinTermMonths:             value.MinTermMonths,
		MaxTermMonths:             value.MaxTermMonths,
		IndicativeRateBps:         value.IndicativeRateBps,
		RequiredDocumentTypes:     documentTypes(value.RequiredDocumentTypes),
	}
}

func purposeTags(values []string) []common.PurposeTag {
	result := make([]common.PurposeTag, len(values))
	for i, value := range values {
		result[i] = common.PurposeTag(value)
	}
	return result
}

func documentTypes(values []string) []common.DocumentType {
	result := make([]common.DocumentType, len(values))
	for i, value := range values {
		result[i] = common.DocumentType(value)
	}
	return result
}

func policyVersion(v *string) *common.ProductRequirementsPolicyVersion {
	if v == nil {
		return nil
	}
	r := common.ProductRequirementsPolicyVersion(*v)
	return &r
}
