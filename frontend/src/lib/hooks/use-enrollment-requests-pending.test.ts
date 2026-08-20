import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import type { UseUnreadCountOptions } from "./use-unread-count";

const {
  mockUseSession,
  mockUseShellAuth,
  mockCanReview,
  mockUseUnreadCount,
  mockFetchPendingEnrollmentChangeRequestCount,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUseShellAuth: vi.fn(),
  mockCanReview: vi.fn(),
  mockUseUnreadCount: vi.fn(),
  mockFetchPendingEnrollmentChangeRequestCount: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: (): ReturnType<typeof mockUseSession> => mockUseSession(),
}));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: (): ReturnType<typeof mockUseShellAuth> => mockUseShellAuth(),
}));

vi.mock("~/lib/change-request-access", () => ({
  canReviewEnrollmentChangeRequests: (...args: unknown[]): boolean =>
    mockCanReview(...args) as boolean,
}));

vi.mock("~/lib/change-request-list-api", () => ({
  fetchPendingEnrollmentChangeRequestCount:
    mockFetchPendingEnrollmentChangeRequestCount,
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: (): string | null => mockTenantSlug,
}));

vi.mock("./use-unread-count", () => ({
  useUnreadCount: (opts: UseUnreadCountOptions): unknown =>
    mockUseUnreadCount(opts) as unknown,
}));

let mockTenantSlug: string | null = "testschool";

import { useEnrollmentRequestsPending } from "./use-enrollment-requests-pending";

/** Rendert den Hook und gibt die an useUnreadCount übergebenen Optionen zurück. */
function capturedOptions(): UseUnreadCountOptions {
  renderHook(() => useEnrollmentRequestsPending());
  expect(mockUseUnreadCount).toHaveBeenCalledTimes(1);
  return mockUseUnreadCount.mock.calls[0]![0] as UseUnreadCountOptions;
}

beforeEach(() => {
  vi.clearAllMocks();
  mockTenantSlug = "testschool";
  mockUseUnreadCount.mockReturnValue({ unreadCount: 0, isLoading: false });
  mockCanReview.mockReturnValue(true);
  mockUseShellAuth.mockReturnValue({ mode: "teacher" });
  mockUseSession.mockReturnValue({
    data: { user: { id: "7" } },
    status: "authenticated",
  });
});

describe("useEnrollmentRequestsPending", () => {
  it("zählt für eine Person mit config:manage im Lehrkraft-Bereich", () => {
    const opts = capturedOptions();
    expect(opts.enabled).toBe(true);
    expect(mockCanReview).toHaveBeenCalledWith({ user: { id: "7" } });
  });

  it("zählt nicht, wer Anmeldungsänderungen nicht entscheiden darf", () => {
    mockCanReview.mockReturnValue(false);
    expect(capturedOptions().enabled).toBe(false);
  });

  it("zählt nicht außerhalb des Lehrkraft-Bereichs", () => {
    mockUseShellAuth.mockReturnValue({ mode: "operator" });
    expect(capturedOptions().enabled).toBe(false);
  });

  it("zählt nicht, solange die Sitzung nicht angemeldet ist", () => {
    mockUseSession.mockReturnValue({ data: null, status: "loading" });
    expect(capturedOptions().enabled).toBe(false);
  });

  it("verdrahtet den Zähl-Abruf und das Auffrisch-Ereignis der Warteschlange", () => {
    const opts = capturedOptions();
    expect(opts.fetcher).toBe(mockFetchPendingEnrollmentChangeRequestCount);
    expect(opts.eventNames).toEqual(["change-requests-refresh"]);
    expect(opts.eventDebounceMs).toBe(500);
    expect(opts.refetchOnFocus).toBe(true);
  });

  it("bindet den Cache-Schlüssel an Schule und Konto", () => {
    expect(capturedOptions().cacheKey).toBe(
      "enrollment_requests_pending_count:testschool:7",
    );
  });

  it("kommt ohne Schul-Slug und ohne Konto-ID aus", () => {
    mockTenantSlug = null;
    mockUseSession.mockReturnValue({ data: {}, status: "authenticated" });
    expect(capturedOptions().cacheKey).toBe(
      "enrollment_requests_pending_count::",
    );
  });
});
