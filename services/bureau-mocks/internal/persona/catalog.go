package persona

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"gopkg.in/yaml.v3"
)

type Scenario struct {
	Profile      *bureaupb.CompanyProfileResponse
	Credit       *bureaupb.CreditHistoryResponse
	ProfileDelay time.Duration
	CreditDelay  time.Duration
	ProfileError bool
	CreditError  bool
}

type Catalog struct {
	byINN map[string]Scenario
}

type seed struct {
	Version  int           `yaml:"version"`
	Personas []personaSeed `yaml:"personas"`
}

type personaSeed struct {
	INN      string      `yaml:"inn"`
	Profile  profileSeed `yaml:"profile"`
	Credit   creditSeed  `yaml:"credit_history"`
	Behavior behavior    `yaml:"behavior"`
}

type profileSeed struct {
	LegalForm    string   `yaml:"legal_form"`
	Status       string   `yaml:"status"`
	RegisteredAt string   `yaml:"registered_at"`
	OKVED        string   `yaml:"okved"`
	DirectorName string   `yaml:"director_name"`
	Founders     []string `yaml:"founder_names"`
	Address      string   `yaml:"address"`
}

type moneySeed struct {
	AmountKopecks int64  `yaml:"amount_kopecks"`
	Currency      string `yaml:"currency"`
}

type obligationSeed struct {
	Kind        string    `yaml:"kind"`
	Outstanding moneySeed `yaml:"outstanding"`
}

type creditSeed struct {
	TotalDebt       *moneySeed       `yaml:"total_debt"`
	Obligations     []obligationSeed `yaml:"obligations"`
	OverdueCount12M *int32           `yaml:"overdue_count_12m"`
	MaxOverdueDays  *int32           `yaml:"max_overdue_days"`
}

type behavior struct {
	ProfileDelayMS int  `yaml:"profile_delay_ms"`
	CreditDelayMS  int  `yaml:"credit_delay_ms"`
	ProfileError   bool `yaml:"profile_unavailable"`
	CreditError    bool `yaml:"credit_unavailable"`
}

func Load(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bureau personas: %w", err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var input seed
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("decode bureau personas: %w", err)
	}

	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return nil, errors.New("bureau personas must contain one YAML document")
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decode bureau personas: %w", err)
	}
	if input.Version != 1 || len(input.Personas) == 0 {
		return nil, errors.New("bureau personas require version 1 and at least one persona")
	}

	catalog := &Catalog{byINN: make(map[string]Scenario, len(input.Personas))}
	for _, item := range input.Personas {
		if !ValidINN(item.INN) {
			return nil, fmt.Errorf("invalid bureau persona INN %q", item.INN)
		}
		if _, exists := catalog.byINN[item.INN]; exists {
			return nil, fmt.Errorf("duplicate bureau persona INN %q", item.INN)
		}

		scenario, err := convert(item)
		if err != nil {
			return nil, fmt.Errorf("bureau persona %s: %w", item.INN, err)
		}

		catalog.byINN[item.INN] = scenario
	}

	return catalog, nil
}

func (c *Catalog) Lookup(inn string) (Scenario, bool) {
	value, ok := c.byINN[inn]
	return value, ok
}

func convert(item personaSeed) (Scenario, error) {
	legalForm := commonpb.LegalForm_LEGAL_FORM_UNSPECIFIED
	switch item.Profile.LegalForm {
	case "":
	case "IP":
		if len(item.INN) != 12 {
			return Scenario{}, errors.New("IP requires a 12-digit INN")
		}
		legalForm = commonpb.LegalForm_LEGAL_FORM_IP
	case "LLC":
		if len(item.INN) != 10 {
			return Scenario{}, errors.New("LLC requires a 10-digit INN")
		}
		legalForm = commonpb.LegalForm_LEGAL_FORM_LLC
	default:
		return Scenario{}, errors.New("unknown legal form")
	}

	statuses := map[string]bureaupb.CompanyStatus{
		"ACTIVE":      bureaupb.CompanyStatus_COMPANY_STATUS_ACTIVE,
		"LIQUIDATING": bureaupb.CompanyStatus_COMPANY_STATUS_LIQUIDATING,
		"LIQUIDATED":  bureaupb.CompanyStatus_COMPANY_STATUS_LIQUIDATED,
	}

	companyStatus, ok := statuses[item.Profile.Status]
	if !ok {
		return Scenario{}, errors.New("unknown company status")
	}

	registeredAt, err := time.Parse("2006-01-02", item.Profile.RegisteredAt)
	if err != nil || registeredAt.Format("2006-01-02") != item.Profile.RegisteredAt || item.Profile.OKVED == "" {
		return Scenario{}, errors.New("invalid company profile")
	}
	if item.Behavior.ProfileDelayMS < 0 || item.Behavior.CreditDelayMS < 0 ||
		item.Behavior.ProfileDelayMS > 30000 || item.Behavior.CreditDelayMS > 30000 {
		return Scenario{}, errors.New("response delay must be within 0..30000 ms")
	}

	credit := &bureaupb.CreditHistoryResponse{
		OverdueCount_12M: item.Credit.OverdueCount12M,
		MaxOverdueDays:   item.Credit.MaxOverdueDays,
	}
	if item.Credit.TotalDebt != nil {
		value, err := convertMoney(*item.Credit.TotalDebt)
		if err != nil {
			return Scenario{}, err
		}
		credit.TotalDebt = value
	}
	if item.Credit.OverdueCount12M != nil && *item.Credit.OverdueCount12M < 0 ||
		item.Credit.MaxOverdueDays != nil && *item.Credit.MaxOverdueDays < 0 {
		return Scenario{}, errors.New("negative overdue value")
	}

	for _, entry := range item.Credit.Obligations {
		if entry.Kind == "" {
			return Scenario{}, errors.New("obligation kind is required")
		}
		money, err := convertMoney(entry.Outstanding)
		if err != nil {
			return Scenario{}, err
		}
		credit.Obligations = append(credit.Obligations, &bureaupb.Obligation{Kind: entry.Kind, Outstanding: money})
	}

	return Scenario{
		Profile: &bureaupb.CompanyProfileResponse{
			LegalForm: legalForm, Status: companyStatus, RegisteredAt: item.Profile.RegisteredAt,
			Okved: item.Profile.OKVED, DirectorName: item.Profile.DirectorName,
			FounderNames: item.Profile.Founders, Address: item.Profile.Address,
		},
		Credit: credit, ProfileDelay: time.Duration(item.Behavior.ProfileDelayMS) * time.Millisecond,
		CreditDelay:  time.Duration(item.Behavior.CreditDelayMS) * time.Millisecond,
		ProfileError: item.Behavior.ProfileError, CreditError: item.Behavior.CreditError,
	}, nil
}

func convertMoney(value moneySeed) (*commonpb.Money, error) {
	if value.Currency != "RUB" || value.AmountKopecks < 0 {
		return nil, errors.New("bureau money must be nonnegative RUB kopecks")
	}
	return &commonpb.Money{AmountKopecks: value.AmountKopecks, Currency: value.Currency}, nil
}
