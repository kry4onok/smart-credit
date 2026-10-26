import type { HelpResponse } from "../api/types";

export const blockTitle: Record<string, string> = {
  BLOCK_KIND_DETAILS: "Подробнее",
  BLOCK_KIND_LIMITATIONS: "Ограничения",
  BLOCK_KIND_NEXT_STEPS: "Что сделать",
  BLOCK_KIND_CHANGES: "Что изменилось",
  BLOCK_KIND_COMPARISON: "Сравнение",
};

type Block = NonNullable<HelpResponse["blocks"]>[number];

// tidyBlocks presents the model's plan without repetition: blocks of one kind are merged, the
// standard limitation (statement "limitations") always closes the answer, and the first main
// block needs no "Подробнее" heading because nothing precedes it.
export function tidyBlocks(blocks: Block[]): { block: Block; title?: string }[] {
  const merged: Block[] = [];
  const limits: NonNullable<Block["items"]> = [];
  for (const b of blocks) {
    const items = (b.items ?? []).filter((item) => {
      if (item.statement_id !== "limitations" && b.kind !== "BLOCK_KIND_LIMITATIONS") return true;
      if (!limits.some((l) => l.text === item.text)) limits.push(item);
      return false;
    });
    if (!items.length) continue;
    const same = merged.find((m) => m.kind === b.kind);
    if (same) same.items = [...(same.items ?? []), ...items];
    else merged.push({ ...b, items });
  }
  if (limits.length) merged.push({ kind: "BLOCK_KIND_LIMITATIONS", items: limits });
  return merged.map((block, i) => ({
    block,
    title: i === 0 && block.kind === "BLOCK_KIND_DETAILS" ? undefined : block.kind && blockTitle[block.kind],
  }));
}
