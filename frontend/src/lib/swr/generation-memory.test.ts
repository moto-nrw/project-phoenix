import { beforeEach, describe, expect, it, vi } from "vitest";

// generation.ts imports SWR's preload; the global test mock has none.
vi.unmock("swr");

import {
  _resetSWRFallbacksForTests,
  _swrStampedKeyCountForTests,
  bumpDataGeneration,
  bumpScopeGeneration,
  dataGenerationOf,
  isCurrentSWRValue,
  isFallbackVisible,
  isVisibleSWRValue,
  noteFallbackShown,
  stampSWRValue,
} from "./generation";

// Stamps live as long as their scope generation (#5223).
beforeEach(() => {
  _resetSWRFallbacksForTests();
  bumpScopeGeneration();
});

describe("stamp metadata in the browser", () => {
  it("keeps the metadata of the current scope only", () => {
    const KEYS = 25;
    for (let scope = 0; scope < 40; scope += 1) {
      bumpScopeGeneration();
      expect(_swrStampedKeyCountForTests()).toBe(0);
      for (let i = 0; i < KEYS; i += 1) {
        const key = `school-${scope}:account-${scope}:list-${i}`;
        stampSWRValue(key, { scope, i }, dataGenerationOf(key));
        stampSWRValue(`${key}-total`, i, dataGenerationOf(key));
        noteFallbackShown(`${key}-fallback`, dataGenerationOf(key));
      }
      // Anker: innerhalb der Generation zählen die gestempelten Keys.
      expect(_swrStampedKeyCountForTests()).toBe(KEYS * 3);
    }
  });

  it.each([
    ["object", { rows: ["alt"] }, { rows: ["neu"] }],
    ["primitive", "alt", "neu"],
  ] as const)("lets no released %s count again", (_kind, old, fresh) => {
    const key = "school-a:A:roster";
    stampSWRValue(key, old, dataGenerationOf(key));
    // Anker: der Wert gilt in seiner Generation.
    expect(isVisibleSWRValue(key, old)).toBe(true);

    bumpScopeGeneration();
    expect(isCurrentSWRValue(key, old)).toBeUndefined();
    expect(isVisibleSWRValue(key, old)).toBe(false);
    // Neither a further boundary nor fresh data of the key revives it.
    bumpScopeGeneration();
    expect(isVisibleSWRValue(key, old)).toBe(false);
    stampSWRValue(key, fresh, dataGenerationOf(key));
    expect(isVisibleSWRValue(key, fresh)).toBe(true);
    expect(isVisibleSWRValue(key, old)).toBe(false);
  });

  it("hides an unseen fallback after a boundary released stamps", () => {
    const key = "school-a:A:student-7-detail";
    const other = "school-a:A:student-8-detail";
    // Anker: auf einer frischen Seite gilt ein ungesehener Fallback.
    expect(isFallbackVisible(other)).toBe(true);
    stampSWRValue(key, { name: "alt" }, dataGenerationOf(key));
    bumpScopeGeneration();

    expect(isFallbackVisible(key)).toBe(false);
    expect(isFallbackVisible(other)).toBe(false);
    // Fresh data of the current generation assigns the key again.
    stampSWRValue(other, { name: "neu" }, dataGenerationOf(other));
    expect(isFallbackVisible(other)).toBe(true);
  });

  it("rejects a value without a stamp of a key a data bump hit", () => {
    const hit = "school-a:A:student-7-detail";
    const kept = "school-a:A:student-8-detail";
    // Anker: ohne Freigabe und ohne Bump gilt ein ungestempelter Wert.
    expect(isVisibleSWRValue(hit, "roh")).toBe(true);

    bumpDataGeneration((key) => key.includes("student-7-"));

    expect(isVisibleSWRValue(hit, "roh")).toBe(false);
    expect(isVisibleSWRValue(kept, "roh")).toBe(true);
  });
});
