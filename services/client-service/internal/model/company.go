package model

import "time"

type Company struct {
	INN                string
	Name               string
	OKVED              string
	RegisteredAt       time.Time
	Revenue12mKopecks  int64
	Profit12mKopecks   int64
	ObligationsKopecks int64
	Version            int64
}
