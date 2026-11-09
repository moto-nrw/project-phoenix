import { render, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  SettingsReturnPathMemory,
  useSettingsReturnHref,
} from "./settings-return-path";

const nav = vi.hoisted(() => ({ pathname: "/", search: "" }));

vi.mock("next/navigation", () => ({
  usePathname: () => nav.pathname,
  useSearchParams: () => new URLSearchParams(nav.search),
}));

const KEY = "test-settings-return-path";
const isSettingsPath = (pathname: string) => pathname === "/settings";

describe("settings return path", () => {
  beforeEach(() => {
    sessionStorage.clear();
    nav.pathname = "/";
    nav.search = "";
  });

  it("remembers the last app page with its query", () => {
    nav.pathname = "/students/search";
    nav.search = "group=5";
    render(
      <SettingsReturnPathMemory
        storageKey={KEY}
        isSettingsPath={isSettingsPath}
      />,
    );

    const { result } = renderHook(() => useSettingsReturnHref(KEY, "/start"));
    expect(result.current).toBe("/students/search?group=5");
  });

  it("does not remember the settings themselves", () => {
    sessionStorage.setItem(KEY, "/rooms");
    nav.pathname = "/settings";
    render(
      <SettingsReturnPathMemory
        storageKey={KEY}
        isSettingsPath={isSettingsPath}
      />,
    );

    expect(sessionStorage.getItem(KEY)).toBe("/rooms");
  });

  it.each(["https://example.test", "//example.test", "/\\example.test"])(
    "rejects the foreign address %s",
    (stored) => {
      sessionStorage.setItem(KEY, stored);
      const { result } = renderHook(() => useSettingsReturnHref(KEY, "/start"));
      expect(result.current).toBe("/start");
    },
  );

  it("starts at the start page without a remembered page", () => {
    const { result } = renderHook(() => useSettingsReturnHref(KEY, "/start"));
    expect(result.current).toBe("/start");
  });
});
