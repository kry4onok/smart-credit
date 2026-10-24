package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
)

// Business operations stay unimplemented until their use cases are added.
func (s *server) CancelApplication(context.Context, generated.CancelApplicationRequestObject) (generated.CancelApplicationResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) SignContract(context.Context, generated.SignContractRequestObject) (generated.SignContractResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) GetApplicationTimeline(context.Context, generated.GetApplicationTimelineRequestObject) (generated.GetApplicationTimelineResponseObject, error) {
	return nil, errNotImplemented
}
