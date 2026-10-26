// Package assistanthelp validates selection plans and renders only backend-owned text.
package assistanthelp

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	pb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/assistant"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

//go:embed catalog.json
var catalogJSON []byte

const CatalogVersion = "smart-credit-help-v1"

var ErrIncompatible = errors.New("UNSUPPORTED_CATALOG_VERSION")
var ErrExpired = errors.New("CONTEXT_EXPIRED")
var phrases = func() map[string]string {
	var c struct {
		Phrases map[string]string `json:"phrases"`
	}
	if err := json.Unmarshal(catalogJSON, &c); err != nil {
		panic(err)
	}
	return c.Phrases
}()

func mode(r *pb.HelpRequest) string { return strings.TrimPrefix(r.Mode.String(), "HELP_MODE_") }
func topic(s *pb.Statement) string  { return strings.TrimPrefix(s.Topic.String(), "STATEMENT_TOPIC_") }
func relevant(s *pb.Statement, r *pb.HelpRequest) bool {
	m := mode(r)
	allowed := map[string]string{"NEXT_STEP": "NEXT_STEP PROFILE REQUIREMENT DATA_QUALITY", "REQUIREMENT": "REQUIREMENT POLICY DATA_QUALITY", "ASSESSMENT": "SCORE AFFORDABILITY POLICY DATA_QUALITY", "COMPARE_OPTIONS": "OPTION AFFORDABILITY DATA_QUALITY", "DECISION": "DECISION DATA_QUALITY", "CHANGES": "CHANGE DATA_QUALITY"}
	if m != "CASE_SUMMARY" && m != "QUESTION" && !strings.Contains(" "+allowed[m]+" ", " "+topic(s)+" ") {
		return false
	}
	if len(r.TargetIds) > 0 && s.TargetId != "" && (m == "REQUIREMENT" || m == "COMPARE_OPTIONS" || m == "CHANGES" || m == "ASSESSMENT") {
		for _, id := range r.TargetIds {
			if id == s.TargetId {
				return true
			}
		}
		return false
	}
	return true
}
func primaryTopic(s *pb.Statement, r *pb.HelpRequest) bool {
	m := mode(r)
	switch m {
	case "REQUIREMENT":
		return topic(s) == "REQUIREMENT"
	case "COMPARE_OPTIONS":
		return topic(s) == "OPTION"
	case "CHANGES":
		return topic(s) == "CHANGE"
	case "DECISION":
		return topic(s) == "DECISION"
	}
	return topic(s) != "DATA_QUALITY"
}
func primary(r *pb.HelpRequest) bool {
	covered := map[string]bool{}
	for _, s := range r.Context.Statements {
		if relevant(s, r) && primaryTopic(s, r) {
			covered[s.TargetId] = true
		}
	}
	if mode(r) == "COMPARE_OPTIONS" {
		for _, id := range r.TargetIds {
			if !covered[id] {
				return false
			}
		}
	}
	return len(covered) > 0
}
func blockAllowed(k pb.BlockKind, r *pb.HelpRequest) bool {
	m := mode(r)
	if m == "QUESTION" || m == "CASE_SUMMARY" {
		return k >= 1 && k <= 6
	}
	if k == pb.BlockKind_BLOCK_KIND_SUMMARY || k == pb.BlockKind_BLOCK_KIND_LIMITATIONS || k == pb.BlockKind_BLOCK_KIND_NEXT_STEPS {
		return true
	}
	if m == "CHANGES" {
		return k == pb.BlockKind_BLOCK_KIND_CHANGES
	}
	if m == "COMPARE_OPTIONS" {
		return k == pb.BlockKind_BLOCK_KIND_COMPARISON
	}
	return k == pb.BlockKind_BLOCK_KIND_DETAILS
}
func mainBlock(r *pb.HelpRequest) pb.BlockKind {
	switch mode(r) {
	case "COMPARE_OPTIONS":
		return pb.BlockKind_BLOCK_KIND_COMPARISON
	case "CHANGES":
		return pb.BlockKind_BLOCK_KIND_CHANGES
	case "CASE_SUMMARY":
		return pb.BlockKind_BLOCK_KIND_SUMMARY
	}
	return pb.BlockKind_BLOCK_KIND_DETAILS
}
func Validate(r *pb.HelpRequest, now time.Time) error {
	if r == nil || r.Context == nil {
		return errors.New("INVALID_CONTEXT")
	}
	c := r.Context
	if c.CatalogVersion != CatalogVersion {
		return ErrIncompatible
	}
	if _, e := uuid.Parse(r.RequestId); e != nil {
		return errors.New("INVALID_REQUEST_ID")
	}
	if _, e := uuid.Parse(c.ApplicationId); e != nil {
		return errors.New("INVALID_APPLICATION_ID")
	}
	if c.SchemaVersion != 1 || c.Audience != pb.Audience_AUDIENCE_CLIENT && c.Audience != pb.Audience_AUDIENCE_ANALYST || c.SourceRevision == "" || c.PolicyVersion == "" || len(c.ContextId) != 64 || strings.Trim(c.ContextId, "0123456789abcdef") != "" {
		return errors.New("INVALID_CONTEXT")
	}
	if c.GeneratedAt == nil || c.ValidUntil == nil || c.GeneratedAt.CheckValid() != nil || c.ValidUntil.CheckValid() != nil || c.GeneratedAt.AsTime().After(now) || !c.ValidUntil.AsTime().After(now) || c.ValidUntil.AsTime().Sub(c.GeneratedAt.AsTime()) > 60*time.Second || !c.ValidUntil.AsTime().After(c.GeneratedAt.AsTime()) || c.Freshness == pb.Freshness_FRESHNESS_STALE {
		return ErrExpired
	}
	if c.Freshness != pb.Freshness_FRESHNESS_CURRENT && c.Freshness != pb.Freshness_FRESHNESS_PENDING {
		return errors.New("INVALID_FRESHNESS")
	}
	if r.Mode < 1 || r.Mode > 8 || r.Detail < 1 || r.Detail > 2 || r.Locale != "ru-RU" || len(r.TargetIds) > 3 {
		return errors.New("INVALID_MODE")
	}
	if len(c.Statements) > 80 || len(c.Evidence) > 120 || len(c.Targets) > 120 {
		return errors.New("INPUT_TOO_LARGE")
	}
	raw, e := protojson.Marshal(r)
	if e != nil || len(raw) > 48*1024 {
		return errors.New("INPUT_TOO_LARGE")
	}
	targets := map[string]*pb.Target{}
	evidence := map[string]bool{}
	statements := map[string]*pb.Statement{}
	actions := map[string]bool{}
	for _, t := range c.Targets {
		if t == nil || t.Id == "" || targets[t.Id] != nil || t.Kind < 1 || t.Kind > 6 || t.Label == "" {
			return errors.New("INVALID_TARGET")
		}
		targets[t.Id] = t
	}
	for _, v := range c.Evidence {
		if v == nil || v.Id == "" || evidence[v.Id] || v.Kind < 1 || v.Kind > 10 || v.State < 1 || v.State > 7 || v.SourceRef == "" || v.Label == "" || v.ObservedAt == nil || v.ObservedAt.CheckValid() != nil {
			return errors.New("INVALID_EVIDENCE")
		}
		for _, d := range []*string{v.PeriodFrom, v.PeriodTo} {
			if d != nil {
				if _, e := time.Parse(time.DateOnly, *d); e != nil {
					return errors.New("INVALID_PERIOD")
				}
			}
		}
		if v.PeriodFrom != nil && v.PeriodTo != nil && *v.PeriodFrom > *v.PeriodTo {
			return errors.New("INVALID_PERIOD")
		}
		evidence[v.Id] = true
	}
	for _, s := range c.Statements {
		if s == nil || s.Id == "" || statements[s.Id] != nil || s.Topic < 1 || s.Topic > 10 || s.Priority > 100 || s.TargetId != "" && targets[s.TargetId] == nil || s.TargetId == "" && !s.Required || len(s.Variants) != 2 {
			return errors.New("INVALID_STATEMENT")
		}
		variants := map[string]bool{}
		for _, v := range s.Variants {
			if v == nil || v.Id != "concise" && v.Id != "detailed" || variants[v.Id] || strings.TrimSpace(v.Text) == "" {
				return errors.New("INVALID_VARIANT")
			}
			variants[v.Id] = true
		}
		seen := map[string]bool{}
		for _, id := range s.EvidenceIds {
			if !evidence[id] || seen[id] {
				return errors.New("INVALID_EVIDENCE_REF")
			}
			seen[id] = true
		}
		if s.TargetId != "" && len(s.EvidenceIds) == 0 {
			return errors.New("MISSING_EVIDENCE")
		}
		statements[s.Id] = s
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] || statements[id] == nil {
			return errors.New("INVALID_COMPANION")
		}
		if done[id] {
			return nil
		}
		visiting[id] = true
		seen := map[string]bool{}
		for _, child := range statements[id].CompanionIds {
			if seen[child] {
				return errors.New("DUPLICATE_COMPANION")
			}
			seen[child] = true
			if e := visit(child); e != nil {
				return e
			}
		}
		visiting[id] = false
		done[id] = true
		return nil
	}
	for id := range statements {
		if e := visit(id); e != nil {
			return e
		}
	}
	for _, a := range c.Actions {
		if a == nil || a.Id == "" || actions[a.Id] || targets[a.TargetId] == nil || a.Kind < 1 || a.Kind > 6 || a.Label == "" {
			return errors.New("INVALID_ACTION")
		}
		actions[a.Id] = true
	}
	selected := map[string]bool{}
	for _, id := range r.TargetIds {
		if targets[id] == nil || selected[id] {
			return errors.New("INVALID_SELECTED_TARGET")
		}
		selected[id] = true
	}
	m := mode(r)
	if m == "QUESTION" {
		if n := utf8.RuneCountInString(r.Question); n < 1 || n > 1000 {
			return errors.New("INVALID_QUESTION")
		}
	} else if r.Question != "" {
		return errors.New("UNEXPECTED_QUESTION")
	}
	if m == "CASE_SUMMARY" && c.Audience != pb.Audience_AUDIENCE_ANALYST {
		return errors.New("FORBIDDEN_MODE")
	}
	if m == "REQUIREMENT" && (len(r.TargetIds) != 1 || targets[r.TargetIds[0]].Kind != pb.TargetKind_TARGET_KIND_REQUIREMENT) {
		return errors.New("INVALID_TARGET_KIND")
	}
	if m == "CHANGES" && (len(r.TargetIds) != 1 || targets[r.TargetIds[0]].Kind != pb.TargetKind_TARGET_KIND_CHANGE) {
		return errors.New("INVALID_TARGET_KIND")
	}
	if m == "COMPARE_OPTIONS" {
		if c.CalculationId == nil || len(r.TargetIds) < 2 {
			return errors.New("CALCULATION_REQUIRED")
		}
		for _, id := range r.TargetIds {
			if targets[id].Kind != pb.TargetKind_TARGET_KIND_OPTION {
				return errors.New("INVALID_TARGET_KIND")
			}
		}
	}
	if m == "ASSESSMENT" {
		if len(r.TargetIds) > 1 {
			return errors.New("INVALID_TARGET_KIND")
		}
		for _, id := range r.TargetIds {
			if targets[id].Kind != pb.TargetKind_TARGET_KIND_FACTOR {
				return errors.New("INVALID_TARGET_KIND")
			}
		}
	}
	return nil
}

