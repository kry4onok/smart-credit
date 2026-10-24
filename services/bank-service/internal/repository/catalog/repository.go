package catalogrepository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const maxPublicMoney = 9007199254740991

type Repository struct {
	db           transaction.Queryer
	transactions *transaction.Manager
}

func New(db transaction.Queryer, transactions *transaction.Manager) *Repository {
	return &Repository{db: db, transactions: transactions}
}

func (r *Repository) RunOnce(ctx context.Context) (bool, error) {
	var changed bool
	err := r.transactions.WithinTransaction(ctx, func(ctx context.Context) error {
		q := transaction.QueryerFromContext(ctx, r.db)
		if _, err := q.Exec(ctx, ensureStateQuery); err != nil {
			return fmt.Errorf("ensure catalog publication state: %w", err)
		}
		var version int64
		var previousHash []byte
		if err := q.QueryRow(ctx, lockStateQuery).Scan(&version, &previousHash); err != nil {
			return fmt.Errorf("lock catalog publication state: %w", err)
		}
		products, err := loadProducts(ctx, q)
		if err != nil {
			return err
		}
		catalog := &eventspb.ProductCatalogPublished{Products: products}
		content, err := proto.MarshalOptions{Deterministic: true}.Marshal(catalog)
		if err != nil {
			return fmt.Errorf("encode product catalog: %w", err)
		}
		hash := sha256.Sum256(content)
		if bytes.Equal(previousHash, hash[:]) {
			return nil
		}
		if version == math.MaxInt64 {
			return errors.New("catalog version exhausted")
		}
		version++
		eventID := uuid.New()
		event := &eventspb.Event{
			EventId: eventID.String(), AggregateType: "CATALOG", AggregateId: "catalog",
			AggregateVersion: version, OccurredAt: timestamppb.New(time.Now().UTC()),
			CorrelationId: eventID.String(),
			Payload:       &eventspb.Event_ProductCatalogPublished{ProductCatalogPublished: catalog},
		}
		payload, err := proto.Marshal(event)
		if err != nil {
			return fmt.Errorf("encode product catalog event: %w", err)
		}
		if _, err := q.Exec(ctx, insertOutboxQuery, eventID, version, payload); err != nil {
			return fmt.Errorf("write product catalog outbox: %w", err)
		}
		if _, err := q.Exec(ctx, updateStateQuery, version, hash[:]); err != nil {
			return fmt.Errorf("save catalog publication state: %w", err)
		}
		changed = true
		return nil
	})
	return changed, err
}

func loadProducts(ctx context.Context, q transaction.Queryer) ([]*commonpb.Product, error) {
	rows, err := q.Query(ctx, listProductsQuery)
	if err != nil {
		return nil, fmt.Errorf("list active products: %w", err)
	}
	defer rows.Close()
	products := make([]*commonpb.Product, 0)
	for rows.Next() {
		var p commonpb.Product
		var id uuid.UUID
		var purposeTags, documentTypes []string
		var minAmount, maxAmount int64
		var indicativeRate int32
		if err := rows.Scan(&id, &p.Version, &p.Code, &p.Name, &p.Description,
			&purposeTags, &documentTypes, &minAmount, &maxAmount,
			&p.MinTermMonths, &p.MaxTermMonths, &indicativeRate); err != nil {
			return nil, fmt.Errorf("scan active product: %w", err)
		}
		p.ProductId = id.String()
		p.IndicativeRateBps = indicativeRate
		p.MinAmount = &commonpb.Money{AmountKopecks: minAmount, Currency: "RUB"}
		p.MaxAmount = &commonpb.Money{AmountKopecks: maxAmount, Currency: "RUB"}
		if p.Version <= 0 || minAmount <= 0 || maxAmount < minAmount || maxAmount > maxPublicMoney ||
			p.MinTermMonths <= 0 || p.MaxTermMonths < p.MinTermMonths {
			return nil, fmt.Errorf("active product %s has invalid public terms", id)
		}
		for _, tag := range purposeTags {
			value, ok := commonpb.PurposeTag_value["PURPOSE_TAG_"+tag]
			if !ok || value == 0 {
				return nil, fmt.Errorf("active product %s has invalid purpose %q", id, tag)
			}
			p.PurposeTags = append(p.PurposeTags, commonpb.PurposeTag(value))
		}
		if len(p.PurposeTags) == 0 {
			return nil, fmt.Errorf("active product %s has no purpose tags", id)
		}
		for _, documentType := range documentTypes {
			value, ok := commonpb.DocumentType_value["DOCUMENT_TYPE_"+documentType]
			if !ok || value == 0 {
				return nil, fmt.Errorf("active product %s has invalid document type %q", id, documentType)
			}
			p.RequiredDocumentTypes = append(p.RequiredDocumentTypes, commonpb.DocumentType(value))
		}
		products = append(products, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active products: %w", err)
	}
	return products, nil
}
