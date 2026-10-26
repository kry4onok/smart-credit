package application

import (
	"slices"
	"testing"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/consumer/bankevents"
	productrepo "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/repository/product"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCatalogAllFinancingPurposesIntegration(t *testing.T) {
	f := newV2Fixture(t)
	product := &commonpb.Product{
		ProductId: f.params.ProductID.String(), Version: 2, Code: "CASHFLOW", Name: "Cash-flow credit",
		PurposeTags: []commonpb.PurposeTag{
			commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL, commonpb.PurposeTag_PURPOSE_TAG_EQUIPMENT,
			commonpb.PurposeTag_PURPOSE_TAG_EXPANSION, commonpb.PurposeTag_PURPOSE_TAG_OTHER,
		},
		MinAmount:     &commonpb.Money{AmountKopecks: 10000000, Currency: "RUB"},
		MaxAmount:     &commonpb.Money{AmountKopecks: 500000000, Currency: "RUB"},
		MinTermMonths: 6, MaxTermMonths: 36, RequirementsPolicyVersion: "sme-profile-v2",
	}
	event := &eventspb.Event{
		EventId: uuid.NewString(), AggregateType: "CATALOG", AggregateId: "catalog", AggregateVersion: 1,
		OccurredAt: timestamppb.Now(), CorrelationId: uuid.NewString(),
		Payload: &eventspb.Event_ProductCatalogPublished{ProductCatalogPublished: &eventspb.ProductCatalogPublished{Products: []*commonpb.Product{product}}},
	}
	projection := bankevents.NewProjection(f.db)
	if applied, err := projection.Execute(t.Context(), event); err != nil || !applied {
		t.Fatalf("publish all financing purposes: applied=%v err=%v", applied, err)
	}
	want := []string{"WORKING_CAPITAL", "EQUIPMENT", "EXPANSION", "OTHER"}
	got, err := productrepo.New(f.db).Get(t.Context(), f.params.ProductID)
	if err != nil || !slices.Equal(got.PurposeTags, want) {
		t.Fatalf("catalog purposes: got=%v err=%v", got.PurposeTags, err)
	}
	if applied, err := projection.Execute(t.Context(), event); err != nil || applied {
		t.Fatalf("catalog replay: applied=%v err=%v", applied, err)
	}
	event.EventId, event.AggregateVersion = uuid.NewString(), 2
	product.PurposeTags = []commonpb.PurposeTag{commonpb.PurposeTag(999)}
	if _, err := projection.Execute(t.Context(), event); err == nil {
		t.Fatal("unknown purpose accepted")
	}
	got, err = productrepo.New(f.db).Get(t.Context(), f.params.ProductID)
	if err != nil || !slices.Equal(got.PurposeTags, want) {
		t.Fatalf("invalid replacement damaged catalog: got=%v err=%v", got.PurposeTags, err)
	}
}
