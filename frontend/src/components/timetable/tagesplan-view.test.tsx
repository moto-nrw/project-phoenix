import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { berlinTodayISO } from "~/lib/date-helpers";
import { useSWRAuth } from "~/lib/swr/hooks";
import {
  useOperationalOverviewScope,
  useTimetableEnabled,
} from "~/lib/tenant-context";
import { useTenantRouter } from "~/lib/tenant-router";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

import { TagesplanView } from "./tagesplan-view";

// useSWRAuth is NOT globally mocked (only the raw `swr` package is) — mock the
// exact subpath the component imports so the day fetch is driven per test.
vi.mock("~/lib/swr/hooks", () => ({
  useSWRAuth: vi.fn(),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: vi.fn(),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTimetableEnabled: vi.fn(() => true),
  useOperationalOverviewScope: vi.fn(() => "all_staff"),
}));

// Feste Uhr: 10:00 Berliner Zeit, damit die "Jetzt"-Linie deterministisch ist.
vi.mock("~/lib/pickup-helpers", () => ({
  useMinuteClock: vi.fn(() => new Date("2026-08-31T08:00:00Z")),
}));

vi.mock("~/lib/timetable-operations-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/timetable-operations-api")>();
  return {
    ...actual,
    timetableOperationsApi: {
      ...actual.timetableOperationsApi,
      plannedNow: vi.fn(),
      start: vi.fn(),
    },
  };
});

const searchParams = { current: new URLSearchParams() };
vi.mock("next/navigation", () => ({
  useSearchParams: () => searchParams.current,
}));

function makeInstance(
  overrides: Partial<PlannedTimetableInstance> & { id: string },
): PlannedTimetableInstance {
  return {
    title: "Lernzeit",
    date: berlinTodayISO(),
    startTime: "09:00",
    endTime: "09:45",
    roomId: "5",
    roomName: "Lernraum",
    status: "planned",
    isOverdue: false,
    minutesUntilStart: 60,
    expectedStudentsCount: 8,
    presentStudentsCount: 0,
    notScheduledStudentsCount: 0,
    assignedStaffIds: [],
    isAssigned: false,
    isPrimary: false,
    isSubstitute: false,
    isAbsent: false,
    rosterPreview: [],
    canStart: false,
    startAvailableAt: "",
    startExpiresAt: "",
    activeGroupId: null,
    cancelReason: null,
    planningTrackName: null,
    planningTrackColor: null,
    groupName: null,
    staffNames: [],
    ...overrides,
  };
}

type SWRState = { data?: unknown; isLoading: boolean; error: unknown };

const reloadList = vi.fn();

function setSWR(state: SWRState) {
  vi.mocked(useSWRAuth).mockImplementation(
    () => ({ ...state, mutate: reloadList }) as never,
  );
}

const push = vi.fn();
const replace = vi.fn();

