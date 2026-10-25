package application

import (
	"crypto/sha256"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAcceptedOfferRemainsAvailableForSigningIntegration(t *testing.T) {
	for _, kind := range []commonpb.OfferKind{commonpb.OfferKind_OFFER_KIND_OFFER, commonpb.OfferKind_OFFER_KIND_COUNTER_OFFER} {
		t.Run(kind.String(), func(t *testing.T) {
			f := newApplicationFixture(t)
			f.submit(t)
			offer := &commonpb.Offer{
				OfferId: uuid.NewString(), Kind: kind,
				ProductId: f.params.ProductID.String(), ProductVersion: 1, ProductName: "Демо-продукт",
				Amount: &commonpb.Money{AmountKopecks: 10000, Currency: "RUB"}, TermMonths: 12,
				RateBps: 1900, Explanation: "Условия кредита", ExpiresAt: timestamppb.New(time.Now().Add(72 * time.Hour)),
			}
			f.project(t, 1, &eventspb.ApplicationPublicStatePublished{
				Status: eventspb.ClientStatus_CLIENT_STATUS_OFFER_AVAILABLE, StageLabel: "Предложение", ActiveOffer: offer,
			})
			previousOfferID := uuid.MustParse(offer.GetOfferId())
			offer = proto.Clone(offer).(*commonpb.Offer)
			offer.OfferId = uuid.NewString()
			offer.Amount.AmountKopecks = 20000
			f.project(t, 2, &eventspb.ApplicationPublicStatePublished{
				Status: eventspb.ClientStatus_CLIENT_STATUS_OFFER_AVAILABLE, StageLabel: "Новое предложение", ActiveOffer: offer,
			})
			before, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
			if err != nil || before.Offer == nil {
				t.Fatalf("offer before acceptance = %+v, err=%v", before.Offer, err)
			}
			hash := sha256.Sum256([]byte("accept latest offer"))
			if _, err := f.repo.AcceptOffer(t.Context(), model.AcceptOfferParams{
				OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID,
				OfferID: before.Offer.ID, IdempotencyID: uuid.New(), RequestHash: hash[:],
			}); err != nil {
				t.Fatal(err)
			}
			// The bank confirms acceptance without an active offer. A later snapshot
			// must preserve access to the accepted terms as well.
			for _, version := range []int64{3, 4} {
				f.project(t, version, &eventspb.ApplicationPublicStatePublished{
					Status: eventspb.ClientStatus_CLIENT_STATUS_FORMALIZING, StageLabel: "Оформление кредита",
				})
			}
			var activeOffers int
			if err := f.db.QueryRow(t.Context(), "SELECT count(*) FROM offers WHERE application_id = $1 AND is_active", f.draft.ID).Scan(&activeOffers); err != nil || activeOffers != 0 {
				t.Fatalf("active offers=%d, err=%v", activeOffers, err)
			}
			// A fresh repository has no browser cache or in-memory offer ID.
			reloaded := New(Adapt(f.db))
			got, err := reloaded.Get(t.Context(), f.params.OrgID, f.draft.ID)
			if err != nil || !reflect.DeepEqual(got.Offer, before.Offer) || !slices.Contains(got.AllowedActions, "SIGN") {
				t.Fatalf("reloaded application=%+v, offer=%+v, err=%v", got, got.Offer, err)
			}
			params := model.SignContractParams{
				OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID,
				OfferID: previousOfferID, IdempotencyID: uuid.New(), RequestHash: hash[:],
			}
			if _, err := reloaded.SignContract(t.Context(), params); !errors.Is(err, domain.ErrApplicationSignConflict) {
				t.Fatalf("sign superseded offer: err=%v", err)
			}
			params.OfferID = got.Offer.ID
			if _, err := reloaded.SignContract(t.Context(), params); err != nil {
				t.Fatal(err)
			}
			if signed := f.event(t, "ContractSigned").GetContractSigned(); signed.GetOfferId() != got.Offer.ID.String() {
				t.Fatalf("signed event=%v", signed)
			}
			pending, err := reloaded.Get(t.Context(), f.params.OrgID, f.draft.ID)
			if err != nil || !reflect.DeepEqual(pending.Offer, before.Offer) || len(pending.AllowedActions) != 0 {
				t.Fatalf("sign pending application=%+v, err=%v", pending, err)
			}
			// Inactive historical offers must remain hidden outside FORMALIZING.
			for i, status := range []eventspb.ClientStatus{
				eventspb.ClientStatus_CLIENT_STATUS_DISBURSED,
				eventspb.ClientStatus_CLIENT_STATUS_CLOSED,
				eventspb.ClientStatus_CLIENT_STATUS_REJECTED,
				eventspb.ClientStatus_CLIENT_STATUS_UNDER_REVIEW,
			} {
				f.project(t, int64(5+i), &eventspb.ApplicationPublicStatePublished{Status: status, StageLabel: "Обновление"})
				got, err := reloaded.Get(t.Context(), f.params.OrgID, f.draft.ID)
				if err != nil || got.Offer != nil {
					t.Fatalf("offer in %s = %+v, err=%v", status, got.Offer, err)
				}
			}
		})
	}
}
