import { describe, it, expect } from "vitest";
import type { TrackingIndicatorsResponse } from "~/lib/active-helpers";
import {
  parseTrackingFilter,
  matchesTrackingFilter,
  trackingFilterChipLabel,
  resolveTrackingFilterAfterLabelChange,
  type TrackingFilter,
} from "./tracking-filter";

describe("parseTrackingFilter", () => {
  it('returns { kind: "all" } for "all"', () => {
    expect(parseTrackingFilter("all")).toEqual({ kind: "all" });
  });

  it('returns { kind: "none_visited" } for "none_visited"', () => {
    expect(parseTrackingFilter("none_visited")).toEqual({
      kind: "none_visited",
    });
  });

  it("parses a valid missing:N value", () => {
    expect(parseTrackingFilter("missing:0")).toEqual({
      kind: "missing",
      idx: 0,
    });
    expect(parseTrackingFilter("missing:2")).toEqual({
      kind: "missing",
      idx: 2,
    });
  });

  it('rejects "missing:" with an empty index', () => {
    // Guards against Number("") === 0 silently matching the first label.
    expect(parseTrackingFilter("missing:")).toEqual({ kind: "invalid" });
  });

  it("rejects non-numeric indices", () => {
    expect(parseTrackingFilter("missing:abc")).toEqual({ kind: "invalid" });
    expect(parseTrackingFilter("missing:-1")).toEqual({ kind: "invalid" });
    expect(parseTrackingFilter("missing:1.5")).toEqual({ kind: "invalid" });
  });

  it("rejects unknown values", () => {
    expect(parseTrackingFilter("")).toEqual({ kind: "invalid" });
    expect(parseTrackingFilter("foobar")).toEqual({ kind: "invalid" });
  });
});

describe("matchesTrackingFilter", () => {
  const data: TrackingIndicatorsResponse = {
    labels: ["Hausaufgaben", "Mensa"],
    results: {
      "1": [true, true], // both done
      "2": [true, false], // HA done, Mensa pending
      "3": [false, true], // HA pending, Mensa done
      "4": [false, false], // neither
    },
  };

  describe('filter "all"', () => {
    it("keeps every student regardless of tracking state", () => {
      expect(matchesTrackingFilter({ id: "1" }, "all", data)).toBe(true);
      expect(matchesTrackingFilter({ id: "999" }, "all", data)).toBe(true);
      expect(matchesTrackingFilter({ id: "1" }, "all", undefined)).toBe(true);
    });

    it("keeps redacted students under 'all'", () => {
      expect(
        matchesTrackingFilter({ id: "1", has_full_access: false }, "all", data),
      ).toBe(true);
    });
  });

  describe("redaction (has_full_access === false)", () => {
    it("hides redacted students when a tracking filter is active", () => {
      expect(
        matchesTrackingFilter(
          { id: "4", has_full_access: false },
          "none_visited",
          data,
        ),
      ).toBe(false);
      expect(
        matchesTrackingFilter(
          { id: "4", has_full_access: false },
          "missing:0",
          data,
        ),
      ).toBe(false);
    });
  });

  describe("stale/missing tracking data (SWR revalidation)", () => {
    it("hides students whose id is not in the results map", () => {
      expect(matchesTrackingFilter({ id: "999" }, "none_visited", data)).toBe(
        false,
      );
      expect(matchesTrackingFilter({ id: "999" }, "missing:0", data)).toBe(
        false,
      );
    });

    it("hides all students when trackingData is undefined", () => {
      expect(
        matchesTrackingFilter({ id: "1" }, "none_visited", undefined),
      ).toBe(false);
      expect(matchesTrackingFilter({ id: "1" }, "missing:0", undefined)).toBe(
        false,
      );
    });
  });

  describe('filter "none_visited"', () => {
    it("keeps students with every indicator false", () => {
      expect(matchesTrackingFilter({ id: "4" }, "none_visited", data)).toBe(
        true,
      );
    });

    it("hides students with at least one indicator true", () => {
      expect(matchesTrackingFilter({ id: "1" }, "none_visited", data)).toBe(
        false,
      );
      expect(matchesTrackingFilter({ id: "2" }, "none_visited", data)).toBe(
        false,
      );
      expect(matchesTrackingFilter({ id: "3" }, "none_visited", data)).toBe(
        false,
      );
    });

    it("hides everyone when no labels are configured", () => {
      const emptyLabels: TrackingIndicatorsResponse = {
        labels: [],
        results: { "1": [] },
      };
      expect(
        matchesTrackingFilter({ id: "1" }, "none_visited", emptyLabels),
      ).toBe(false);
    });
  });

  describe('filter "missing:idx"', () => {
    it("keeps students whose label at idx is false", () => {
      expect(matchesTrackingFilter({ id: "3" }, "missing:0", data)).toBe(true); // HA pending
      expect(matchesTrackingFilter({ id: "2" }, "missing:1", data)).toBe(true); // Mensa pending
      expect(matchesTrackingFilter({ id: "4" }, "missing:0", data)).toBe(true);
      expect(matchesTrackingFilter({ id: "4" }, "missing:1", data)).toBe(true);
    });

    it("hides students whose label at idx is true", () => {
      expect(matchesTrackingFilter({ id: "2" }, "missing:0", data)).toBe(false); // HA done
      expect(matchesTrackingFilter({ id: "3" }, "missing:1", data)).toBe(false); // Mensa done
      expect(matchesTrackingFilter({ id: "1" }, "missing:0", data)).toBe(false);
      expect(matchesTrackingFilter({ id: "1" }, "missing:1", data)).toBe(false);
    });

    it("hides everyone when idx is out of range (stale after admin removed a label)", () => {
      expect(matchesTrackingFilter({ id: "4" }, "missing:2", data)).toBe(false);
      expect(matchesTrackingFilter({ id: "4" }, "missing:99", data)).toBe(
        false,
      );
    });
  });

  describe("defensive: results array shorter than labels", () => {
    // Backend guarantees results.length === labels.length, but document the
    // fallback: missing slots are treated as undefined (falsy), so a short array
    // with no truthy entries still counts as "none_visited", and missing:<idx>
    // beyond the array reads undefined !== true → shown.
    const short: TrackingIndicatorsResponse = {
      labels: ["A", "B"],
      results: { "1": [true], "2": [false] },
    };

    it("keeps a student under missing:<oob> because results[oob] is undefined", () => {
      expect(matchesTrackingFilter({ id: "1" }, "missing:1", short)).toBe(true);
    });

    it("hides a student under none_visited if any present entry is true", () => {
      expect(matchesTrackingFilter({ id: "1" }, "none_visited", short)).toBe(
        false,
      );
    });

    it("keeps a student under none_visited if every present entry is false", () => {
      expect(matchesTrackingFilter({ id: "2" }, "none_visited", short)).toBe(
        true,
      );
    });
  });

  describe("invalid filter values", () => {
    it("hides everyone on garbage input", () => {
      expect(
        matchesTrackingFilter({ id: "4" }, "garbage" as TrackingFilter, data),
      ).toBe(false);
    });
  });
});

