import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CareScheduleManager } from "./care-schedule-manager";
import type { ArrivalData } from "~/lib/student-arrival-api";
import type { PickupData } from "~/lib/pickup-schedule-helpers";
import type { StudentStatusDay } from "~/lib/student-status-days-api";

const mockFetchArrivalData = vi.fn();
const mockFetchStudentPickupData = vi.fn();
const mockUpdateArrivalSchedules = vi.fn();
const mockUpdateStudentPickupSchedules = vi.fn();
const mockCreateArrivalException = vi.fn();
const mockUpdateArrivalException = vi.fn();
const mockDeleteArrivalException = vi.fn();
const mockCreateArrivalNote = vi.fn();
const mockDeleteArrivalNote = vi.fn();
const mockCreateStudentPickupException = vi.fn();
const mockUpdateStudentPickupException = vi.fn();
const mockDeleteStudentPickupException = vi.fn();
const mockCreateStudentPickupNote = vi.fn();
const mockDeleteStudentPickupNote = vi.fn();
const FROZEN_NOW = new Date("2026-05-27T12:00:00");

vi.mock("~/lib/student-arrival-api", () => ({
  fetchArrivalData: (...args: unknown[]) => mockFetchArrivalData(...args),
  updateArrivalSchedules: (...args: unknown[]) =>
    mockUpdateArrivalSchedules(...args),
  createArrivalException: (...args: unknown[]) =>
    mockCreateArrivalException(...args),
  updateArrivalException: (...args: unknown[]) =>
    mockUpdateArrivalException(...args),
  deleteArrivalException: (...args: unknown[]) =>
    mockDeleteArrivalException(...args),
  createArrivalNote: (...args: unknown[]) => mockCreateArrivalNote(...args),
  deleteArrivalNote: (...args: unknown[]) => mockDeleteArrivalNote(...args),
}));

vi.mock("~/lib/pickup-schedule-api", () => ({
  fetchStudentPickupData: (...args: unknown[]) =>
    mockFetchStudentPickupData(...args),
  updateStudentPickupSchedules: (...args: unknown[]) =>
    mockUpdateStudentPickupSchedules(...args),
  createStudentPickupException: (...args: unknown[]) =>
    mockCreateStudentPickupException(...args),
  updateStudentPickupException: (...args: unknown[]) =>
    mockUpdateStudentPickupException(...args),
  deleteStudentPickupException: (...args: unknown[]) =>
    mockDeleteStudentPickupException(...args),
  createStudentPickupNote: (...args: unknown[]) =>
    mockCreateStudentPickupNote(...args),
  deleteStudentPickupNote: (...args: unknown[]) =>
    mockDeleteStudentPickupNote(...args),
}));

// The editor is exercised in its own test file; here it is reduced to the two
// callbacks the manager wires up, so these tests stay about the manager's
// persistence and refresh behaviour.
// The editor is exercised in its own test file; here it is reduced to the two
// callbacks the manager wires up, so these tests stay about the manager's
// persistence and refresh behaviour.
vi.mock("./care-plan-editor-modal", () => ({
  CarePlanEditorModal: ({
    isOpen,
    date,
    onClose,
    onSubmitWeekly,
    onSubmitException,
  }: {
    isOpen: boolean;
    date: Date | null;
    onClose: () => void;
    onSubmitWeekly: (data: {
      arrivalSchedules: Array<{
        weekday: number;
        expected_arrival: string;
        notes?: string | null;
      }>;
      pickupSchedules: Array<{
        weekday: number;
        pickupTime: string;
        notes?: string;
      }>;
    }) => Promise<void>;
    onSubmitException: (payload: {
      date: string;
      arrival:
        | { kind: "regular" }
        | { kind: "time"; time: string; reason: string | null }
        | { kind: "none"; reason: string | null }
        | null;
      pickup:
        | { kind: "regular" }
        | { kind: "time"; time: string; reason: string | null }
        | { kind: "none"; reason: string | null }
        | null;
    }) => Promise<void>;
  }) =>
    isOpen ? (
      <div
        data-testid={date ? "care-plan-editor-day" : "care-plan-editor-week"}
      >
        <button
          type="button"
          onClick={() =>
            void onSubmitWeekly({
              arrivalSchedules: [
                { weekday: 1, expected_arrival: "08:30", notes: "Tor" },
              ],
              pickupSchedules: [
                { weekday: 1, pickupTime: "15:30", notes: "Bus" },
              ],
            })
          }
        >
          Wochenplan im Test speichern
        </button>
        <button
          type="button"
          onClick={() =>
            void onSubmitException({
              date: "2026-05-25",
              arrival: { kind: "time", time: "10:10", reason: "Arzt" },
              pickup: { kind: "time", time: "15:15", reason: "Training" },
            }).catch(() => undefined)
          }
        >
          Ausnahme im Test speichern
        </button>
        <button
          type="button"
          onClick={() =>
            void onSubmitException({
              date: "2026-05-25",
              arrival: { kind: "regular" },
              pickup: { kind: "regular" },
            })
          }
        >
          Auf Regulär zurücksetzen
        </button>
        <button type="button" onClick={onClose}>
          Schließen
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    isOpen,
    title,
    children,
    onClose,
    onConfirm,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    onClose: () => void;
    onConfirm: () => void;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        {children}
        <button type="button" onClick={onConfirm}>
          Entfernen
        </button>
        <button type="button" onClick={onClose}>
          Abbrechen
        </button>
      </div>
    ) : null,
}));

