import assert from "node:assert/strict";
import test from "node:test";
import { tidyBlocks } from "../src/lib/helpBlocks.ts";

const limitation = { statement_id: "limitations", text: "Демонстрационные сведения синтетические." };

test("model blocks of one kind merge and the limitation closes the answer once", () => {
  const out = tidyBlocks([
    { kind: "BLOCK_KIND_DETAILS", items: [{ statement_id: "s:decision", text: "Статус: «деньги выданы»." }] },
    { kind: "BLOCK_KIND_DETAILS", items: [limitation] },
    { kind: "BLOCK_KIND_LIMITATIONS", items: [limitation] },
  ]);
  assert.deepEqual(out.map((b) => [b.block.kind, b.title, b.block.items.length]), [
    ["BLOCK_KIND_DETAILS", undefined, 1],
    ["BLOCK_KIND_LIMITATIONS", "Ограничения", 1],
  ]);
});

test("details after a summary keep their heading", () => {
  const out = tidyBlocks([
    { kind: "BLOCK_KIND_SUMMARY", items: [{ statement_id: "a", text: "Итог" }] },
    { kind: "BLOCK_KIND_DETAILS", items: [{ statement_id: "b", text: "Детали" }] },
  ]);
  assert.deepEqual(out.map((b) => b.title), [undefined, "Подробнее"]);
});


test("identical statements in different blocks are shown once", () => {
  const out = tidyBlocks([
    { kind: "BLOCK_KIND_DETAILS", items: [{ statement_id: "a", text: "Проверьте статус заявки." }] },
    { kind: "BLOCK_KIND_NEXT_STEPS", items: [{ statement_id: "b", text: "Проверьте статус заявки." }] },
  ]);
  assert.equal(out.length, 1);
  assert.equal(out[0].block.items[0].text, "Проверьте статус заявки.");
});
