package timeline

import (
	"context"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
)

func (h *Handler) Handle(ctx context.Context, request generated.GetApplicationTimelineRequestObject) (generated.GetApplicationTimelineResponseObject, error) {
	client, ok := middleware.Client(ctx)
	if !ok {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действительная сессия.")}, nil
	}
	entries, err := h.usecase.Execute(ctx, client.OrgID, request.ApplicationId)
	if err != nil {
		return problemResponse{domainerror.Map(ctx, err)}, nil
	}
	items := make([]generated.TimelineEntry, len(entries))
	for i, entry := range entries {
		items[i] = generated.TimelineEntry{
			Id: common.Id(entry.ID), Status: common.ClientStatus(entry.Status),
			Message: entry.Message, OccurredAt: entry.OccurredAt.UTC(),
		}
	}
	return timelineResponse{generated.GetApplicationTimeline200JSONResponse{Items: items}}, nil
}

type timelineResponse struct {
	generated.GetApplicationTimeline200JSONResponse
}

func (r timelineResponse) VisitGetApplicationTimelineResponse(w http.ResponseWriter) error {
	w.Header().Set("Cache-Control", "no-store")
	return r.GetApplicationTimeline200JSONResponse.VisitGetApplicationTimelineResponse(w)
}

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitGetApplicationTimelineResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