const arrivalData: ArrivalData = {
  schedules: [
    {
      id: 1,
      student_id: 42,
      weekday: 1,
      weekday_name: "Montag",
      expected_arrival: "08:00",
      notes: "Tor",
      created_by: 1,
      created_at: "2026-05-01T00:00:00Z",
      updated_at: "2026-05-01T00:00:00Z",
    },
  ],
  exceptions: [
    {
      id: 10,
      student_id: 42,
      exception_date: "2026-05-25",
      expected_arrival: "10:10",
      reason: "testnotizhg",
      created_by: 1,
      created_at: "2026-05-01T00:00:00Z",
      updated_at: "2026-05-01T00:00:00Z",
    },
  ],
  notes: [
    {
      id: 100,
      student_id: 42,
      note_date: "2026-05-25",
      content: "Kommt später",
      created_by: 1,
      created_at: "2026-05-01T00:00:00Z",
      updated_at: "2026-05-01T00:00:00Z",
    },
  ],
};

const pickupData: PickupData = {
  schedules: [
    {
      id: "1",
      studentId: "42",
      weekday: 1,
      weekdayName: "Montag",
      pickupTime: "15:00",
      notes: "Bus",
      createdBy: "1",
      createdAt: "2026-05-01T00:00:00Z",
      updatedAt: "2026-05-01T00:00:00Z",
    },
  ],
  exceptions: [
    {
      id: "20",
      studentId: "42",
      exceptionDate: "2026-05-25",
      pickupTime: "15:15",
      reason: "arzt",
      createdBy: "1",
      createdAt: "2026-05-01T00:00:00Z",
      updatedAt: "2026-05-01T00:00:00Z",
    },
  ],
  notes: [
    {
      id: "200",
      studentId: "42",
      noteDate: "2026-05-25",
      content: "Traffic",
      createdBy: "1",
      createdAt: "2026-05-01T00:00:00Z",
      updatedAt: "2026-05-01T00:00:00Z",
    },
  ],
};

const statusDays: StudentStatusDay[] = [
  {
    id: "7",
    student_id: "42",
    date: "2026-05-26",
    status: "excused",
    label: "Entschuldigt",
    reported_at: "2026-05-25T08:00:00Z",
    cleared_at: null,
    source: "planned",
    created_at: "2026-05-25T08:00:00Z",
    updated_at: "2026-05-25T08:00:00Z",
  },
  {
    id: "8",
    student_id: "42",
    date: "2026-05-29",
    status: "sick",
    label: "Krank",
    reported_at: "2026-05-25T08:00:00Z",
    cleared_at: null,
    source: "planned",
    created_at: "2026-05-25T08:00:00Z",
    updated_at: "2026-05-25T08:00:00Z",
  },
  {
    id: "9",
    student_id: "42",
    date: "2026-05-27",
    status: "class_trip",
    label: "Klassenfahrt",
    reported_at: "2026-05-25T08:00:00Z",
    cleared_at: null,
    source: "planned",
    created_at: "2026-05-25T08:00:00Z",
    updated_at: "2026-05-25T08:00:00Z",
  },
];

