// Command scoring-smoke checks the real Go-to-Python gRPC contract using shared fixtures.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

type dataIssue struct {
	Code      string `json:"code"`
	FieldPath string `json:"field_path"`
}

type factorPoints struct {
	FactorName string `json:"factor_name"`
	Points     int32  `json:"points"`
}

type fixtureCase struct {
	Name     string          `json:"name"`
	Request  json.RawMessage `json:"request"`
	Expected struct {
		AssessmentStatus string         `json:"assessment_status"`
		Score            *int32         `json:"score"`
		Grade            string         `json:"grade"`
		DataIssues       []dataIssue    `json:"data_issues"`
		Factors          []factorPoints `json:"factor_contributions"`
	} `json:"expected"`
}

type fixtureFile struct {
	SchemaVersion string        `json:"schema_version"`
	Cases         []fixtureCase `json:"cases"`
}

type expectedReason struct {
	contribution int32
	code         string
	direction    scoringpb.Direction
}

func main() {
	target := flag.String("target", "127.0.0.1:50051", "scoring gRPC endpoint")
	fixturesPath := flag.String("fixtures", "seed/scoring/personas.json", "shared fixtures path")
	configPath := flag.String("config", "ml/scoring/models/scorecard-v1.json", "exact server model artifact")
	caseName := flag.String("case", "", "run one fixture by name")
	flag.Parse()
	if err := run(*target, *fixturesPath, *configPath, *caseName); err != nil {
		// No financial inputs or arbitrary server error details are printed.
		fmt.Fprintf(os.Stderr, "Scoring smoke failed: %v\n", err)
		os.Exit(1)
	}
}

func run(target, fixturesPath, configPath, caseName string) error {
	data, err := os.ReadFile(fixturesPath)
	if err != nil {
		return fmt.Errorf("read fixtures: %w", err)
	}
	var fixtures fixtureFile
	if err := json.Unmarshal(data, &fixtures); err != nil {
		return fmt.Errorf("decode fixtures: %w", err)
	}
	if fixtures.SchemaVersion != "scoring-personas-v1" {
		return errors.New("unknown fixture schema")
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read model artifact: %w", err)
	}
	digest := sha256.Sum256(config)
	checksum := hex.EncodeToString(digest[:])
	connection, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return errors.New("create scoring connection")
	}
	defer connection.Close()
	for _, service := range []string{"", "smartcredit.scoring.ScoringService"} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		response, rpcErr := healthpb.NewHealthClient(connection).Check(ctx, &healthpb.HealthCheckRequest{Service: service})
		cancel()
		if rpcErr != nil {
			return fmt.Errorf("health RPC: %s", status.Code(rpcErr))
		}
		if response == nil || response.Status != healthpb.HealthCheckResponse_SERVING {
			return errors.New("scoring health is not SERVING")
		}
	}
	client := scoringpb.NewScoringServiceClient(connection)
	seen := make(map[string]bool)
	passed := 0
	for _, fixture := range fixtures.Cases {
		if seen[fixture.Name] {
			return errors.New("duplicate fixture name")
		}
		seen[fixture.Name] = true
		if caseName != "" && fixture.Name != caseName {
			continue
		}
		request := &scoringpb.ScoreRequest{}
		if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(fixture.Request, request); err != nil {
			return fmt.Errorf("%s: invalid request fixture", fixture.Name)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		response, rpcErr := client.Score(ctx, request)
		cancel()
		if rpcErr != nil {
			return fmt.Errorf("%s: scoring RPC: %s", fixture.Name, status.Code(rpcErr))
		}
		if err := verifyResponse(fixture, request, response, checksum); err != nil {
			return fmt.Errorf("%s: %w", fixture.Name, err)
		}
		passed++
		fmt.Printf("PASS %s: %s", fixture.Name, fixture.Expected.AssessmentStatus)
		if response.Score != nil {
			fmt.Printf(" score=%d grade=%s", response.GetScore(), fixture.Expected.Grade)
		}
		fmt.Println()
	}
	if passed == 0 {
		return errors.New("no matching fixtures")
	}
	fmt.Printf("Go → Python scoring: %d fixtures passed\n", passed)
	return nil
}

