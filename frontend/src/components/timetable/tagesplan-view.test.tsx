import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { berlinTodayISO } from "~/lib/date-helpers";
import { useSWRAuth } from "~/lib/swr/hooks";
import {
  useOperationalOverviewScope,
  useTimetableEnabled,
  useWeekendFollowsFriday,
} from "~/lib/tenant-context";
import { useTenantRouter } from "~/lib/tenant-router";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

import { TagesplanView } from "./tagesplan-view";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

// Starten ist eine Aktion ohne Formular: Fehler gehen als Toast über
// useApiErrorDisplay (#2516). Ladefehler laufen über den echten Hook.
const showActionError = vi.hoisted(() => vi.fn());
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useApiErrorDisplay: () => ({ show: showActionError }),
}));

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
  useWeekendFollowsFriday: vi.fn(() => false),
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
    vi.mocked(useWeekendFollowsFriday).mockReturnValue(false);
    vi.mocked(useTenantRouter).mockReturnValue({ push, replace } as never);
    setSWR({ data: [], isLoading: false, error: null });
  });

  it("steps a day forward and back, skipping the weekend", () => {
    searchParams.current = new URLSearchParams("d=2026-07-15");
    render(<TagesplanView />);
    fireEvent.click(screen.getByRole("button", { name: "Nächster Tag" }));
    expect(replace).toHaveBeenLastCalledWith("/tagesplan?d=2026-07-16");

    searchParams.current = new URLSearchParams("d=2026-07-17");
    render(<TagesplanView />);
    fireEvent.click(
      screen.getAllByRole("button", { name: "Nächster Tag" })[1]!,
    );
    expect(replace).toHaveBeenLastCalledWith("/tagesplan?d=2026-07-20");
  });

  it("steps onto Saturday when the weekend follows Friday's plan (#3921)", () => {
    vi.mocked(useWeekendFollowsFriday).mockReturnValue(true);
    searchParams.current = new URLSearchParams("d=2026-07-17");
    render(<TagesplanView />);
    fireEvent.click(screen.getByRole("button", { name: "Nächster Tag" }));
    expect(replace).toHaveBeenLastCalledWith("/tagesplan?d=2026-07-18");
    fireEvent.click(screen.getByRole("button", { name: "Vorheriger Tag" }));
    expect(replace).toHaveBeenLastCalledWith("/tagesplan?d=2026-07-16");
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

  // #3921: Flos Abend-Rundgang zeigte „Kreativraum · 41 von 0 da“ an einem
  // spontanen Block, der um 20:32 noch „bis 16:16“ lief.
  it("counts a spontaneous block by who is there and shows no invented end", () => {
    setSWR({
      data: [
        makeInstance({
          id: "4",
          title: "Basteln",
          roomName: "Kreativraum",
          status: "active",
          activeGroupId: "92",
          isSpontaneous: true,
          startTime: "15:16",
          endTime: "16:16",
          expectedStudentsCount: 0,
          presentStudentsCount: 41,
          currentStudentsCount: 13,
          plannedStudentsCount: 0,
        }),
      ],
      isLoading: false,
      error: null,
    });

    render(<TagesplanView />);

    expect(
      screen.getByText(/Kreativraum · 13 da · 28 gegangen/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/von 0 da/)).not.toBeInTheDocument();
    expect(screen.getByText("Ende offen")).toBeInTheDocument();
    expect(screen.queryByText("bis 16:16")).not.toBeInTheDocument();
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

  it("shows a duty without children, start or Nicht gestartet (#3822)", () => {
    setSWR({
      data: [
        makeInstance({
          id: "7",
          title: "Busaufsicht",
          startTime: "08:00",
          endTime: "08:30",
          roomId: "0",
          roomName: null,
          isDuty: true,
          expectedStudentsCount: 0,
        }),
        makeInstance({
          id: "8",
          title: "Essensausgabe",
          startTime: "10:00",
          endTime: "11:00",
          roomName: "Mensa",
          isDuty: true,
          expectedStudentsCount: 0,
          canStart: true,
          startExpiresAt: "2099-01-01T00:00:00Z",
        }),
      ],
      isLoading: false,
      error: null,
    });

    render(<TagesplanView />);

    expect(screen.getByText("Dienst")).toBeInTheDocument();
    expect(screen.getByText("Dienst · Mensa")).toBeInTheDocument();
    expect(screen.queryByText(/Kinder/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Nicht gestartet/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Raum 0/)).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Starten" }),
    ).not.toBeInTheDocument();
  });

  it("shows a distinct empty state for a day without blocks", () => {
    setSWR({ data: [], isLoading: false, error: null });

    render(<TagesplanView />);

    expect(
      screen.getByText("Heute ist keine Betreuung geplant"),
    ).toBeInTheDocument();
  });

  it("shows a retryable error state when the list fails to load", async () => {
    setSWR({
      data: undefined,
      isLoading: false,
      error: new ApiError("Failed to fetch", 503, {
        code: "general.unavailable",
      }),
    });

    render(<TagesplanView />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Termine"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Failed to fetch/)).not.toBeInTheDocument();
    expect(
      screen.queryByText("Heute ist keine Betreuung geplant"),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(reloadList).toHaveBeenCalled();
  });

  it("explains missing permission with the catalog text (403)", async () => {
    const { TimetableOperationsApiError } =
      await import("~/lib/timetable-operations-api");
    setSWR({
      data: undefined,
      isLoading: false,
      error: new TimetableOperationsApiError("forbidden", 403),
    });

    render(<TagesplanView />);

    expect(
      await screen.findByText(
        catalogText("general.permission", "die Liste der Termine"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("forbidden")).not.toBeInTheDocument();
  });

  it("reports a failed start as a toast and retries the same block", async () => {
    const error = new ApiError("too early", 409, {
      code: "timetable.start_too_early",
    });
    vi.mocked(timetableOperationsApi.start).mockRejectedValueOnce(error);
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

    await waitFor(() =>
      expect(showActionError).toHaveBeenCalledWith(error, {
        object: "das Starten des Termins",
        retry: expect.any(Function),
      }),
    );
    expect(push).not.toHaveBeenCalled();
    expect(reloadList).toHaveBeenCalled();

    vi.mocked(timetableOperationsApi.start).mockResolvedValueOnce({
      activeGroupId: "77",
    } as never);
    const { retry } = showActionError.mock.calls[0]![1] as {
      retry: () => void;
    };
    retry();
    await waitFor(() =>
      expect(push).toHaveBeenCalledWith("/active-supervisions?session=77"),
    );
    expect(timetableOperationsApi.start).toHaveBeenLastCalledWith("4");
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