func FallbackPlan(r *pb.HelpRequest) *pb.AnswerPlan {
	p := &pb.AnswerPlan{Status: pb.AnswerStatus_ANSWER_STATUS_INSUFFICIENT_DATA}
	if mode(r) == "QUESTION" {
		for _, t := range r.Context.Targets[:min(3, len(r.Context.Targets))] {
			p.ClarifyTargetIds = append(p.ClarifyTargetIds, t.Id)
		}
		if len(p.ClarifyTargetIds) > 0 {
			p.Status = pb.AnswerStatus_ANSWER_STATUS_CLARIFY
		}
		for _, s := range r.Context.Statements {
			if topic(s) == "NEXT_STEP" {
				p.Blocks = []*pb.PlanBlock{{Kind: pb.BlockKind_BLOCK_KIND_NEXT_STEPS, Selections: []*pb.Selection{{StatementId: s.Id, VariantId: "concise"}}}}
				break
			}
		}
		return p
	}
	if !primary(r) {
		return p
	}
	items := append([]*pb.Statement(nil), r.Context.Statements...)
	chosen := map[string]bool{}
	for _, id := range r.TargetIds {
		chosen[id] = true
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Required != b.Required {
			return a.Required
		}
		if chosen[a.TargetId] != chosen[b.TargetId] {
			return chosen[a.TargetId]
		}
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		return a.Id < b.Id
	})
	kind := mainBlock(r)
	main, limits := &pb.PlanBlock{Kind: kind}, &pb.PlanBlock{Kind: pb.BlockKind_BLOCK_KIND_LIMITATIONS}
	count := 0
	for _, s := range items {
		if relevant(s, r) && (s.Required || len(chosen) == 0 || chosen[s.TargetId]) {
			v := "concise"
			if r.Detail == pb.Detail_DETAIL_DETAILED {
				v = "detailed"
			}
			selection := &pb.Selection{StatementId: s.Id, VariantId: v}
			if topic(s) == "DATA_QUALITY" {
				limits.Selections = append(limits.Selections, selection)
			} else {
				main.Selections = append(main.Selections, selection)
			}
			count++
		}
		if count >= 18 {
			break
		}
	}
	if count == 0 {
		return p
	}
	p.Status = pb.AnswerStatus_ANSWER_STATUS_ANSWERED
	if len(main.Selections) > 0 {
		p.Blocks = append(p.Blocks, main)
	}
	if len(limits.Selections) > 0 {
		p.Blocks = append(p.Blocks, limits)
		if mode(r) == "CASE_SUMMARY" {
			p.Status = pb.AnswerStatus_ANSWER_STATUS_PARTIAL
		}
	}
	selectedTargets := map[string]bool{}
	for _, selection := range main.Selections {
		for _, statement := range r.Context.Statements {
			if statement.Id == selection.StatementId {
				selectedTargets[statement.TargetId] = true
			}
		}
	}
	for _, action := range r.Context.Actions {
		if selectedTargets[action.TargetId] {
			p.ActionIds = append(p.ActionIds, action.Id)
			if len(p.ActionIds) == 3 {
				break
			}
		}
	}
	return p
}

