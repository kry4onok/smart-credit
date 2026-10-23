package scoringworker

import (
	"context"
	"errors"
	"testing"
	"time"

	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
	scoringrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/scoring"
	"github.com/google/uuid"
)

type queueStub struct {
	claim      scoringrepository.Claim
	current    scoringrepository.CurrentAttempt
	finished   bool
	finishCode string
	released   bool
}

func (q *queueStub) ClaimNext(context.Context) (scoringrepository.Claim, bool, error) {
	return q.claim, true, nil
}
func (q *queueStub) CurrentAttempt(context.Context, scoringrepository.Claim) (scoringrepository.CurrentAttempt, error) {
	return q.current, nil
}
func (q *queueStub) Finish(_ context.Context, _ scoringrepository.Claim, code string) (bool, error) {
	q.finished, q.finishCode = true, code
	return true, nil
}
func (q *queueStub) Release(context.Context, scoringrepository.Claim) error {
	q.released = true
	return nil
}

type runnerStub struct {
	queue *queueStub
	err   error
	calls int
}

func (r *runnerStub) Run(_ context.Context, _ uuid.UUID) (uuid.UUID, bool, error) {
	r.calls++
	if r.err == nil {
		id := uuid.New()
		r.queue.current = scoringrepository.CurrentAttempt{ID: &id, Status: "ASSESSED"}
	}
	return uuid.Nil, false, r.err
}

type completerStub struct {
	queue *queueStub
	calls int
}

func (c *completerStub) Complete(_ context.Context, _ uuid.UUID, _ *scoringpb.ScoreResponse, code string) (bool, error) {
	c.calls++
	if code != "SCORING_INTERRUPTED" {
		return false, errors.New("unexpected recovery code")
	}
	c.queue.current.Status = "TECHNICAL_ERROR"
	return true, nil
}

func TestWorkerProcessesNewAndInterruptedCases(t *testing.T) {
	for _, tc := range []struct {
		name          string
		initial       string
		wantRunCalls  int
		wantCompletes int
	}{
		{"new case", "", 1, 0},
		{"interrupted attempt", "PENDING", 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := &queueStub{claim: scoringrepository.Claim{
				CaseID: uuid.New(), LeaseUntil: time.Now().Add(time.Minute),
			}}
			if tc.initial != "" {
				id := uuid.New()
				q.current = scoringrepository.CurrentAttempt{ID: &id, Status: tc.initial, Recoverable: true}
			}
			runner := &runnerStub{queue: q}
			completer := &completerStub{queue: q}
			worked, err := New(q, runner, completer).RunOnce(t.Context())
			if err != nil || !worked || !q.finished || q.released ||
				runner.calls != tc.wantRunCalls || completer.calls != tc.wantCompletes {
				t.Fatalf("worked=%t err=%v finished=%t released=%t run=%d complete=%d",
					worked, err, q.finished, q.released, runner.calls, completer.calls)
			}
		})
	}
}

func TestWorkerRoutesMissingInputToReview(t *testing.T) {
	q := &queueStub{claim: scoringrepository.Claim{CaseID: uuid.New()}}
	runner := &runnerStub{queue: q, err: scoringrepository.ErrInputNotReady}
	worked, err := New(q, runner, &completerStub{queue: q}).RunOnce(t.Context())
	if err != nil || !worked || !q.finished || q.finishCode != "SCORING_INPUT_NOT_READY" || q.released {
		t.Fatalf("worked=%t err=%v finished=%t code=%q released=%t",
			worked, err, q.finished, q.finishCode, q.released)
	}
}

func TestWorkerDoesNotInterruptFreshPendingAttempt(t *testing.T) {
	id := uuid.New()
	q := &queueStub{
		claim:   scoringrepository.Claim{CaseID: uuid.New()},
		current: scoringrepository.CurrentAttempt{ID: &id, Status: "PENDING"},
	}
	completer := &completerStub{queue: q}
	_, err := New(q, &runnerStub{queue: q}, completer).RunOnce(t.Context())
	if !errors.Is(err, scoringrepository.ErrPendingAttempt) || completer.calls != 0 || !q.released {
		t.Fatalf("error=%v complete=%d released=%t", err, completer.calls, q.released)
	}
}

func TestWorkerReleasesLeaseOnTransientFailure(t *testing.T) {
	q := &queueStub{claim: scoringrepository.Claim{CaseID: uuid.New()}}
	runner := &runnerStub{queue: q, err: errors.New("database unavailable")}
	_, err := New(q, runner, &completerStub{queue: q}).RunOnce(t.Context())
	if err == nil || !q.released || q.finished {
		t.Fatalf("error=%v released=%t finished=%t", err, q.released, q.finished)
	}
}
