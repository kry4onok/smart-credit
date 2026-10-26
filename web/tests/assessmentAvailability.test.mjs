import assert from "node:assert/strict";
import test from "node:test";
import { offerBasis } from "../src/lib/assessmentAvailability.ts";

const now = Date.parse("2026-10-26T12:00:00Z");
const expired = { candidate_id: "old", valid_until: "2026-10-26T12:00:00Z" };
const current = { candidate_id: "new", valid_until: "2026-10-27T12:00:00Z" };
const data = { freshness: "CURRENT", assessment: { status: "READY_FOR_CONFIRMATION", valid_until: current.valid_until, candidates: [expired, current] } };

test("only unexpired candidates from a current assessment can be confirmed", () => {
  assert.deepEqual(offerBasis(data, now).candidates, [current]);
  for (const freshness of ["PENDING", "STALE"]) {
    assert.deepEqual(offerBasis({ ...data, freshness }, now).candidates, []);
  }
  assert.deepEqual(offerBasis(data, Date.parse(current.valid_until)).candidates, []);
  assert.equal(offerBasis(data, Date.parse(current.valid_until)).stale, true);
});

test("pending v2 data does not become an unrestricted legacy offer", () => {
  assert.equal(offerBasis(null, now), null);
  assert.deepEqual(offerBasis({ freshness: "PENDING", assessment: null }, now), { stale: true, status: undefined, candidates: [] });
});