func ValidatePlan(p *pb.AnswerPlan, r *pb.HelpRequest) error {
	if p == nil || p.Status < 1 || p.Status > 5 || len(p.Blocks) > 6 || len(p.ActionIds) > 3 || len(p.ClarifyTargetIds) > 3 {
		return errors.New("INVALID_PLAN")
	}
	statements := map[string]*pb.Statement{}
	for _, s := range r.Context.Statements {
		statements[s.Id] = s
	}
	seen := map[string]bool{}
	count := 0
	hasMain := false
	for _, b := range p.Blocks {
		if b == nil || !blockAllowed(b.Kind, r) {
			return errors.New("INVALID_BLOCK")
		}
		for _, s := range b.Selections {
			if s == nil || statements[s.StatementId] == nil || seen[s.StatementId] || s.VariantId != "concise" && s.VariantId != "detailed" || !relevant(statements[s.StatementId], r) {
				return errors.New("INVALID_SELECTION")
			}
			seen[s.StatementId] = true
			count++
			hasMain = hasMain || primaryTopic(statements[s.StatementId], r)
		}
	}
	if count > 80 {
		return errors.New("TOO_MANY_SELECTIONS")
	}
	for _, s := range r.Context.Statements {
		if s.Required && relevant(s, r) && primaryTopic(s, r) {
			hasMain = true
		}
	}
	if p.Status == pb.AnswerStatus_ANSWER_STATUS_PARTIAL {
		hasLimit, requiredLimit, selectedLimit := false, false, false
		for _, s := range r.Context.Statements {
			if topic(s) == "DATA_QUALITY" {
				hasLimit = true
				requiredLimit = requiredLimit || s.Required
				selectedLimit = selectedLimit || seen[s.Id]
			}
		}
		if hasLimit && !requiredLimit && !selectedLimit {
			return errors.New("MISSING_LIMITATION")
		}
	}
	if p.Status == pb.AnswerStatus_ANSWER_STATUS_ANSWERED && (count == 0 || !hasMain || !primary(r)) {
		return errors.New("EMPTY_ANSWER")
	}
	actions := map[string]bool{}
	targets := map[string]bool{}
	for _, a := range r.Context.Actions {
		actions[a.Id] = true
	}
	for _, t := range r.Context.Targets {
		targets[t.Id] = true
	}
	for _, ids := range []struct {
		values  []string
		allowed map[string]bool
	}{{p.ActionIds, actions}, {p.ClarifyTargetIds, targets}} {
		seen := map[string]bool{}
		for _, id := range ids.values {
			if !ids.allowed[id] || seen[id] {
				return errors.New("UNKNOWN_OR_DUPLICATE_ID")
			}
			seen[id] = true
		}
	}
	if (p.Status == pb.AnswerStatus_ANSWER_STATUS_CLARIFY) != (len(p.ClarifyTargetIds) > 0) {
		return errors.New("INVALID_CLARIFICATION")
	}
	if (p.Status == pb.AnswerStatus_ANSWER_STATUS_OUT_OF_SCOPE || p.Status == pb.AnswerStatus_ANSWER_STATUS_INSUFFICIENT_DATA) && len(p.ActionIds) > 0 {
		return errors.New("UNSUPPORTED_ACTION")
	}
	return nil
}

