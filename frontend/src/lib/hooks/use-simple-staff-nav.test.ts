import { act, renderHook } from "@testing-library/react";
import type { Session } from "next-auth";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const tenant = vi.hoisted(() => ({ simpleNavigation: true }));

vi.mock("~/lib/tenant-context", () => ({
  useSimpleStaffNavigationSetting: () => tenant.simpleNavigation,
  useTenantSlugSafe: () => "burbach",
}));

import { useSimpleStaffNav } from "~/lib/hooks/use-simple-staff-nav";

function staffSession(
  id: string,
  permissions: string[] = ["users:read"],
): Session {
  return {
    expires: "2099-01-01T00:00:00.000Z",
    user: { id, roles: ["user"], permissions },
  } as unknown as Session;
}

describe("useSimpleStaffNav (#6956)", () => {
  beforeEach(() => {
    tenant.simpleNavigation = true;
    globalThis.localStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows the simple navigation to staff who do not lead the school", () => {
    const { result } = renderHook(() => useSimpleStaffNav(staffSession("7")));
    expect(result.current.available).toBe(true);
    expect(result.current.active).toBe(true);
  });

  it("keeps the full navigation while the school has not switched it on", () => {
    tenant.simpleNavigation = false;
    const { result } = renderHook(() => useSimpleStaffNav(staffSession("7")));
    expect(result.current.available).toBe(false);
    expect(result.current.active).toBe(false);
  });

  it("keeps the full navigation for a person who leads the school", () => {
    const { result } = renderHook(() =>
      useSimpleStaffNav(staffSession("7", ["config:manage"])),
    );
    expect(result.current.available).toBe(false);
    expect(result.current.active).toBe(false);
  });

  it("keeps the full navigation without a session", () => {
    const { result } = renderHook(() => useSimpleStaffNav(null));
    expect(result.current.active).toBe(false);
  });

  it("remembers per account that the person wants every page", () => {
    const first = renderHook(() => useSimpleStaffNav(staffSession("7")));
    act(() => first.result.current.showAllPages());
    expect(first.result.current.active).toBe(false);
    expect(first.result.current.available).toBe(true);

    // Eine zweite Fläche derselben Seite (Seitenleiste und mobile Leiste)
    // folgt sofort, auch ohne Neuladen.
    const second = renderHook(() => useSimpleStaffNav(staffSession("7")));
    expect(second.result.current.active).toBe(false);

    // Eine andere Person am selben Gerät behält die einfache Ansicht.
    const colleague = renderHook(() => useSimpleStaffNav(staffSession("8")));
    expect(colleague.result.current.active).toBe(true);

    act(() => second.result.current.showSimplePages());
    expect(first.result.current.active).toBe(true);
    expect(second.result.current.active).toBe(true);
  });

  it.each([
    { failure: "full storage", stored: false, blockReads: false },
    { failure: "failed removal", stored: true, blockReads: false },
    { failure: "blocked storage", stored: false, blockReads: true },
  ])(
    "shares the choice despite $failure",
    ({ failure, stored, blockReads }) => {
      const accountId = `storage-${failure}`;
      const key = `simple-staff-nav-show-all:burbach:${accountId}`;
      if (stored) globalThis.localStorage.setItem(key, "1");
      const throwStorageError = () => {
        throw new Error("Storage unavailable");
      };
      vi.spyOn(globalThis.localStorage, "setItem").mockImplementation(
        throwStorageError,
      );
      vi.spyOn(globalThis.localStorage, "removeItem").mockImplementation(
        throwStorageError,
      );
      if (blockReads) {
        vi.spyOn(globalThis.localStorage, "getItem").mockImplementation(
          throwStorageError,
        );
      }

      const first = renderHook(() =>
        useSimpleStaffNav(staffSession(accountId)),
      );
      const second = renderHook(() =>
        useSimpleStaffNav(staffSession(accountId)),
      );
      expect(first.result.current.active).toBe(!stored);

      act(() => first.result.current.toggle());
      expect(first.result.current.active).toBe(stored);
      expect(second.result.current.active).toBe(stored);

      // A newly mounted menu also sees the unpersisted choice.
      const remounted = renderHook(() =>
        useSimpleStaffNav(staffSession(accountId)),
      );
      expect(remounted.result.current.active).toBe(stored);
      const colleague = renderHook(() =>
        useSimpleStaffNav(staffSession("other")),
      );
      expect(colleague.result.current.active).toBe(true);

      act(() => second.result.current.toggle());
      expect(first.result.current.active).toBe(!stored);
      expect(second.result.current.active).toBe(!stored);
      expect(remounted.result.current.active).toBe(!stored);

      // Once storage recovers, successful writes clear the fallback so changes
      // from another tab can be read again.
      vi.restoreAllMocks();
      act(() => first.result.current.showSimplePages());
      act(() => {
        globalThis.localStorage.setItem(key, "1");
        globalThis.dispatchEvent(new StorageEvent("storage", { key }));
      });
      expect(first.result.current.active).toBe(false);
      expect(second.result.current.active).toBe(false);
    },
  );
});
