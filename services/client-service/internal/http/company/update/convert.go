package update

import (
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/profile"
)

func fromRequest(v common.CompanyProfile) model.Company { return profile.FromHTTP(v) }

func toResponse(c model.Company) generated.VersionedCompany {
	return generated.VersionedCompany{Version: c.Version, Profile: profile.ToHTTP(c)}
}
