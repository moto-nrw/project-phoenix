import { describe, it, expect } from "vitest";
import {
  buildHelpSearchIndex,
  fold,
  foldForMatch,
  GUIDE_SOURCES,
  helpGuidePaths,
  helpSearchIndex,
  indexedHelpSearchIndex,
} from "./guide-search";
import { setupChapters, appChapters, nfcChapters } from "./guide-data";

describe("buildHelpSearchIndex", () => {
  const index = buildHelpSearchIndex();

  it("emits exactly one record per step card (no chapter-level records)", () => {
    const allChapters = GUIDE_SOURCES.flatMap((guide) => guide.chapters);
    const expected = allChapters.reduce(
      (total, chapter) => total + chapter.steps.length,
      0,
    );
    expect(index).toHaveLength(expected);
  });

  it("covers all guide paths", () => {
    const paths = new Set(index.map((r) => r.href.split("#")[0]));
    expect(paths).toEqual(new Set(helpGuidePaths));
  });

  it("indexes the NFC quickstart page", () => {
    expect(index).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          href: "/help/nfc/erste-schritte#tablet-montieren-und-einschalten",
          guideLabel: "NFC Erste Schritte",
          title: "Tablet montieren und einschalten",
        }),
      ]),
    );
  });

  it("builds a deep-link href from the guide path and the step id", () => {
    const firstChapter = setupChapters[0]!;
    const firstStep = firstChapter.steps[0]!;
    const stepRecord = index.find(
      (r) => r.href === `/help/setup#${firstStep.id}`,
    );
    expect(stepRecord).toMatchObject({
      guideLabel: "Ersteinrichtung",
      chapterTitle: firstChapter.title,
      title: firstStep.title,
    });
  });

  it("exposes the chapter heading as its own field and folds the intro into the body", () => {
    const firstChapter = setupChapters[0]!;
    const firstStep = firstChapter.steps[0]!;
    const record = index.find((r) => r.href === `/help/setup#${firstStep.id}`);
    // Heading lives in its own weighted field (chapterFold), not duplicated in
    // the body; the chapter intro is what gets folded into the card haystack.
    expect(record?.chapterTitle).toBe(firstChapter.title);
    expect(record?.body).toContain(firstChapter.description);
  });

  it("folds step detail text (steps, checklist, callout) into the body", () => {
    // Find a step that carries a callout and assert its text reaches the body.
    for (const chapter of setupChapters) {
      for (const step of chapter.steps) {
        if (step.callout) {
          const record = index.find((r) => r.href === `/help/setup#${step.id}`);
          expect(record?.body).toContain(step.callout.title);
          return;
        }
      }
    }
    throw new Error("expected at least one setup step with a callout");
  });

  it("indexes hidden aliases for renamed areas", () => {
    const record = index.find(
      (entry) => entry.href === "/help/features#kindersuche",
    );
    expect(record?.searchTerms).toContain("Kindersuche");
  });

  it("produces unique record ids", () => {
    const ids = index.map((r) => r.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it("exposes the prebuilt index with the same contents", () => {
    expect(helpSearchIndex).toHaveLength(index.length);
  });
});

/**
 * Every chapter and step id doubles as a URL anchor: the guide page renders
 * `<section id={chapter.id}>` / `<article id={step.id}>` and both the in-page
 * table of contents and the search results deep-link to `#${id}`. A non-slug id
 * (uppercase, spaces, umlauts) produces a fragment that won't survive
 * encode/decode round-tripping, so the deep link silently lands nowhere. These
 * ids are hand-authored in guide-data, so guard the format here — the build
 * fails loud the moment someone adds a malformed id, instead of a user clicking
 * a result into the void months later.
 */
describe("anchor id format (deep-link safety)", () => {
  const SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
  const allChapters = [...setupChapters, ...appChapters, ...nfcChapters];

  it("every chapter id is a URL-safe slug", () => {
    for (const chapter of allChapters) {
      expect(chapter.id, `chapter id "${chapter.id}"`).toMatch(SLUG);
    }
  });

  it("every step id is a URL-safe slug", () => {
    for (const chapter of allChapters) {
      for (const step of chapter.steps) {
        expect(step.id, `step id "${step.id}"`).toMatch(SLUG);
      }
    }
  });
});

describe("fold (length-preserving, for highlighting)", () => {
  it("strips umlaut diacritics without changing length", () => {
    expect(fold("Räume")).toBe("raume");
    expect(fold("Räume")).toHaveLength("Räume".length);
    expect(fold("Über")).toBe("uber");
  });

  it("keeps ß as a single character so highlight offsets stay aligned", () => {
    expect(fold("Straße")).toBe("straße");
    expect(fold("Straße")).toHaveLength("Straße".length);
  });
});

describe("foldForMatch (aggressive, for fuzzy matching)", () => {
  it("expands ß to ss so 'strasse' and 'Straße' fold to the same key", () => {
    expect(foldForMatch("Straße")).toBe("strasse");
    expect(foldForMatch("strasse")).toBe("strasse");
  });

  it("also folds umlauts like the base fold", () => {
    expect(foldForMatch("Frühstück")).toBe("fruhstuck");
  });
});

describe("indexedHelpSearchIndex", () => {
  it("pairs every record with precomputed folded fields", () => {
    expect(indexedHelpSearchIndex).toHaveLength(helpSearchIndex.length);
    for (const entry of indexedHelpSearchIndex) {
      expect(entry.titleFold).toBe(foldForMatch(entry.record.title));
      expect(entry.searchTermFolds).toEqual(
        entry.record.searchTerms.map(foldForMatch),
      );
      expect(entry.bodyFold).toBe(foldForMatch(entry.record.body));
    }
  });
});
