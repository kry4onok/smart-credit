package caserepository

import "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"

type Queryer = transaction.Queryer

type rowScanner interface {
	Scan(dest ...any) error
}
