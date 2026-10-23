package list

import (
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	listapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/list"
)

func toResponse(output listapplication.Output) generated.ListApplications200JSONResponse {
	items := make([]generated.ApplicationListItem, 0, len(output.Items))
	for _, item := range output.Items {
		response := generated.ApplicationListItem{
			Id: item.ID, ProductId: item.ProductID, Status: common.ClientStatus(item.Status),
			NextAction: item.NextAction, CreatedAt: item.CreatedAt,
		}
		if item.AmountKopecks != nil {
			response.Amount = &common.Money{AmountKopecks: *item.AmountKopecks, Currency: common.MoneyCurrencyRUB}
		}
		items = append(items, response)
	}
	return generated.ListApplications200JSONResponse{Items: items, NextCursor: output.NextCursor}
}
