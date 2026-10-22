package scoring

import (
	"errors"
	"fmt"
	"math"
	"time"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
	"github.com/google/uuid"
)

const FeatureSchemaVersion = "borrower-features-v1"

const maxMoneyKopecks int64 = 9_007_199_254_740_991

// Input contains the saved application snapshot and successful enrichment
// responses for one assessment attempt. The caller owns attempt/revision checks.
// AsOfDate is fixed when the attempt starts and reused on retries.
type Input struct {
	ApplicationID uuid.UUID
	ProductID     uuid.UUID
	AsOfDate      time.Time
	Company       *commonpb.CompanySnapshot
	Profile       *bureaupb.CompanyProfileResponse
	CreditHistory *bureaupb.CreditHistoryResponse
}

func BuildRequest(input Input) (*scoringpb.ScoreRequest, error) {
	if input.ApplicationID == uuid.Nil || input.ProductID == uuid.Nil {
		return nil, errors.New("scoring application and product IDs must be nonzero UUIDs")
	}
	if input.AsOfDate.IsZero() {
		return nil, errors.New("scoring as_of_date is required")
	}
	if input.Company == nil || input.Profile == nil || input.CreditHistory == nil {
		return nil, errors.New("scoring requires the saved company snapshot and successful bureau responses")
	}

	revenue, err := copyMoney("revenue_12m", input.Company.Revenue_12M, false)
	if err != nil {
		return nil, err
	}

	profit, err := copyMoney("profit_12m", input.Company.Profit_12M, true)
	if err != nil {
		return nil, err
	}

	debt, err := copyMoney("total_debt", input.CreditHistory.TotalDebt, false)
	if err != nil {
		return nil, err
	}

	var age *int32
	if input.Profile.RegisteredAt != "" {
		registeredAt, parseErr := time.Parse("2006-01-02", input.Profile.RegisteredAt)
		if parseErr != nil || registeredAt.Format("2006-01-02") != input.Profile.RegisteredAt {
			return nil, errors.New("invalid bureau registered_at date")
		}

		months, ageErr := companyAgeMonths(registeredAt, input.AsOfDate)
		if ageErr != nil {
			return nil, ageErr
		}

		age = &months
	}

	var count *int32
	if input.CreditHistory.OverdueCount_12M != nil {
		value := *input.CreditHistory.OverdueCount_12M
		count = &value
	}

	if count != nil && *count < 0 {
		return nil, errors.New("negative overdue_count_12m")
	}

	var days *int32
	if input.CreditHistory.MaxOverdueDays != nil {
		value := *input.CreditHistory.MaxOverdueDays
		days = &value
	}

	if days != nil && *days < 0 {
		return nil, errors.New("negative max_overdue_days")
	}

	return &scoringpb.ScoreRequest{
		ApplicationId:        input.ApplicationID.String(),
		ProductId:            input.ProductID.String(),
		FeatureSchemaVersion: FeatureSchemaVersion,
		Features: &scoringpb.Features{
			Revenue_12M:      revenue,
			Profit_12M:       profit,
			TotalDebt:        debt,
			CompanyAgeMonths: age,
			OverdueCount_12M: count,
			MaxOverdueDays:   days,
		},
	}, nil
}

func copyMoney(name string, money *commonpb.Money, allowNegative bool) (*commonpb.Money, error) {
	if money == nil {
		return nil, nil
	}
	if money.Currency != "RUB" || money.AmountKopecks > maxMoneyKopecks ||
		money.AmountKopecks < -maxMoneyKopecks || (!allowNegative && money.AmountKopecks < 0) {
		return nil, fmt.Errorf("invalid scoring %s money", name)
	}
	return &commonpb.Money{AmountKopecks: money.AmountKopecks, Currency: money.Currency}, nil
}

func companyAgeMonths(registeredAt, asOfDate time.Time) (int32, error) {
	registeredAt = registeredAt.UTC()
	asOfDate = asOfDate.UTC()
	registeredDate := time.Date(registeredAt.Year(), registeredAt.Month(), registeredAt.Day(), 0, 0, 0, 0, time.UTC)
	asOf := time.Date(asOfDate.Year(), asOfDate.Month(), asOfDate.Day(), 0, 0, 0, 0, time.UTC)
	if registeredDate.After(asOf) {
		return 0, errors.New("bureau registered_at is after scoring as_of_date")
	}

	months := (asOf.Year()-registeredDate.Year())*12 + int(asOf.Month()-registeredDate.Month())
	lastDay := time.Date(asOf.Year(), asOf.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	anniversaryDay := min(registeredDate.Day(), lastDay)
	if asOf.Day() < anniversaryDay {
		months--
	}
	if months > math.MaxInt32 {
		return 0, errors.New("company age exceeds int32 months")
	}
	return int32(months), nil
}
