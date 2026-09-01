import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockUseSession, mockUseShellAuth, mockUseSWRAuth } = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUseShellAuth: vi.fn(),
  mockUseSWRAuth: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => mockUseSession(),
}));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => mockUseShellAuth(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (...args: unknown[]) => mockUseSWRAuth(...args),
}));

import { useChangeRequestAccess } from "./use-change-request-access";

describe("useChangeRequestAccess", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSession.mockReturnValue({
      data: {
        user: {
          id: "7",
          token: "token",
          roles: ["user"],
          permissions: ["users:read", "users:update"],
        },
      },
      status: "authenticated",
    });
    mockUseShellAuth.mockReturnValue({ mode: "teacher" });
    mockUseSWRAuth.mockReturnValue({
      data: undefined,
      error: undefined,
      isLoading: true,
      mutate: vi.fn(),
    });
  });

  it("bleibt während der effektiven Zugriffsauswertung geschlossen", () => {
    const { result } = renderHook(() => useChangeRequestAccess());

    expect(result.current.isLoading).toBe(true);
    expect(result.current.canOpenRequestsPage).toBe(false);
    expect(result.current.canReviewParentRequests).toBe(false);
  });

  it("bleibt während die Sitzung geladen wird im Ladezustand", () => {
    mockUseSession.mockReturnValue({
      data: undefined,
      status: "loading",
    });

    const { result } = renderHook(() => useChangeRequestAccess());

    expect(result.current.isLoading).toBe(true);
    expect(result.current.canOpenRequestsPage).toBe(false);
  });

  it("trennt die Zugriffsauswertung nach Konto", () => {
    const { rerender } = renderHook(() => useChangeRequestAccess());
    const firstAccountKey = mockUseSWRAuth.mock.calls.at(-1)?.[0];

    mockUseSession.mockReturnValue({
      data: {
        user: {
          id: "8",
          token: "other-token",
          roles: ["user"],
          permissions: ["users:read", "users:update"],
        },
      },
      status: "authenticated",
    });
    rerender();

    expect(firstAccountKey).toBe("change-request-access:7");
    expect(mockUseSWRAuth.mock.calls.at(-1)?.[0]).toBe(
      "change-request-access:8",
    );
    expect(mockUseSWRAuth.mock.calls.at(-1)?.[2]).toMatchObject({
      keepPreviousData: false,
    });
  });

  it("öffnet Elternanfragen für eine aktuelle Gruppenleitung", () => {
    mockUseSWRAuth.mockReturnValue({
      data: "group_leader",
      error: undefined,
      isLoading: false,
      mutate: vi.fn(),
    });

    const { result } = renderHook(() => useChangeRequestAccess());

    expect(result.current.canReviewParentRequests).toBe(true);
    expect(result.current.canOpenRequestsPage).toBe(true);
  });

  it("bleibt bei einem Fehler in der effektiven Zugriffsauswertung geschlossen", () => {
    mockUseSWRAuth.mockReturnValue({
      data: undefined,
      error: new Error("unavailable"),
      isLoading: false,
      mutate: vi.fn(),
    });

    const { result } = renderHook(() => useChangeRequestAccess());

    expect(result.current.isLoading).toBe(false);
    expect(result.current.canReviewParentRequests).toBe(false);
    expect(result.current.canOpenRequestsPage).toBe(false);
  });

  it("öffnet unabhängige Anfragearten ohne Elternanfragen-Abfrage", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          id: "7",
          token: "token",
          roles: ["user"],
          permissions: ["vacation:approve"],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useChangeRequestAccess());

    expect(mockUseSWRAuth.mock.calls[0]?.[0]).toBeNull();
    expect(result.current.canReviewParentRequests).toBe(false);
    expect(result.current.canReviewStaffAbsenceRequests).toBe(true);
    expect(result.current.canOpenRequestsPage).toBe(true);
  });

  it("gewährt Admins sofort den vollständigen Zugriff", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          id: "1",
          token: "token",
          roles: ["admin"],
          permissions: ["admin:*"],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useChangeRequestAccess());

    expect(mockUseSWRAuth.mock.calls[0]?.[0]).toBeNull();
    expect(result.current.canReviewParentRequests).toBe(true);
    expect(result.current.canOpenRequestsPage).toBe(true);
  });
});
