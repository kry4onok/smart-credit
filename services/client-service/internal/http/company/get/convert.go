package get

import (
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func convert(c model.Company) generated.VersionedCompany {
	return generated.VersionedCompany{Version: c.Version, Profile: common.CompanyProfile{
		Inn: c.INN, Name: c.Name, Okved: c.OKVED,
		RegisteredAt: openapi_types.Date{Time: c.RegisteredAt},
		Revenue12m:   common.Money{AmountKopecks: c.Revenue12mKopecks, Currency: common.MoneyCurrencyRUB},
		Profit12m:    common.SignedMoney{AmountKopecks: c.Profit12mKopecks, Currency: common.SignedMoneyCurrencyRUB},
		Obligations:  common.Money{AmountKopecks: c.ObligationsKopecks, Currency: common.MoneyCurrencyRUB},
	}}
}
