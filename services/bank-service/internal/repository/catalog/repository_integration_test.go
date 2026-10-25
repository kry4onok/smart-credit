package catalogrepository

import (
	"context"
	"os"
	"testing"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

const createTestProductsQuery = `CREATE TEMP TABLE products (LIKE public.products INCLUDING ALL)`
const createTestOutboxQuery = `CREATE TEMP TABLE outbox (LIKE public.outbox INCLUDING ALL)`
const createTestStateQuery = `CREATE TEMP TABLE catalog_publication (LIKE public.catalog_publication INCLUDING ALL)`
const insertTestProductQuery = `
INSERT INTO products (id, version, code, name, description, purpose_tags,
    required_document_types, min_amount_kopecks, max_amount_kopecks,
    min_term_months, max_term_months, indicative_rate_bps, is_active)
VALUES ($1, 1, $2, $3, 'Описание', ARRAY['WORKING_CAPITAL'],
    ARRAY['BANK_STATEMENT'], 100000000, 500000000, 6, 24, 1650, $4)`
const selectTestEventQuery = `
SELECT aggregate_version, payload
FROM outbox
WHERE aggregate_id = 'catalog'
ORDER BY aggregate_version DESC
LIMIT 1`
const countTestEventsQuery = `SELECT count(*) FROM outbox WHERE aggregate_id = 'catalog'`
const deactivateTestProductQuery = `UPDATE products SET is_active = false WHERE id = $1`

func TestPublishesFullCatalogOnlyWhenChanged(t *testing.T) {
	databaseURL := os.Getenv("BANK_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("BANK_DATABASE_URL is not set")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	for _, query := range []string{createTestProductsQuery, createTestOutboxQuery, createTestStateQuery} {
		if _, err := conn.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	repository := New(conn, transaction.New(conn))
	assertPublished := func(want bool, version int64, count int64) *eventspb.Event {
		t.Helper()
		changed, err := repository.RunOnce(ctx)
		if err != nil || changed != want {
			t.Fatalf("RunOnce changed=%v, want=%v, err=%v", changed, want, err)
		}
		var gotCount int64
		if err := conn.QueryRow(ctx, countTestEventsQuery).Scan(&gotCount); err != nil || gotCount != count {
			t.Fatalf("outbox count=%d, want=%d, err=%v", gotCount, count, err)
		}
		var gotVersion int64
		var payload []byte
		if err := conn.QueryRow(ctx, selectTestEventQuery).Scan(&gotVersion, &payload); err != nil {
			t.Fatal(err)
		}
		var event eventspb.Event
		if err := proto.Unmarshal(payload, &event); err != nil {
			t.Fatal(err)
		}
		if gotVersion != version || event.GetAggregateVersion() != version ||
			event.GetAggregateType() != "CATALOG" || event.GetAggregateId() != "catalog" ||
			event.GetProductCatalogPublished() == nil {
			t.Fatalf("invalid catalog event: %+v", &event)
		}
		return &event
	}

	if products := assertPublished(true, 1, 1).GetProductCatalogPublished().GetProducts(); len(products) != 0 {
		t.Fatalf("initial catalog has %d products", len(products))
	}
	firstID, secondID, inactiveID := uuid.New(), uuid.New(), uuid.New()
	for _, product := range []struct {
		id     uuid.UUID
		code   string
		active bool
	}{{firstID, "WORKING", true}, {secondID, "FLEX", true}, {inactiveID, "HIDDEN", false}} {
		if _, err := conn.Exec(ctx, insertTestProductQuery, product.id, product.code, product.code, product.active); err != nil {
			t.Fatal(err)
		}
	}
	active, err := repository.ListActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 || active[0].GetCode() != "FLEX" || active[1].GetCode() != "WORKING" ||
		active[1].GetProductId() != firstID.String() || active[1].GetVersion() != 1 ||
		active[1].GetMaxAmount().GetAmountKopecks() != 500000000 {
		t.Fatalf("active bank products = %+v", active)
	}
	products := assertPublished(true, 2, 2).GetProductCatalogPublished().GetProducts()
	if len(products) != 2 || products[0].GetCode() != "FLEX" || products[1].GetCode() != "WORKING" ||
		products[1].GetProductId() != firstID.String() || products[1].GetVersion() != 1 ||
		products[1].GetMinAmount().GetCurrency() != "RUB" ||
		products[1].GetPurposeTags()[0] != commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL ||
		products[1].GetRequiredDocumentTypes()[0] != commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT {
		t.Fatalf("unexpected public products: %+v", products)
	}
	assertPublished(false, 2, 2)
	if _, err := conn.Exec(ctx, deactivateTestProductQuery, firstID); err != nil {
		t.Fatal(err)
	}
	products = assertPublished(true, 3, 3).GetProductCatalogPublished().GetProducts()
	if len(products) != 1 || products[0].GetProductId() != secondID.String() {
		t.Fatalf("catalog did not remove inactive product: %+v", products)
	}
	// A fresh repository instance must keep the stored version and snapshot.
	repository = New(conn, transaction.New(conn))
	assertPublished(false, 3, 3)
}
