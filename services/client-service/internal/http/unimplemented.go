package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
)

// Business operations stay unimplemented until their use cases are added.
func (s *server) ListApplications(context.Context, generated.ListApplicationsRequestObject) (generated.ListApplicationsResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) CreateApplication(context.Context, generated.CreateApplicationRequestObject) (generated.CreateApplicationResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) GetApplication(context.Context, generated.GetApplicationRequestObject) (generated.GetApplicationResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) UpdateApplication(context.Context, generated.UpdateApplicationRequestObject) (generated.UpdateApplicationResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) CancelApplication(context.Context, generated.CancelApplicationRequestObject) (generated.CancelApplicationResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) AddApplicationDocument(context.Context, generated.AddApplicationDocumentRequestObject) (generated.AddApplicationDocumentResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) ProvideInfo(context.Context, generated.ProvideInfoRequestObject) (generated.ProvideInfoResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) AcceptOffer(context.Context, generated.AcceptOfferRequestObject) (generated.AcceptOfferResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) DeclineOffer(context.Context, generated.DeclineOfferRequestObject) (generated.DeclineOfferResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) SignContract(context.Context, generated.SignContractRequestObject) (generated.SignContractResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) SubmitApplication(context.Context, generated.SubmitApplicationRequestObject) (generated.SubmitApplicationResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) GetApplicationTimeline(context.Context, generated.GetApplicationTimelineRequestObject) (generated.GetApplicationTimelineResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) ListClientProducts(context.Context, generated.ListClientProductsRequestObject) (generated.ListClientProductsResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) GetClientProduct(context.Context, generated.GetClientProductRequestObject) (generated.GetClientProductResponseObject, error) {
	return nil, errNotImplemented
}
