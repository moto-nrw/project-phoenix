import { describe, expect, it, vi } from "vitest";

import {
  columnMenuEntries,
  phoneDetailMenuEntries,
} from "./collection-view-switch";

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

describe("phoneDetailMenuEntries (#3834)", () => {
  const columns = [
    { key: "name", header: "Name", hideable: false, stacked: "title" },
    { key: "status", header: "Aufenthalt", stacked: "meta" },
    { key: "class", header: "Klasse" },
    { key: "pickup", header: "Gehzeit" },
  ];

  it("offers none plus every column that is not part of the row itself", () => {
    const entries = phoneDetailMenuEntries(columns, "pickup", vi.fn());

    expect(entries).toMatchObject([
      { kind: "header", label: "In der Zeile zeigen" },
      { kind: "radio", label: "Nur Name und Status", checked: false },
      { kind: "radio", label: "Klasse", checked: false },
      { kind: "radio", label: "Gehzeit", checked: true },
    ]);
  });

  it("reports the chosen column, or null for none", () => {
    const onChange = vi.fn();
    const entries = phoneDetailMenuEntries(columns, "pickup", onChange);

    for (const entry of entries) {
      if ("kind" in entry && entry.kind === "radio") entry.onClick();
    }

    expect(onChange.mock.calls).toEqual([[null], ["class"], ["pickup"]]);
  });
});