func verifyResponse(fixture fixtureCase, request *scoringpb.ScoreRequest, response *scoringpb.ScoreResponse, checksum string) error {
	if response == nil {
		return errors.New("missing response")
	}
	expected := fixture.Expected
	statusValue, exists := scoringpb.AssessmentStatus_value["ASSESSMENT_STATUS_"+expected.AssessmentStatus]
	if !exists || statusValue == 0 || response.AssessmentStatus != scoringpb.AssessmentStatus(statusValue) {
		return errors.New("assessment status mismatch")
	}
	if response.ModelVersion != "monotone-scorecard-v1" || response.FeatureSchemaVersion != "borrower-features-v1" || response.BaselineScore != 500 {
		return errors.New("model metadata mismatch")
	}
	if response.ConfigChecksum != checksum {
		return errors.New("model artifact checksum mismatch")
	}
	if len(response.DataIssues) != len(expected.DataIssues) {
		return errors.New("data issues mismatch")
	}
	previousIssue := ""
	for index, issue := range response.DataIssues {
		if issue == nil || issue.Code != expected.DataIssues[index].Code || issue.FieldPath != expected.DataIssues[index].FieldPath {
			return errors.New("data issue mismatch")
		}
		key := issue.FieldPath + "\x00" + issue.Code
		if key < previousIssue {
			return errors.New("data issue ordering mismatch")
		}
		previousIssue = key
	}
	if response.AssessmentStatus != scoringpb.AssessmentStatus_ASSESSMENT_STATUS_ASSESSED {
		if response.Score != nil || response.Grade != scoringpb.Grade_GRADE_UNSPECIFIED || len(response.Reasons) != 0 || len(response.FactorContributions) != 0 {
			return errors.New("non-assessed response contains a score, grade, or explanations")
		}
		return nil
	}
	gradeValue, exists := scoringpb.Grade_value["GRADE_"+expected.Grade]
	if !exists || expected.Score == nil || response.Score == nil || response.GetScore() != *expected.Score || response.Grade != scoringpb.Grade(gradeValue) {
		return errors.New("score presence, score, or grade mismatch")
	}
	if len(response.FactorContributions) != 4 || len(expected.Factors) != 4 {
		return errors.New("expected exactly four factor groups")
	}
	points := make(map[string]int32)
	for _, factor := range expected.Factors {
		points[factor.FactorName] = factor.Points
	}
	seen := make(map[string]bool)
	sum := int32(500)
	for _, factor := range response.FactorContributions {
		if factor == nil {
			return errors.New("missing factor")
		}
		expectedPoints, exists := points[factor.FactorName]
		if !exists || seen[factor.FactorName] || factor.Points != expectedPoints || strings.TrimSpace(factor.Description) == "" {
			return errors.New("factor name, points, or description mismatch")
		}
		seen[factor.FactorName] = true
		sum += factor.Points
	}
	if sum != response.GetScore() {
		return errors.New("contributions do not sum to score")
	}
	reasons := reasonsForFixture(points, request.GetFeatures())
	if len(response.Reasons) != len(reasons) {
		return errors.New("reason count mismatch")
	}
	for index, reason := range response.Reasons {
		if reason == nil || strings.TrimSpace(reason.Description) == "" || reason.Contribution != reasons[index].contribution || reason.Direction != reasons[index].direction || reason.Code.String() != "REASON_CODE_"+reasons[index].code {
			return errors.New("reason code, magnitude, direction, order, or description mismatch")
		}
	}
	return nil
}

func reasonsForFixture(points map[string]int32, features *scoringpb.Features) []expectedReason {
	positiveHistory := "GOOD_PAYMENT_DISCIPLINE"
	if features != nil && features.OverdueCount_12M != nil && features.MaxOverdueDays != nil && features.GetOverdueCount_12M() == 0 && features.GetMaxOverdueDays() == 0 {
		positiveHistory = "NO_OVERDUE"
	}
	codes := map[string][2]string{
		"business_age":       {"SHORT_HISTORY", "STABLE_BUSINESS"},
		"profit_margin":      {"LOW_PROFIT_MARGIN", "HEALTHY_PROFIT_MARGIN"},
		"bureau_debt_load":   {"HIGH_BUREAU_DEBT", "MANAGEABLE_BUREAU_DEBT"},
		"payment_discipline": {"RECENT_OVERDUE", positiveHistory},
	}
	reasons := make([]expectedReason, 0, 4)
	for name, value := range points {
		if value == 0 {
			continue
		}
		reason := expectedReason{contribution: -value, code: codes[name][0], direction: scoringpb.Direction_DIRECTION_INCREASES_RISK}
		if value > 0 {
			reason = expectedReason{contribution: value, code: codes[name][1], direction: scoringpb.Direction_DIRECTION_DECREASES_RISK}
		}
		reasons = append(reasons, reason)
	}
	sort.Slice(reasons, func(left, right int) bool {
		if reasons[left].contribution != reasons[right].contribution {
			return reasons[left].contribution > reasons[right].contribution
		}
		return reasons[left].code < reasons[right].code
	})
	if len(reasons) > 3 {
		reasons = reasons[:3]
	}
	return reasons
}
