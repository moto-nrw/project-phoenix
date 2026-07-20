import { describe, it, expect } from "vitest";
import { GUIDE_SOURCES, helpGuidePaths } from "./guide-search";
import { guideEntryPoints } from "./guide-data";

/**
 * Guard against silent search drift: the search index pulls its guides from
 * `GUIDE_SOURCES`, while the help landing page advertises full guide pages via
 * searchable `guideEntryPoints`. If a guide is renamed/added/removed in one
 * place but not the other, it would still render on the page yet vanish from
 * search. These tests force the two lists to stay aligned.
 */
describe("help search ↔ guide entry point sync", () => {
  const searchableEntryPoints = guideEntryPoints.filter(
    (entry) => entry.searchable !== false,
  );

  it("every searchable guide path has a matching landing-page entry point", () => {
    const entryHrefs = new Set(searchableEntryPoints.map((e) => e.href));
    for (const path of helpGuidePaths) {
      expect(entryHrefs).toContain(path);
    }
  });

  it("every landing-page entry point is covered by the search index", () => {
    const sourcePaths = new Set(GUIDE_SOURCES.map((g) => g.path));
    for (const entry of searchableEntryPoints) {
      expect(sourcePaths).toContain(entry.href);
    }
  });

  it("guide labels match their advertised titles", () => {
    const titleByHref = new Map(
      searchableEntryPoints.map((e) => [e.href, e.title]),
    );
    for (const guide of GUIDE_SOURCES) {
      expect(guide.label).toBe(titleByHref.get(guide.path));
    }
  });
});
