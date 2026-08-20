import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

const { mockUseSession, mockUseSWRAuth, mockListPending } = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUseSWRAuth: vi.fn(),
  mockListPending: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: (): ReturnType<typeof mockUseSession> => mockUseSession(),
}));

vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: {
    listPending: (): ReturnType<typeof mockListPending> => mockListPending(),
  },
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null, fetcher: unknown, options: unknown) =>
    mockUseSWRAuth(key, fetcher, options) as unknown,
}));

import { useStaffPendingAbsences } from "./use-staff-pending-absences";

function sessionWith(permissions: readonly string[]) {
  return { data: { user: { id: "7", roles: ["user"], permissions } } };
}

describe("useStaffPendingAbsences", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSWRAuth.mockReturnValue({ data: undefined });
  });

  it("lädt die offenen Anträge mit vacation:approve", () => {
    mockUseSession.mockReturnValue(sessionWith(["vacation:approve"]));
    const rows = [{ id: 1, staff_id: 3 }];
    mockUseSWRAuth.mockReturnValue({ data: rows });

    const { result } = renderHook(() => useStaffPendingAbsences());

    expect(result.current.canReview).toBe(true);
    expect(result.current.rows).toBe(rows);
    // Der Schlüssel trägt das gemeinsame Präfix, über das die
    // Entscheidungs-Ansichten alle Zähler auffrischen.
    expect(mockUseSWRAuth.mock.calls[0]![0]).toContain(
      "staff-pending-absences-",
    );
  });

  it("fragt ohne Freigaberecht gar nicht erst ab", () => {
    mockUseSession.mockReturnValue(sessionWith(["users:read"]));

    const { result } = renderHook(() => useStaffPendingAbsences());

    expect(result.current.canReview).toBe(false);
    expect(result.current.rows).toEqual([]);
    // key === null hält SWR davon ab, den Endpunkt überhaupt aufzurufen.
    expect(mockUseSWRAuth.mock.calls[0]![0]).toBeNull();
  });

  it("lässt Admins ohne ausdrückliches Recht abfragen", () => {
    mockUseSession.mockReturnValue({
      data: { user: { id: "1", roles: ["admin"], permissions: ["admin:*"] } },
    });

    const { result } = renderHook(() => useStaffPendingAbsences());

    expect(result.current.canReview).toBe(true);
    expect(mockUseSWRAuth.mock.calls[0]![0]).not.toBeNull();
  });

  it("reicht den Fetcher an den Abwesenheits-Endpunkt durch", async () => {
    mockUseSession.mockReturnValue(sessionWith(["vacation:approve"]));
    mockListPending.mockResolvedValue([]);

    renderHook(() => useStaffPendingAbsences());
    const fetcher = mockUseSWRAuth.mock.calls[0]![1] as () => Promise<unknown>;
    await fetcher();

    expect(mockListPending).toHaveBeenCalled();
  });
});
