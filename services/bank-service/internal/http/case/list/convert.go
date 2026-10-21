package listhandler

import (
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	listcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/list"
)

func toModelStatus(status common.BankStatus) model.CaseStatus {
	return model.CaseStatus(status)
}

func toResponse(output listcase.Output) generated.ListCases200JSONResponse {
	items := make([]generated.CaseListItem, 0, len(output.Items))
	for _, item := range output.Items {
		companyName := item.CompanyName
		items = append(items, generated.CaseListItem{
			Id:        item.ID,
			Status:    common.BankStatus(item.Status),
			ProductId: item.ProductID,
			RequestedAmount: common.Money{
				AmountKopecks: item.RequestedAmountKopecks,
				Currency:      common.MoneyCurrencyRUB,
			},
			CompanyName: &companyName,
			AssigneeId:  item.AssigneeID,
			CreatedAt:   item.CreatedAt,
			Version:     item.Version,
		})
	}

	return generated.ListCases200JSONResponse{
		Items:      items,
		NextCursor: output.NextCursor,
	}
}