describe("TagesplanView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchParams.current = new URLSearchParams();
    vi.mocked(useTimetableEnabled).mockReturnValue(true);
    vi.mocked(useOperationalOverviewScope).mockReturnValue("all_staff");
    vi.mocked(useTenantRouter).mockReturnValue({ push, replace } as never);
    setSWR({ data: [], isLoading: false, error: null });
  });

  it("renders the day's blocks in time order with room, Zielgruppe and staff", () => {
    setSWR({
      data: [
        makeInstance({
          id: "2",
          title: "Fußball-AG",
          startTime: "14:00",
          endTime: "15:00",
          roomName: "Turnhalle",
          groupName: "Gruppe Sonne",
          staffNames: [
            { staffId: "9", displayName: "Maria Muster", isSubstitute: false },
            {
              staffId: "10",
              displayName: "Vera Vertretung",
              isSubstitute: true,
            },
          ],
        }),
        makeInstance({
          id: "1",
          title: "Mittagessen",
          startTime: "12:00",
          endTime: "13:00",
          roomName: "Mensa",
        }),
      ],
      isLoading: false,
      error: null,
    });

    render(<TagesplanView />);

    const titles = screen
      .getAllByText(/Mittagessen|Fußball-AG/)
      .map((el) => el.textContent);
    expect(titles[0]).toContain("Mittagessen");
    expect(titles[1]).toContain("Fußball-AG");
    expect(screen.getByText(/Turnhalle · Gruppe Sonne/)).toBeInTheDocument();
    expect(
      screen.getByText("Maria Muster, Vera Vertretung (Vertretung)"),
    ).toBeInTheDocument();
  });

  it("opens the live list of a running block on tap", () => {
    setSWR({
      data: [
        makeInstance({
          id: "3",
          title: "Freispiel",
          status: "active",
          activeGroupId: "91",
          presentStudentsCount: 5,
        }),
      ],
      isLoading: false,
      error: null,
    });

    render(<TagesplanView />);

    expect(screen.getAllByText("Läuft").length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole("button", { name: /Freispiel/ }));
    expect(push).toHaveBeenCalledWith("/active-supervisions?session=91");
  });

  it("starts an own planned block via the existing start flow and jumps to its list", async () => {
    vi.mocked(timetableOperationsApi.start).mockResolvedValue({
      instanceId: "4",
      activeGroupId: "77",
      status: "active",
    });
    setSWR({
      data: [
        makeInstance({
          id: "4",
          canStart: true,
          startExpiresAt: "2099-01-01T00:00:00Z",
          isAssigned: true,
        }),
      ],
      isLoading: false,
      error: null,
    });

    render(<TagesplanView />);

    fireEvent.click(screen.getByRole("button", { name: "Starten" }));
    await waitFor(() => {
      expect(timetableOperationsApi.start).toHaveBeenCalledWith("4");
      expect(push).toHaveBeenCalledWith("/active-supervisions?session=77");
    });
  });

  it("renders cancelled and completed blocks as plain display without actions", () => {
    setSWR({
      data: [
        makeInstance({
          id: "5",
          title: "Bastel-AG",
          status: "cancelled",
          cancelReason: "Personalausfall",
        }),
        makeInstance({ id: "6", title: "Hausaufgaben", status: "completed" }),
      ],
      isLoading: false,
      error: null,
    });

    render(<TagesplanView />);

    expect(screen.getByText("Fällt aus · Personalausfall")).toBeInTheDocument();
    expect(screen.getByText(/Beendet/)).toBeInTheDocument();
    // Keine Zeile ist antippbar, kein Start-Knopf vorhanden.
    expect(
      screen.queryByRole("button", { name: /Bastel-AG|Hausaufgaben|Starten/ }),
    ).not.toBeInTheDocument();
  });

  it("shows a distinct empty state for a day without blocks", () => {
    setSWR({ data: [], isLoading: false, error: null });

    render(<TagesplanView />);

    expect(
      screen.getByText("Heute ist keine Betreuung geplant"),
    ).toBeInTheDocument();
  });

  it("shows a retryable error state when the list fails to load", () => {
    setSWR({ data: undefined, isLoading: false, error: new Error("boom") });

    render(<TagesplanView />);

    expect(
      screen.getByText("Der Tagesplan konnte nicht geladen werden."),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Noch einmal versuchen" }),
    );
    expect(reloadList).toHaveBeenCalled();
  });

  it("explains missing permission instead of a bare error (403)", async () => {
    const { TimetableOperationsApiError } =
      await import("~/lib/timetable-operations-api");
    setSWR({
      data: undefined,
      isLoading: false,
      error: new TimetableOperationsApiError("forbidden", 403),
    });

    render(<TagesplanView />);

    expect(
      screen.getByText("Kein Zugriff auf den Betreuungsplan"),
    ).toBeInTheDocument();
  });

  it("keeps the previous start path visible when the Betreuungsplan is disabled", () => {
    vi.mocked(useTimetableEnabled).mockReturnValue(false);

    render(<TagesplanView />);

    expect(screen.getByTestId("tagesplan-disabled")).toBeInTheDocument();
  });

  it("shows the own-scope hint when the school keeps staff on their own blocks (#2380)", () => {
    vi.mocked(useOperationalOverviewScope).mockReturnValue("own");
    setSWR({ data: [], isLoading: false, error: null });

    render(<TagesplanView />);

    expect(
      screen.getByText(/Sie sehen nur Termine, für die Sie eingeteilt sind/),
    ).toBeInTheDocument();
  });

  it("honours the ?d= day parameter for back navigation to a chosen day", () => {
    searchParams.current = new URLSearchParams("d=2026-01-05");
    const keys: unknown[] = [];
    vi.mocked(useSWRAuth).mockImplementation(((key: unknown) => {
      keys.push(key);
      return {
        data: [],
        isLoading: false,
        error: null,
        mutate: reloadList,
      } as never;
    }) as never);

    render(<TagesplanView />);

    expect(keys).toContain("tagesplan-2026-01-05");
    expect(
      screen.getByRole("heading", { name: /05\.01\.2026/ }),
    ).toBeInTheDocument();
  });
});
