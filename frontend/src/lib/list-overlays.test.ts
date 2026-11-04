import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  addListOverlay,
  applyListOverlays,
  startListFetch,
  useListOverlays,
} from "./list-overlays";

const applyToItems = (items: string[], update: (items: string[]) => string[]) =>
  update(items);

describe("list overlays (#4988)", () => {
  it("applies a change only to data fetched before it", () => {
    const list = "school-a|requests:1:";
    const before = startListFetch();
    addListOverlay<string>(list, (items) => items.filter((id) => id !== "x"));
    const after = startListFetch();
    const { result } = renderHook(() => useListOverlays<string>(list));

    // Anker: der ältere Stand verliert die entschiedene Zeile.
    expect(
      applyListOverlays(["x", "y"], before, result.current, applyToItems),
    ).toEqual(["y"]);
    // Ein später begonnener Abruf kennt die Entscheidung und gilt unverändert.
    expect(
      applyListOverlays(["x", "y"], after, result.current, applyToItems),
    ).toEqual(["x", "y"]);
  });

  it("retains every change while an older snapshot can still be shown", () => {
    const list = "school-a|requests:many:";
    const fetchedAt = startListFetch();
    const items = Array.from({ length: 102 }, (_, index) => String(index));
    const { result } = renderHook(() => useListOverlays<string>(list));

    act(() => {
      for (const id of items.slice(0, 101)) {
        addListOverlay<string>(list, (current) =>
          current.filter((item) => item !== id),
        );
      }
    });

    expect(
      applyListOverlays(items, fetchedAt, result.current, applyToItems),
    ).toEqual(["101"]);
  });

  it("keeps the changes of one list away from another", () => {
    const fetchedAt = startListFetch();
    addListOverlay<string>("school-a|requests:2:", () => []);
    const { result } = renderHook(() =>
      useListOverlays<string>("school-b|requests:2:"),
    );

    expect(
      applyListOverlays(["x"], fetchedAt, result.current, applyToItems),
    ).toEqual(["x"]);
  });

  it("renders the list again when a change arrives", () => {
    const list = "school-a|requests:3:";
    const fetchedAt = startListFetch();
    const { result } = renderHook(() => useListOverlays<string>(list));
    expect(
      applyListOverlays(["x"], fetchedAt, result.current, applyToItems),
    ).toEqual(["x"]);

    act(() => addListOverlay<string>(list, () => []));

    expect(
      applyListOverlays(["x"], fetchedAt, result.current, applyToItems),
    ).toEqual([]);
  });
});
