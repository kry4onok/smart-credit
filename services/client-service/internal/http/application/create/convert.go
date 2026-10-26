package create

import (
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	get "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/get"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

func toResponse(v model.Application) generated.ClientApplication { return get.ToResponse(v) }
