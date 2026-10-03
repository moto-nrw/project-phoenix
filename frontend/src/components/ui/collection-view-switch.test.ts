import { describe, expect, it, vi } from "vitest";

import { columnMenuEntries } from "./collection-view-switch";

const COLUMNS = [
  { key: "name", header: "Name", hideable: false },
  { key: "class", header: "Klasse" },
  { key: "group", header: "Gruppe" },
];

describe("columnMenuEntries (#3834)", () => {
  it("offers a heading and one switch per hideable column", () => {
    const entries = columnMenuEntries(COLUMNS, new Set(["group"]), vi.fn());

    expect(entries).toMatchObject([
      { kind: "header", label: "Spalten in der Liste" },
      { kind: "checkbox", label: "Klasse", checked: true, keepOpen: true },
      { kind: "checkbox", label: "Gruppe", checked: false, keepOpen: true },
    ]);
  });

  it("switches a column to the opposite of its state", () => {
    const onChange = vi.fn();
    const entries = columnMenuEntries(COLUMNS, new Set(["group"]), onChange);

    for (const entry of entries) {
      if ("kind" in entry && entry.kind === "checkbox") entry.onClick();
    }

    expect(onChange).toHaveBeenCalledWith("class", false);
    expect(onChange).toHaveBeenCalledWith("group", true);
  });

  it("keeps the last shown column switched on", () => {
    const entries = columnMenuEntries(COLUMNS, new Set(["group"]), vi.fn());

    expect(entries[1]).toMatchObject({ label: "Klasse", disabled: true });
    expect(entries[2]).toMatchObject({ label: "Gruppe", disabled: false });
  });

  it("adds nothing when no column can be hidden", () => {
    expect(columnMenuEntries([COLUMNS[0]!], new Set(), vi.fn())).toEqual([]);
  });
});
