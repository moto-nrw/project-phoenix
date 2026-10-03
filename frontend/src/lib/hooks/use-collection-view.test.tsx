import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useCollectionView } from "./use-collection-view";

const sessionUser = vi.hoisted(() => ({
  current: { id: "7", tenantId: 3 } as { id: string; tenantId: number },
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { user: sessionUser.current } }),
}));

const COLUMNS = [
  { id: "class", defaultVisible: true },
  { id: "notes", defaultVisible: false },
];

describe("useCollectionView (#3834)", () => {
  beforeEach(() => {
    localStorage.clear();
    sessionUser.current = { id: "7", tenantId: 3 };
  });
  afterEach(() => {
    localStorage.clear();
  });

  it("opens with tiles and each column's default", () => {
    const { result } = renderHook(() => useCollectionView("page", COLUMNS));

    expect(result.current.view).toBe("tiles");
    expect([...result.current.hiddenColumns]).toEqual(["notes"]);
  });

  it("remembers the view and switched columns", () => {
    const { result } = renderHook(() => useCollectionView("page", COLUMNS));

    act(() => result.current.setView("table"));
    act(() => result.current.setColumnVisible("class", false));
    act(() => result.current.setColumnVisible("notes", true));

    expect(result.current.view).toBe("table");
    expect([...result.current.hiddenColumns]).toEqual(["class"]);

    const { result: reopened } = renderHook(() =>
      useCollectionView("page", COLUMNS),
    );
    expect(reopened.current.view).toBe("table");
    expect([...reopened.current.hiddenColumns]).toEqual(["class"]);
  });

  it("keeps the choice apart per page and per account", () => {
    const { result } = renderHook(() => useCollectionView("page", COLUMNS));
    act(() => result.current.setView("table"));

    const { result: otherPage } = renderHook(() =>
      useCollectionView("other-page", COLUMNS),
    );
    expect(otherPage.current.view).toBe("tiles");

    sessionUser.current = { id: "8", tenantId: 3 };
    const { result: otherAccount } = renderHook(() =>
      useCollectionView("page", COLUMNS),
    );
    expect(otherAccount.current.view).toBe("tiles");
  });

  it("falls back to the defaults for a damaged stored value", () => {
    localStorage.setItem(
      "collection-view:page:tenant-3:7",
      '{"view":"grid","columns":{"class":"no"}}',
    );
    const { result } = renderHook(() => useCollectionView("page", COLUMNS));

    expect(result.current.view).toBe("tiles");
    expect([...result.current.hiddenColumns]).toEqual(["notes"]);
  });
});
