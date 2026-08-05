import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockSearch } = vi.hoisted(() => ({ mockSearch: { value: "" } }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(mockSearch.value),
}));

import { useOneTimeQueryFlag } from "./use-one-time-query-flag";

function setLocation(path: string) {
  window.history.replaceState(null, "", path);
}

describe("useOneTimeQueryFlag", () => {
  beforeEach(() => {
    mockSearch.value = "";
    setLocation("/enroll/status/tok");
  });

  it("returns false when the flag is absent and leaves the URL alone", () => {
    mockSearch.value = "duplicate_warning=1";
    setLocation("/enroll/status/tok?duplicate_warning=1");

    const { result } = renderHook(() => useOneTimeQueryFlag("submitted"));

    expect(result.current).toBe(false);
    expect(window.location.search).toBe("?duplicate_warning=1");
  });

  it("returns true and strips the flag from the URL", () => {
    mockSearch.value = "submitted=1";
    setLocation("/enroll/status/tok?submitted=1");

    const { result } = renderHook(() => useOneTimeQueryFlag("submitted"));

    expect(result.current).toBe(true);
    expect(window.location.search).toBe("");
    expect(window.location.pathname).toBe("/enroll/status/tok");
  });

  it("keeps other query params and the hash when stripping", () => {
    mockSearch.value = "submitted=1&duplicate_warning=1";
    setLocation("/enroll/status/tok?submitted=1&duplicate_warning=1#top");

    const { result } = renderHook(() => useOneTimeQueryFlag("submitted"));

    expect(result.current).toBe(true);
    expect(window.location.search).toBe("?duplicate_warning=1");
    expect(window.location.hash).toBe("#top");
  });

  it("stays true after the URL rewrite for the lifetime of the component", () => {
    mockSearch.value = "submitted=1";
    setLocation("/enroll/status/tok?submitted=1");

    const { result, rerender } = renderHook(() =>
      useOneTimeQueryFlag("submitted"),
    );
    // Simulate the router syncing useSearchParams after replaceState.
    mockSearch.value = "";
    rerender();

    expect(result.current).toBe(true);
  });

  it("is false on a fresh mount once the flag is gone from the URL", () => {
    mockSearch.value = "";
    setLocation("/enroll/status/tok");

    const { result } = renderHook(() => useOneTimeQueryFlag("submitted"));

    expect(result.current).toBe(false);
  });
});
