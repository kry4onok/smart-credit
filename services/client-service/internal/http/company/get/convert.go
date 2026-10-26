package get

import (
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/profile"
)

func convert(c model.Company) generated.VersionedCompany {
	return generated.VersionedCompany{Version: c.Version, Profile: profile.ToHTTP(c)}
}
