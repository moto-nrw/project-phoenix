import "@testing-library/jest-dom/vitest";
import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useGroupedItems, type Grouper } from "./use-grouped-items";

interface Item {
  id: string;
  type: string;
  name: string;
}

const items: Item[] = [
  { id: "1", type: "kiosk", name: "Eingang" },
  { id: "2", type: "info_point", name: "Foyer Info" },
  { id: "3", type: "kiosk", name: "Backup" },
];

const groupers: Record<"type", Grouper<Item>> = {
  type: (item) => ({ id: item.type, title: item.type }),
};

describe("useGroupedItems", () => {
  it("returns a single flat bucket when mode is 'none'", () => {
    const { result } = renderHook(() =>
      useGroupedItems<Item, "type">(items, "none", groupers, "Geräte"),
    );
    expect(result.current).toHaveLength(1);
    expect(result.current[0]?.id).toBe("__flat__");
    expect(result.current[0]?.title).toBe("Alle Geräte (3)");
    expect(result.current[0]?.items).toHaveLength(3);
  });

  it("groups items by the selected grouper and includes a count in each title", () => {
    const { result } = renderHook(() =>
      useGroupedItems<Item, "type">(items, "type", groupers, "Geräte"),
    );
    const ids = result.current.map((g) => g.id);
    expect(ids).toContain("kiosk");
    expect(ids).toContain("info_point");
    const kiosk = result.current.find((g) => g.id === "kiosk");
    expect(kiosk?.title).toBe("kiosk (2)");
    expect(kiosk?.items).toHaveLength(2);
  });

  it("falls back to the flat bucket when the selected grouper is not registered", () => {
    const { result } = renderHook(() =>
      useGroupedItems<Item, "type" | "missing">(
        items,
        "missing",
        groupers,
        "Geräte",
      ),
    );
    expect(result.current).toHaveLength(1);
    expect(result.current[0]?.id).toBe("__flat__");
  });

  it("returns an empty array when there are no items", () => {
    const { result } = renderHook(() =>
      useGroupedItems<Item, "type">([], "type", groupers, "Geräte"),
    );
    expect(result.current).toEqual([]);
  });
});
