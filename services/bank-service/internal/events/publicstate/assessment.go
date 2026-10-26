package publicstate

import (
	"context"
	"errors"

	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/proto"
)

// Publish composes a complete public snapshot inside the business transaction.
// Status-only transitions preserve the current assessment; a changed input or
// generation clears it. The relay never reconstructs historical events later.
func Publish(ctx context.Context, q transaction.Queryer, eventID, caseID any, version int64, payload []byte) (pgconn.CommandTag, error) {
	event := new(eventspb.Event)
	if err := proto.Unmarshal(payload, event); err != nil {
		return pgconn.CommandTag{}, err
	}
	if state := event.GetApplicationPublicStatePublished(); state != nil {
		var data []byte
		err := q.QueryRow(ctx, `SELECT a.public_result FROM assessments a JOIN cases c ON c.id=a.case_id WHERE c.id=$1 AND a.input_revision=c.scoring_input_revision AND a.assessment_generation=c.assessment_generation`, caseID).Scan(&data)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return pgconn.CommandTag{}, err
		}
		state.AssessmentJson = data
		payload, err = proto.Marshal(event)
		if err != nil {
			return pgconn.CommandTag{}, err
		}
	}
	return q.Exec(ctx, `INSERT INTO outbox(event_id,aggregate_id,aggregate_version,event_type,payload) VALUES($1,$2,$3,'ApplicationPublicStatePublished',$4)`, eventID, caseID, version, payload)
}
