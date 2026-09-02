import { describe, expect, it } from "vitest";
import type { ResolvedSetting, SchemaTab } from "~/lib/settings-api";
import {
  categorySummary,
  changedCount,
  filterCategoryItems,
  normalizeQuery,
  searchTabs,
} from "./settings-filter";

function setting(
  overrides: Partial<ResolvedSetting> & Pick<ResolvedSetting, "key" | "label">,
): ResolvedSetting {
  return {
    description: "",
    type: "boolean",
    default: true,
    value: true,
    is_default: true,
    writable: true,
    visible: true,
    sort_order: 0,
    access_policy: "shared",
    ...overrides,
  };
}

const tabs: SchemaTab[] = [
  {
    key: "operations",
    label: "operations",
    categories: [
      {
        key: "elternportal",
        label: "elternportal",
        items: [
          setting({
            key: "parent.sick_note",
            label: "Krankmeldung über Elternportal",
            description: "Eltern melden ihr Kind krank.",
          }),
          setting({
            key: "parent.messages",
            label: "Eltern-OGS-Nachrichten",
            description: "Eltern schreiben dem Team.",
            visible: false,
          }),
        ],
      },
      {
        key: "stundenplan",
        label: "stundenplan",
        items: [
          setting({
            key: "timetable.enabled",
            label: "Betreuungsplan anzeigen",
          }),
          setting({
            key: "timetable.ratio",
            label: "Betreuungsschlüssel",
            is_default: false,
          }),
        ],
      },
    ],
  },
  {
    key: "gdpr",
    label: "gdpr",
    categories: [
      {
        key: "bewegungsdaten",
        label: "bewegungsdaten",
        items: [
          setting({
            key: "gdpr.retention",
            label: "Aufbewahrung",
            description: "Wie lange Besuchsdaten bleiben.",
          }),
        ],
      },
    ],
  },
];

describe("normalizeQuery", () => {
  it("trims and lower-cases", () => {
    expect(normalizeQuery("  Krank ")).toBe("krank");
    expect(normalizeQuery("   ")).toBe("");
  });
});

describe("filterCategoryItems", () => {
  it("returns the visible items without a query", () => {
    const items = filterCategoryItems(tabs[0]!.categories[0]!, "");
    expect(items.map((item) => item.key)).toEqual(["parent.sick_note"]);
  });

  it("matches label and description", () => {
    expect(
      filterCategoryItems(tabs[1]!.categories[0]!, "besuchsdaten").map(
        (item) => item.key,
      ),
    ).toEqual(["gdpr.retention"]);
    expect(filterCategoryItems(tabs[1]!.categories[0]!, "xyz")).toEqual([]);
  });

  it("keeps every item when the displayed category name matches", () => {
    const items = filterCategoryItems(
      tabs[0]!.categories[1]!,
      "betreuungsplan",
    );
    expect(items).toHaveLength(2);
  });
});

describe("searchTabs", () => {
  it("collects hits across tabs with their tab and category", () => {
    const hits = searchTabs(tabs, "eltern");
    expect(hits).toHaveLength(1);
    expect(hits[0]!.tab.key).toBe("operations");
    expect(hits[0]!.category.key).toBe("elternportal");
    expect(hits[0]!.items.map((item) => item.key)).toEqual([
      "parent.sick_note",
    ]);
  });

  it("ignores hidden settings", () => {
    expect(searchTabs(tabs, "nachrichten")).toEqual([]);
  });
});

describe("categorySummary / changedCount", () => {
  it("joins up to three labels and counts the rest", () => {
    const items = ["A", "B", "C", "D", "E"].map((label) =>
      setting({ key: label, label }),
    );
    expect(categorySummary(items.slice(0, 2))).toBe("A, B");
    expect(categorySummary(items.slice(0, 3))).toBe("A, B, C");
    expect(categorySummary(items.slice(0, 4))).toBe("A, B, C und 1 weitere");
    expect(categorySummary(items)).toBe("A, B, C und 2 weitere");
  });

  it("counts settings that differ from their default", () => {
    expect(changedCount(tabs[0]!.categories[1]!.items)).toBe(1);
    expect(changedCount(tabs[1]!.categories[0]!.items)).toBe(0);
  });
});