func Render(r *pb.HelpRequest, plan *pb.AnswerPlan, source pb.ResponseSource, model string, reason pb.FallbackReason, now time.Time) (*pb.HelpResponse, error) {
	if err := Validate(r, now); err != nil {
		return nil, err
	}
	if err := ValidatePlan(plan, r); err != nil {
		return nil, err
	}
	p := proto.Clone(plan).(*pb.AnswerPlan)
	statements := map[string]*pb.Statement{}
	for _, s := range r.Context.Statements {
		statements[s.Id] = s
	}
	if p.Status == pb.AnswerStatus_ANSWER_STATUS_ANSWERED || p.Status == pb.AnswerStatus_ANSWER_STATUS_PARTIAL {
		seen := map[string]bool{}
		pending := []string{}
		for _, s := range r.Context.Statements {
			if s.Required && relevant(s, r) {
				pending = append(pending, s.Id)
			}
		}
		for _, b := range p.Blocks {
			for _, s := range b.Selections {
				seen[s.StatementId] = true
				pending = append(pending, s.StatementId)
			}
		}
		expanded := map[string]bool{}
		for len(pending) > 0 {
			id := pending[0]
			pending = pending[1:]
			s := statements[id]
			if !seen[id] {
				kind := mainBlock(r)
				if topic(s) == "DATA_QUALITY" {
					kind = pb.BlockKind_BLOCK_KIND_LIMITATIONS
				}
				var block *pb.PlanBlock
				for _, b := range p.Blocks {
					if b.Kind == kind {
						block = b
						break
					}
				}
				if block == nil {
					block = &pb.PlanBlock{Kind: kind}
					p.Blocks = append(p.Blocks, block)
				}
				variant := "concise"
				if r.Detail == pb.Detail_DETAIL_DETAILED {
					variant = "detailed"
				}
				block.Selections = append(block.Selections, &pb.Selection{StatementId: id, VariantId: variant})
				seen[id] = true
			}
			if !expanded[id] {
				pending = append(pending, s.CompanionIds...)
				expanded[id] = true
			}
		}
	}
	c := r.Context
	out := &pb.HelpResponse{RequestId: r.RequestId, ContextId: c.ContextId, SourceRevision: c.SourceRevision, GeneratedAt: timestamppb.New(now), ValidUntil: c.ValidUntil, Status: p.Status, Source: source, ModelVersion: model, PromptVersion: "help-plan-v1", CatalogVersion: CatalogVersion, FallbackReason: reason, Plan: p}
	used := map[string]bool{}
	chars := 0
	for _, b := range p.Blocks {
		block := &pb.Block{Kind: b.Kind}
		for _, sel := range b.Selections {
			s := statements[sel.StatementId]
			var text string
			for _, v := range s.Variants {
				if v.Id == sel.VariantId {
					text = v.Text
				}
			}
			block.Items = append(block.Items, &pb.BlockItem{StatementId: s.Id, Text: text, EvidenceIds: s.EvidenceIds})
			chars += utf8.RuneCountInString(text)
			for _, id := range s.EvidenceIds {
				used[id] = true
			}
		}
		out.Blocks = append(out.Blocks, block)
	}
	label := strings.TrimPrefix(p.Status.String(), "ANSWER_STATUS_")
	if label == "CLARIFY" || label == "INSUFFICIENT_DATA" || label == "OUT_OF_SCOPE" {
		text := phrases[label]
		out.Blocks = append(out.Blocks, &pb.Block{Kind: pb.BlockKind_BLOCK_KIND_SUMMARY, Items: []*pb.BlockItem{{Text: text}}})
		chars += utf8.RuneCountInString(text)
	}
	for _, id := range p.ActionIds {
		for _, a := range c.Actions {
			if a.Id == id {
				out.Actions = append(out.Actions, a)
			}
		}
	}
	for _, id := range p.ClarifyTargetIds {
		for _, t := range c.Targets {
			if t.Id == id {
				out.ClarifyTargets = append(out.ClarifyTargets, t)
			}
		}
	}
	evidence := append([]*pb.Evidence(nil), c.Evidence...)
	sort.Slice(evidence, func(i, j int) bool { return evidence[i].Id < evidence[j].Id })
	for _, v := range evidence {
		if used[v.Id] {
			out.Evidence = append(out.Evidence, v)
		}
	}
	limit := 1800
	if c.Audience == pb.Audience_AUDIENCE_ANALYST {
		limit = 5000
	}
	if chars > limit {
		out.Blocks = []*pb.Block{{Kind: pb.BlockKind_BLOCK_KIND_SUMMARY, Items: []*pb.BlockItem{{Text: phrases["LIMITED"]}}}}
		out.Evidence = nil
		out.Actions = nil
		out.ClarifyTargets = nil
		out.Status = pb.AnswerStatus_ANSWER_STATUS_PARTIAL
		out.Source = pb.ResponseSource_RESPONSE_SOURCE_TEMPLATE
		out.ModelVersion = ""
		out.Plan = &pb.AnswerPlan{Status: out.Status}
	}
	return out, nil
}
func Fallback(r *pb.HelpRequest, reason pb.FallbackReason, now time.Time) (*pb.HelpResponse, error) {
	return Render(r, FallbackPlan(r), pb.ResponseSource_RESPONSE_SOURCE_TEMPLATE, "", reason, now)
}
func Call(ctx context.Context, target string, r *pb.HelpRequest) (*pb.HelpResponse, error) {
	if err := Validate(r, time.Now().UTC()); err != nil {
		return nil, err
	}
	reason := pb.FallbackReason_FALLBACK_REASON_MODEL_UNAVAILABLE
	if target != "" {
		conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			defer func() { _ = conn.Close() }()
			callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			response, e := pb.NewAssistantHelpServiceClient(conn).Help(callCtx, r)
			if status.Code(e) == codes.FailedPrecondition {
				return nil, fmt.Errorf("assistant incompatible or stale: %w", e)
			}
			if status.Code(e) == codes.DeadlineExceeded {
				reason = pb.FallbackReason_FALLBACK_REASON_MODEL_TIMEOUT
			}
			if e == nil {
				reason = pb.FallbackReason_FALLBACK_REASON_INVALID_MODEL_OUTPUT
				if response != nil && response.CatalogVersion != CatalogVersion {
					return nil, ErrIncompatible
				}
				if response != nil && response.RequestId == r.RequestId && response.ContextId == r.Context.ContextId && response.SourceRevision == r.Context.SourceRevision && response.Status == response.GetPlan().GetStatus() && response.Source >= 1 && response.Source <= 2 && response.FallbackReason >= 1 && response.FallbackReason <= 7 {
					if rendered, e := Render(r, response.Plan, response.Source, response.ModelVersion, response.FallbackReason, time.Now().UTC()); e == nil {
						return rendered, nil
					}
				}
			}
		}
	}
	return Fallback(r, reason, time.Now().UTC())
}