describe("CareScheduleManager", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime(FROZEN_NOW);
    vi.clearAllMocks();
    mockFetchArrivalData.mockResolvedValue(arrivalData);
    mockFetchStudentPickupData.mockResolvedValue(pickupData);
    mockUpdateArrivalSchedules.mockResolvedValue([]);
    mockUpdateStudentPickupSchedules.mockResolvedValue([]);
    mockCreateArrivalException.mockResolvedValue({});
    mockUpdateArrivalException.mockResolvedValue({});
    mockDeleteArrivalException.mockResolvedValue(undefined);
    mockCreateArrivalNote.mockResolvedValue({});
    mockDeleteArrivalNote.mockResolvedValue(undefined);
    mockCreateStudentPickupException.mockResolvedValue({});
    mockUpdateStudentPickupException.mockResolvedValue({});
    mockDeleteStudentPickupException.mockResolvedValue(undefined);
    mockCreateStudentPickupNote.mockResolvedValue({});
    mockDeleteStudentPickupNote.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("loads and displays the weekly care schedule", async () => {
    render(
      <CareScheduleManager studentId="42" statusDays={statusDays} readOnly />,
    );

    expect(document.querySelector(".animate-spin")).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("Betreuungsplan")).toBeInTheDocument();
    });

    expect(mockFetchArrivalData).toHaveBeenCalledWith("42");
    expect(mockFetchStudentPickupData).toHaveBeenCalledWith("42");
    expect(screen.getAllByText("10:10").length).toBeGreaterThan(0);
    expect(screen.getAllByText("15:15").length).toBeGreaterThan(0);
    expect(
      screen.getAllByText("Ganztägig entschuldigt").length,
    ).toBeGreaterThan(0);
    expect(
      screen.getAllByText("Ganztägig Klassenfahrt").length,
    ).toBeGreaterThan(0);
    expect(
      screen.getAllByText("Ganztägig krank gemeldet").length,
    ).toBeGreaterThan(0);
    expect(screen.getAllByText("Ankunft:").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Abholung:").length).toBeGreaterThan(0);
  });

  it("re-fetches arrival and pickup on a remote care-schedule-stale event", async () => {
    // The editor holds arrival/pickup in local state and stays force-mounted, so
    // it cannot see SWR invalidation; the global SSE hook announces remote
    // pickup/arrival changes on this window event, which it must react to.
    render(
      <CareScheduleManager studentId="42" statusDays={statusDays} readOnly />,
    );
    await screen.findByText("Betreuungsplan");

    expect(mockFetchArrivalData).toHaveBeenCalledTimes(1);
    expect(mockFetchStudentPickupData).toHaveBeenCalledTimes(1);

    await act(async () => {
      window.dispatchEvent(new CustomEvent("phoenix:care-schedule-stale"));
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(mockFetchArrivalData).toHaveBeenCalledTimes(2);
      expect(mockFetchStudentPickupData).toHaveBeenCalledTimes(2);
    });
  });

  it("defers a remote refresh while the weekly-plan editor is open, then applies it on close", async () => {
    // The modal seeds its rows from arrivalData/pickupData and re-seeds whenever
    // those change identity, so refreshing mid-edit would silently discard the
    // user's typing. Someone else's edit must not cost this user their work —
    // but the update must not be lost either.
    render(<CareScheduleManager studentId="42" statusDays={statusDays} />);
    await screen.findByText("Betreuungsplan");
    expect(mockFetchArrivalData).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByTitle("Wochenplan bearbeiten"));
    expect(screen.getByTestId("care-plan-editor-week")).toBeInTheDocument();

    await act(async () => {
      window.dispatchEvent(new CustomEvent("phoenix:care-schedule-stale"));
      await Promise.resolve();
    });

    // Draft preserved: no refetch happened while the modal was open.
    expect(mockFetchArrivalData).toHaveBeenCalledTimes(1);
    expect(mockFetchStudentPickupData).toHaveBeenCalledTimes(1);

    await act(async () => {
      fireEvent.click(screen.getByText("Schließen"));
      await Promise.resolve();
    });

    // Deferred, not dropped — the update lands once the editor is closed.
    await waitFor(() => {
      expect(mockFetchArrivalData).toHaveBeenCalledTimes(2);
      expect(mockFetchStudentPickupData).toHaveBeenCalledTimes(2);
    });
  });

  it("ignores a superseded in-flight refresh so it cannot overwrite newer data", async () => {
    // Two refreshes in flight at once (two SSE announcements, or one racing the
    // refetch after a local save) can resolve in either order. The older one
    // must not land last and revert the newer schedule state.
    const stalePickup: PickupData = {
      ...pickupData,
      exceptions: [
        {
          ...pickupData.exceptions[0]!,
          pickupTime: "09:09",
          reason: "veraltet",
        },
      ],
    };
    const freshPickup: PickupData = {
      ...pickupData,
      exceptions: [
        {
          ...pickupData.exceptions[0]!,
          pickupTime: "17:17",
          reason: "aktuell",
        },
      ],
    };

    render(
      <CareScheduleManager studentId="42" statusDays={statusDays} readOnly />,
    );
    await screen.findByText("Betreuungsplan");

    // Hold the FIRST (older) response open; let the second resolve immediately.
    let releaseStale: (value: PickupData) => void = () => undefined;
    mockFetchStudentPickupData.mockImplementationOnce(
      () =>
        new Promise<PickupData>((resolve) => {
          releaseStale = resolve;
        }),
    );
    mockFetchStudentPickupData.mockImplementationOnce(() =>
      Promise.resolve(freshPickup),
    );

    await act(async () => {
      window.dispatchEvent(new CustomEvent("phoenix:care-schedule-stale"));
      await Promise.resolve();
    });
    await act(async () => {
      window.dispatchEvent(new CustomEvent("phoenix:care-schedule-stale"));
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(screen.getAllByText("17:17").length).toBeGreaterThan(0);
    });

    // Now let the stale request finish last — it must be discarded.
    await act(async () => {
      releaseStale(stalePickup);
      await Promise.resolve();
    });

    expect(screen.queryByText("09:09")).not.toBeInTheDocument();
    expect(screen.getAllByText("17:17").length).toBeGreaterThan(0);
  });

  it("keeps the initial result when a newer refresh fails instead of blanking the editor", async () => {
    // The initial load and an SSE refresh overlap; the refresh loses. The
    // initial result must still be applied — dropping it because a newer fetch
    // merely EXISTED left the editor with no data, no error and nothing to
    // retry, since a background refresh reports failures to the log only.
    let resolveInitial: (value: PickupData) => void = () => undefined;
    mockFetchStudentPickupData.mockImplementationOnce(
      () =>
        new Promise<PickupData>((resolve) => {
          resolveInitial = resolve;
        }),
    );
    mockFetchStudentPickupData.mockImplementationOnce(() =>
      Promise.reject(new Error("Netzwerkfehler")),
    );

    render(
      <CareScheduleManager studentId="42" statusDays={statusDays} readOnly />,
    );

    // The refresh starts (and fails) while the initial load is still pending.
    await act(async () => {
      window.dispatchEvent(new CustomEvent("phoenix:care-schedule-stale"));
      await Promise.resolve();
    });

    // Now the initial load succeeds — last one to resolve, but the only data
    // anyone has.
    await act(async () => {
      resolveInitial(pickupData);
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(screen.getByText("Betreuungsplan")).toBeInTheDocument();
    });
    expect(screen.getAllByText("15:15").length).toBeGreaterThan(0);
    expect(document.querySelector(".animate-spin")).not.toBeInTheDocument();
  });

  it("does not raise an error banner from a request a newer refresh already overtook", async () => {
    // The initial load hangs, a remote refresh overtakes it and renders fresh
    // data, and only then does the initial load fail. Its error belongs to state
    // that is no longer on screen, so surfacing it would put a failure banner
    // over perfectly good data.
    const freshPickup: PickupData = {
      ...pickupData,
      exceptions: [
        {
          ...pickupData.exceptions[0]!,
          pickupTime: "17:17",
          reason: "aktuell",
        },
      ],
    };

    let failInitial: (reason: Error) => void = () => undefined;
    mockFetchStudentPickupData.mockImplementationOnce(
      () =>
        new Promise<PickupData>((_resolve, reject) => {
          failInitial = reject;
        }),
    );
    mockFetchStudentPickupData.mockImplementationOnce(() =>
      Promise.resolve(freshPickup),
    );

    render(
      <CareScheduleManager studentId="42" statusDays={statusDays} readOnly />,
    );

    // The remote refresh overtakes the still-pending initial load.
    await act(async () => {
      window.dispatchEvent(new CustomEvent("phoenix:care-schedule-stale"));
      await Promise.resolve();
    });
    await waitFor(() => {
      expect(screen.getAllByText("17:17").length).toBeGreaterThan(0);
    });

    // Now let the superseded initial load reject.
    await act(async () => {
      failInitial(new Error("Netzwerkfehler"));
      await Promise.resolve();
    });

    expect(screen.queryByText("Netzwerkfehler")).not.toBeInTheDocument();
    expect(screen.getAllByText("17:17").length).toBeGreaterThan(0);
    // And the spinner is gone — a superseded failure must not strand the view.
    expect(document.querySelector(".animate-spin")).not.toBeInTheDocument();
  });

  it("reports the newly visible week range when navigating", async () => {
    const onVisibleDateRangeChange = vi.fn();

    render(
      <CareScheduleManager
        studentId="42"
        statusDays={statusDays}
        onVisibleDateRangeChange={onVisibleDateRangeChange}
      />,
    );
    await screen.findByText("Betreuungsplan");
    fireEvent.click(screen.getAllByLabelText("Nächste Woche")[0]!);

    expect(onVisibleDateRangeChange).toHaveBeenCalledWith(
      "2026-06-01",
      "2026-06-05",
    );
  });

  it("updates the weekly plan from the edit action", async () => {
    const onUpdate = vi.fn();

    render(
      <CareScheduleManager
        studentId="42"
        statusDays={statusDays}
        onUpdate={onUpdate}
      />,
    );
    await screen.findByText("Betreuungsplan");

    fireEvent.click(screen.getByTitle("Wochenplan bearbeiten"));
    fireEvent.click(screen.getByText("Wochenplan im Test speichern"));

    await waitFor(() => {
      expect(mockUpdateArrivalSchedules).toHaveBeenCalledWith("42", [
        { weekday: 1, expected_arrival: "08:30", notes: "Tor" },
      ]);
      expect(mockUpdateStudentPickupSchedules).toHaveBeenCalledWith("42", {
        schedules: [{ weekday: 1, pickupTime: "15:30", notes: "Bus" }],
      });
      expect(onUpdate).toHaveBeenCalled();
    });
  });

  it("opens the exception editor and persists the day change", async () => {
    render(<CareScheduleManager studentId="42" statusDays={statusDays} />);
    await screen.findByText("Betreuungsplan");

    fireEvent.click(screen.getAllByText("Ausnahme")[0]!);
    expect(screen.getByTestId("care-plan-editor-day")).toBeInTheDocument();
    fireEvent.click(screen.getByText("Ausnahme im Test speichern"));

    await waitFor(() => {
      expect(mockUpdateArrivalException).toHaveBeenCalledWith("42", 10, {
        exception_date: "2026-05-25",
        expected_arrival: "10:10",
        reason: "Arzt",
      });
      expect(mockUpdateStudentPickupException).toHaveBeenCalledWith(
        "42",
        "20",
        {
          exceptionDate: "2026-05-25",
          pickupTime: "15:15",
          reason: "Training",
        },
      );
    });
  });

  it("refreshes after a partially saved day exception", async () => {
    mockUpdateStudentPickupException.mockRejectedValueOnce(
      new Error("Abholung konnte nicht gespeichert werden"),
    );

    render(<CareScheduleManager studentId="42" statusDays={statusDays} />);
    await screen.findByText("Betreuungsplan");

    fireEvent.click(screen.getAllByText("Ausnahme")[0]!);
    fireEvent.click(screen.getByText("Ausnahme im Test speichern"));

    await waitFor(() => {
      expect(mockUpdateArrivalException).toHaveBeenCalled();
      expect(mockUpdateStudentPickupException).toHaveBeenCalled();
      expect(mockFetchArrivalData).toHaveBeenCalledTimes(2);
      expect(mockFetchStudentPickupData).toHaveBeenCalledTimes(2);
    });
  });

  it("removes the existing exceptions when the day is set back to regular", async () => {
    render(<CareScheduleManager studentId="42" statusDays={statusDays} />);
    await screen.findByText("Betreuungsplan");

    fireEvent.click(screen.getAllByText("Ausnahme")[0]!);
    fireEvent.click(screen.getByText("Auf Regulär zurücksetzen"));

    await waitFor(() => {
      expect(mockDeleteArrivalException).toHaveBeenCalledWith("42", 10);
      expect(mockDeleteStudentPickupException).toHaveBeenCalledWith("42", "20");
    });
    expect(mockCreateArrivalException).not.toHaveBeenCalled();
    expect(mockCreateStudentPickupException).not.toHaveBeenCalled();
  });

  it("opens confirmation before deleting a planned status day", async () => {
    const onDeleteStatusDay = vi.fn().mockResolvedValue(undefined);

    render(
      <CareScheduleManager
        studentId="42"
        statusDays={statusDays}
        onDeleteStatusDay={onDeleteStatusDay}
      />,
    );
    await screen.findByText("Betreuungsplan");

    fireEvent.click(
      screen.getAllByLabelText("Ganztägig entschuldigt entfernen")[0]!,
    );

    expect(
      screen.getByRole("dialog", { name: "Geplanten Status entfernen?" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Dienstag, 26.05.2026 entfernt/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Entfernen" }));

    await waitFor(() => {
      expect(onDeleteStatusDay).toHaveBeenCalledWith("7");
    });
  });

  it("shows a load error", async () => {
    mockFetchArrivalData.mockRejectedValueOnce(new Error("Netzwerkfehler"));

    render(<CareScheduleManager studentId="42" />);

    await waitFor(() => {
      expect(screen.getByText("Netzwerkfehler")).toBeInTheDocument();
    });
  });
});
