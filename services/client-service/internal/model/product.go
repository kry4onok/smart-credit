package model

import "github.com/google/uuid"

type Product struct {
	ID                    uuid.UUID
	Version               int64
	Code                  string
	Name                  string
	Description           string
	PurposeTags           []string
	MinAmountKopecks      int64
	MaxAmountKopecks      int64
	MinTermMonths         int
	MaxTermMonths         int
	IndicativeRateBps     *int
	RequiredDocumentTypes []string
}

type ProductCatalog struct {
	Items   []Product
	Version int64
}