describe("trackingFilterChipLabel", () => {
  const labels = ["Hausaufgaben", "Mensa"];

  it('returns null for "all" (chip should not render)', () => {
    expect(trackingFilterChipLabel("all", labels)).toBeNull();
  });

  it('returns "Noch nichts erledigt" for none_visited', () => {
    expect(trackingFilterChipLabel("none_visited", labels)).toBe(
      "Noch nichts erledigt",
    );
  });

  it("returns Noch nicht in <label> for a valid missing:idx", () => {
    expect(trackingFilterChipLabel("missing:0", labels)).toBe(
      "Noch nicht in Hausaufgaben",
    );
    expect(trackingFilterChipLabel("missing:1", labels)).toBe(
      "Noch nicht in Mensa",
    );
  });

  it("returns null when missing:idx points past the labels array", () => {
    expect(trackingFilterChipLabel("missing:5", labels)).toBeNull();
  });

  it("returns null for invalid filter values", () => {
    expect(
      trackingFilterChipLabel("garbage" as TrackingFilter, labels),
    ).toBeNull();
  });
});

describe("resolveTrackingFilterAfterLabelChange", () => {
  const data: TrackingIndicatorsResponse = {
    labels: ["Hausaufgaben", "Mensa"],
    results: {},
  };

  it("returns the filter unchanged while trackingData is still loading", () => {
    expect(resolveTrackingFilterAfterLabelChange("missing:0", undefined)).toBe(
      "missing:0",
    );
    expect(
      resolveTrackingFilterAfterLabelChange("none_visited", undefined),
    ).toBe("none_visited");
  });

  it('leaves "all" alone in every state', () => {
    expect(resolveTrackingFilterAfterLabelChange("all", undefined)).toBe("all");
    expect(resolveTrackingFilterAfterLabelChange("all", data)).toBe("all");
    expect(
      resolveTrackingFilterAfterLabelChange("all", {
        labels: [],
        results: {},
      }),
    ).toBe("all");
  });

  it("leaves a valid missing:idx and none_visited unchanged", () => {
    expect(resolveTrackingFilterAfterLabelChange("missing:0", data)).toBe(
      "missing:0",
    );
    expect(resolveTrackingFilterAfterLabelChange("missing:1", data)).toBe(
      "missing:1",
    );
    expect(resolveTrackingFilterAfterLabelChange("none_visited", data)).toBe(
      "none_visited",
    );
  });

  it('resets to "all" when labels shrink past the selected index', () => {
    expect(resolveTrackingFilterAfterLabelChange("missing:5", data)).toBe(
      "all",
    );
    expect(resolveTrackingFilterAfterLabelChange("missing:2", data)).toBe(
      "all",
    );
  });

  it('resets to "all" when labels are empty', () => {
    const empty: TrackingIndicatorsResponse = { labels: [], results: {} };
    expect(resolveTrackingFilterAfterLabelChange("missing:0", empty)).toBe(
      "all",
    );
    expect(resolveTrackingFilterAfterLabelChange("none_visited", empty)).toBe(
      "all",
    );
  });

  it('resets to "all" on invalid filter input', () => {
    expect(
      resolveTrackingFilterAfterLabelChange("garbage" as TrackingFilter, data),
    ).toBe("all");
  });
});
