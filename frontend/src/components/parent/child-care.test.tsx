import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  PickupTimeModal,
  resolveTodayPickup,
  SickNoteModal,
  SickStatusSummary,
  useChildCare,
} from "./child-care";
import type {
  CareException,
  ExcusedRequest,
  StatusDay,
} from "~/lib/parent-api";
import * as parentApi from "~/lib/parent-api";
import {
  berlinTodayISO,
  parseISODate,
  todayISO,
  toISODate,
} from "~/lib/date-helpers";

// The factory is hoisted above the imports, so the mock helper has to be
// pulled in inside it rather than at the top of the file.
vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

// de-locale (the test locale) renders YYYY-MM-DD as DD.MM.YYYY.
function de(iso: string): string {
  const [y, m, d] = iso.split("-");
  return `${d}.${m}.${y}`;
}

function makeRequest(over: Partial<ExcusedRequest>): ExcusedRequest {
  return {
    id: "1",
    student_id: "1",
    status: "pending",
    dates: [],
    note: "Termin",
    created_at: "2026-03-01T09:00:00Z",
    is_self: false,
    ...over,
  };
}

function makeStatusDay(over: Partial<StatusDay>): StatusDay {
  return {
    id: "1",
    student_id: "1",
    date: todayISO(),
    status: "sick",
    reported_at: "2026-03-01T09:00:00Z",
    source: "parent",
    ...over,
  };
}

// Regression coverage for the failed-preload save path: when the care-exception
// list never loaded (careExceptionsLoaded=false), the modal must not let a
// parent save. submitCareException treats an omitted leg as an authoritative
// clear, so saving from an unknown state could silently wipe an existing
// override the UI never managed to prefill.

const LOAD_ERROR_DE =
  "Die aktuell hinterlegten Zeiten konnten nicht geladen werden. Bitte laden Sie die Seite neu und versuchen Sie es erneut.";

function renderModal(
  overrides: Partial<React.ComponentProps<typeof PickupTimeModal>> = {},
) {
  const onSubmit = vi.fn().mockResolvedValue(undefined);
  const onRemove = vi.fn().mockResolvedValue(undefined);
  const onClose = vi.fn();
  render(
    <PickupTimeModal
      careExceptions={[]}
      careExceptionsLoaded
      pickupChangeEnabled
      onClose={onClose}
      onSubmit={onSubmit}
      onRemove={onRemove}
      {...overrides}
    />,
  );
  // The shared Modal renders via createPortal to document.body, so the fields
  // live outside the render container — query the document instead.
  const dateInput =
    document.querySelector<HTMLInputElement>('input[type="date"]')!;
  const pickupInput =
    document.querySelector<HTMLInputElement>('input[type="time"]');
  const reasonInput = document.querySelector<HTMLTextAreaElement>("textarea");
  return { onSubmit, onRemove, onClose, dateInput, pickupInput, reasonInput };
}

describe("PickupTimeModal — failed preload guard", () => {
  it("blocks saving and warns when the exception list failed to load", () => {
    const { onSubmit } = renderModal({ careExceptionsLoaded: false });

    // The load-failure notice is shown.
    expect(screen.getByText(LOAD_ERROR_DE)).toBeInTheDocument();

    // The save button is disabled so an omitted leg can't be sent as a clear.
    const saveButton = screen.getByRole("button", { name: "Speichern" });
    expect(saveButton).toBeDisabled();

    // Even if a click slips through, handleSubmit refuses to submit.
    fireEvent.click(saveButton);
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("allows saving once the exception list is known", () => {
    const { onSubmit, pickupInput, reasonInput } = renderModal({
      careExceptionsLoaded: true,
    });

    // No load-failure notice when the list loaded.
    expect(screen.queryByText(LOAD_ERROR_DE)).not.toBeInTheDocument();

    fireEvent.change(pickupInput!, { target: { value: "14:30" } });
    fireEvent.change(reasonInput!, { target: { value: "Arzttermin" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        pickupTime: "14:30",
        reason: "Arzttermin",
      }),
    );
  });

  it("preserves an existing arrival leg the parent did not touch", () => {
    // A loaded override with both legs: the parent edits pickup only. Because
    // the list loaded, the arrival field is prefilled and travels back intact
    // rather than being cleared.
    const existing: CareException = {
      date: "2026-03-17",
      pickup_time: "15:00",
      arrival_time: "08:30",
      reason: "Termin",
      source: "guardian",
      updated_at: "2026-03-10T09:00:00Z",
    };
    const { onSubmit, dateInput, pickupInput } = renderModal({
      careExceptions: [existing],
      careExceptionsLoaded: true,
    });

    // Point the picker at the override's date so the fields prefill from it.
    fireEvent.change(dateInput, { target: { value: "2026-03-17" } });

    // Change only the pickup time; arrival stays at its prefilled value.
    fireEvent.change(pickupInput!, { target: { value: "16:00" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(onSubmit).toHaveBeenCalledWith({
      date: "2026-03-17",
      pickupTime: "16:00",
      arrivalTime: "08:30",
      reason: "Termin",
    });
  });

  it("requires a reason for a changed pickup time", () => {
    const { onSubmit, pickupInput, reasonInput } = renderModal();

    fireEvent.change(pickupInput!, { target: { value: "14:30" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      screen.getByText(
        "Bitte geben Sie einen kurzen Grund für die Änderung an.",
      ),
    ).toHaveAttribute("role", "alert");
    expect(reasonInput).toHaveAttribute("aria-invalid", "true");
    expect(reasonInput).toHaveFocus();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});

// Issue #1735: the former "Krank melden" modal became a generic "Abmelden" modal
// with a Krank/Entschuldigt choice. These pin that the chosen kind reaches the
// submit handler as the status argument — the heart of the feature.
describe("SickNoteModal — Abmeldegrund", () => {
  it("defaults to a Krankmeldung (status sick)", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    render(<SickNoteModal onClose={onClose} onSubmit={onSubmit} />);

    fireEvent.click(screen.getByRole("button", { name: "Abmeldung senden" }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    const [dates, reason, status] = onSubmit.mock.calls[0]!;
    expect(dates).toHaveLength(1); // from/to default to today
    expect(reason).toBe("");
    expect(status).toBe("sick");
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("submits an excused absence with a note when Entschuldigt is chosen", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<SickNoteModal onClose={vi.fn()} onSubmit={onSubmit} />);

    // Switch the kind to "Entschuldigt" via the CustomSelect combobox.
    fireEvent.click(
      screen.getByRole("combobox", { name: "Art der Abmeldung" }),
    );
    fireEvent.click(screen.getByRole("option", { name: "Entschuldigt" }));

    const reasonField = document.querySelector("textarea")!;
    fireEvent.change(reasonField, { target: { value: "Zahnarzttermin" } });

    fireEvent.click(screen.getByRole("button", { name: "Abmeldung senden" }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.any(Array),
      "Zahnarzttermin",
      "excused",
    );
  });

  it("blocks submission and surfaces an error for an invalid date range", () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<SickNoteModal onClose={vi.fn()} onSubmit={onSubmit} />);

    // Force "Bis" before "Von" so the enumerated date set is empty.
    const dateInputs = Array.from(
      document.querySelectorAll<HTMLInputElement>('input[type="date"]'),
    );
    const [, toInput] = dateInputs;
    fireEvent.change(toInput!, { target: { value: "2000-01-01" } });

    fireEvent.click(screen.getByRole("button", { name: "Abmeldung senden" }));

    expect(onSubmit).not.toHaveBeenCalled();
  });
});

// #1845 review: the parent summary must not misrepresent request dates.
describe("SickStatusSummary — date rendering", () => {
  it("lists non-contiguous dates instead of a false continuous range", () => {
    render(
      <SickStatusSummary
        sickDays={[]}
        excusedRequests={[
          makeRequest({ id: "10", dates: ["2026-03-16", "2026-03-18"] }),
        ]}
      />,
    );
    // Mon + Wed must render as "16.03.2026, 18.03.2026", never "16.03.2026 – 18.03.2026"
    // (which would wrongly imply Tuesday is included too).
    expect(screen.getByText(/16\.03\.2026, 18\.03\.2026/)).toBeInTheDocument();
    expect(screen.queryByText(/16\.03\.2026 – 18\.03\.2026/)).toBeNull();
  });

  it("keeps a contiguous range collapsed", () => {
    render(
      <SickStatusSummary
        sickDays={[]}
        excusedRequests={[
          makeRequest({ id: "11", dates: ["2026-03-16", "2026-03-17"] }),
        ]}
      />,
    );
    expect(screen.getByText(/16\.03\.2026 – 17\.03\.2026/)).toBeInTheDocument();
  });

  it("shows an out-of-window approved date but not one superseded by a newer status", () => {
    const today = todayISO();
    const past = (() => {
      const d = parseISODate(today);
      d.setDate(d.getDate() - 30);
      return toISODate(d);
    })();

    render(
      <SickStatusSummary
        // Authoritative current status for today: sick (a newer decision that
        // replaced the earlier excused approval for the same day).
        sickDays={[makeStatusDay({ date: today, status: "sick" })]}
        excusedRequests={[
          makeRequest({
            id: "12",
            status: "approved",
            dates: [past, today],
          }),
        ]}
      />,
    );

    // The past date is outside the fetched window (today..+2mo) and has no
    // status day, so it must surface from the approved request.
    const excusedLines = screen.getAllByText(/^Entschuldigt:/);
    expect(excusedLines).toHaveLength(1);
    expect(excusedLines[0]!.textContent).toContain(de(past));
    // today must NOT appear as excused — it is authoritatively sick now, so
    // showing it as both sick AND excused would be wrong.
    expect(excusedLines[0]!.textContent).not.toContain(de(today));
    // today is shown as sick instead.
    expect(screen.getByText(/^Krank:/).textContent).toContain(de(today));
  });
});

describe("resolveTodayPickup", () => {
  const TODAY = todayISO();
  // Today's ISO weekday (Mon=1 .. Sun=7) so the base-plan cases work whatever
  // day the suite runs on.
  const TODAY_WD = ((parseISODate(TODAY).getDay() + 6) % 7) + 1;

  function makeException(over: Partial<CareException>): CareException {
    return {
      date: TODAY,
      source: "guardian",
      updated_at: "2026-03-01T09:00:00Z",
      ...over,
    };
  }

  it("returns the base-plan pickup time for today (not marked changed)", () => {
    expect(
      resolveTodayPickup({
        weekdays: [{ weekday: TODAY_WD, pickup: "16:00", modes: [] }],
        weekPlanLoaded: true,
        todayAbsent: false,
        careExceptions: [],
        careExceptionsLoaded: true,
        today: TODAY,
      }),
    ).toEqual({ kind: "time", time: "16:00", changed: false });
  });

  it("prefers a same-day override and marks it changed", () => {
    expect(
      resolveTodayPickup({
        weekdays: [{ weekday: TODAY_WD, pickup: "16:00", modes: [] }],
        weekPlanLoaded: true,
        todayAbsent: false,
        careExceptions: [makeException({ pickup_time: "15:00" })],
        careExceptionsLoaded: true,
        today: TODAY,
      }),
    ).toEqual({ kind: "time", time: "15:00", changed: true });
  });

  it("does NOT mark an override changed when it equals the base plan", () => {
    // The backend permits an exception equal to the normal pickup time; such a
    // day has no effective change, so it must not render as "geändert" (#1725
    // review).
    expect(
      resolveTodayPickup({
        weekdays: [{ weekday: TODAY_WD, pickup: "16:00", modes: [] }],
        weekPlanLoaded: true,
        todayAbsent: false,
        careExceptions: [makeException({ pickup_time: "16:00" })],
        careExceptionsLoaded: true,
        today: TODAY,
      }),
    ).toEqual({ kind: "time", time: "16:00", changed: false });
  });

  it("falls back to the base plan when an override changes only arrival", () => {
    expect(
      resolveTodayPickup({
        weekdays: [{ weekday: TODAY_WD, pickup: "16:00", modes: [] }],
        weekPlanLoaded: true,
        todayAbsent: false,
        careExceptions: [makeException({ arrival_time: "08:00" })],
        careExceptionsLoaded: true,
        today: TODAY,
      }),
    ).toEqual({ kind: "time", time: "16:00", changed: false });
  });

  it("reports an absence when the child is off today, over any configured time", () => {
    expect(
      resolveTodayPickup({
        weekdays: [{ weekday: TODAY_WD, pickup: "16:00", modes: [] }],
        weekPlanLoaded: true,
        todayAbsent: true,
        careExceptions: [makeException({ pickup_time: "15:00" })],
        careExceptionsLoaded: true,
        today: TODAY,
      }),
    ).toEqual({ kind: "absent" });
  });

  it("resolves a staff 'not coming today' pickup exception as absent", () => {
    // A staff pickup row with no time (pickup_absent) is an absence marker that
    // creates no status day, so today_absent is false. The tile must still show
    // an absence, not fall back to the base-plan pickup (#1725 review).
    expect(
      resolveTodayPickup({
        weekdays: [{ weekday: TODAY_WD, pickup: "16:00", modes: [] }],
        weekPlanLoaded: true,
        todayAbsent: false,
        careExceptions: [
          makeException({ source: "staff", pickup_absent: true }),
        ],
        careExceptionsLoaded: true,
        today: TODAY,
      }),
    ).toEqual({ kind: "absent" });
  });

  it("resolves a staff arrival-only absence as absent over a regular pickup", () => {
    // An arrival row with no time (arrival_absent) is a "not coming today" marker
    // that also creates no status day. Even with a regular base-plan pickup, the
    // tile must resolve to an absence, not tell the guardian to expect a pickup
    // for a child who is not coming (#1725 review).
    expect(
      resolveTodayPickup({
        weekdays: [{ weekday: TODAY_WD, pickup: "16:00", modes: [] }],
        weekPlanLoaded: true,
        todayAbsent: false,
        careExceptions: [
          makeException({ source: "staff", arrival_absent: true }),
        ],
        careExceptionsLoaded: true,
        today: TODAY,
      }),
    ).toEqual({ kind: "absent" });
  });

  it("returns 'none' on a care day with no pickup configured", () => {
    expect(
      resolveTodayPickup({
        weekdays: [],
        weekPlanLoaded: true,
        todayAbsent: false,
        careExceptions: [],
        careExceptionsLoaded: true,
        today: TODAY,
      }),
    ).toEqual({ kind: "none" });
  });

  it("returns 'unknown' when the plan failed to load and there is no override", () => {
    expect(
      resolveTodayPickup({
        weekdays: [],
        weekPlanLoaded: false,
        todayAbsent: false,
        careExceptions: [],
        careExceptionsLoaded: true,
        today: TODAY,
      }),
    ).toEqual({ kind: "unknown" });
  });

  it("returns 'unknown' when the override list failed to load", () => {
    // A failed care-exception fetch is an empty array, indistinguishable from
    // "no override" — we must not fall through to a base-plan time that a real,
    // unloaded override could contradict (#1725 review).
    expect(
      resolveTodayPickup({
        weekdays: [{ weekday: TODAY_WD, pickup: "16:00", modes: [] }],
        weekPlanLoaded: true,
        todayAbsent: false,
        careExceptions: [],
        careExceptionsLoaded: false,
        today: TODAY,
      }),
    ).toEqual({ kind: "unknown" });
  });

  it("shows a same-day override when the plan failed to load, but cannot call it changed", () => {
    // The override list loaded (with the override); the base plan did not. The
    // override is still authoritative, but with no base to compare against we
    // must not claim a difference (#1725 review).
    expect(
      resolveTodayPickup({
        weekdays: [],
        weekPlanLoaded: false,
        todayAbsent: false,
        careExceptions: [makeException({ pickup_time: "15:00" })],
        careExceptionsLoaded: true,
        today: TODAY,
      }),
    ).toEqual({ kind: "time", time: "15:00", changed: false });
  });

  it("never marks an override changed against a stale plan while weekPlanLoaded is false", () => {
    // A failed or midnight-stale care-schedule refetch keeps the PREVIOUS
    // weekdays array live (setWeekdays only runs on success) while
    // weekPlanLoaded flips false. The override must still render, but comparing
    // it against that stale base plan must NOT flag "changed" — we can't verify a
    // difference we don't trust (#1725 review). `weekdays: []` doesn't exercise
    // this; the base entry has to be present-but-untrusted.
    expect(
      resolveTodayPickup({
        weekdays: [{ weekday: TODAY_WD, pickup: "16:00", modes: [] }],
        weekPlanLoaded: false,
        todayAbsent: false,
        careExceptions: [makeException({ pickup_time: "15:00" })],
        careExceptionsLoaded: true,
        today: TODAY,
      }),
    ).toEqual({ kind: "time", time: "15:00", changed: false });
  });
});

describe("useChildCare reportSick", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  // A direct sick/excused report that includes today makes the child absent NOW.
  // todayPickup reads todayAbsent exclusively, so the hook must flip it on the
  // optimistic merge path — otherwise the "Heute → Abholung" tile keeps showing
  // the normal pickup time next to the just-reported absence until an unrelated
  // reload (#1725 review).
  it("marks today absent after a same-day report without reloading", async () => {
    const today = berlinTodayISO();
    const todayWd = ((parseISODate(today).getDay() + 6) % 7) + 1;

    vi.spyOn(parentApi, "listSickDays").mockResolvedValue([]);
    vi.spyOn(parentApi, "listExcusedRequests").mockResolvedValue([]);
    vi.spyOn(parentApi, "listCareExceptions").mockResolvedValue([]);
    vi.spyOn(parentApi, "getChildCareSchedule").mockResolvedValue({
      weekdays: [{ weekday: todayWd, pickup: "16:00", modes: [] }],
      can_request: false,
      request_capabilities: {
        arrival: false,
        pickup: false,
        departure_mode: false,
      },
      today_absent: false,
    });
    // Reject features so the hook falls back to DEFAULT_FEATURES — avoids
    // constructing the full flag object and is irrelevant to this path.
    vi.spyOn(parentApi, "getChildFeatures").mockRejectedValue(
      new Error("no features"),
    );
    const submitSickNote = vi
      .spyOn(parentApi, "submitSickNote")
      .mockResolvedValue({
        status_days: [
          {
            id: "1",
            student_id: "1",
            date: today,
            status: "sick",
            reported_at: "2026-03-01T09:00:00Z",
            source: "parent",
          },
        ],
      });

    const { result } = renderHook(() => useChildCare("1"));

    // Baseline: the tile resolves the base-plan pickup before any report.
    await waitFor(() =>
      expect(result.current.todayPickup).toEqual({
        kind: "time",
        time: "16:00",
        changed: false,
      }),
    );

    await act(async () => {
      await result.current.reportSick([today], "", "sick");
    });

    // No reload was needed (the direct-write path returns status days inline);
    // the tile now shows an absence purely from the optimistic state update.
    expect(submitSickNote).toHaveBeenCalledTimes(1);
    expect(parentApi.getChildCareSchedule).toHaveBeenCalledTimes(1);
    expect(result.current.todayPickup).toEqual({ kind: "absent" });
  });
});

describe("useChildCare studentId switch", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  // Navigating from child A to child B reuses the hook instance (only the prop
  // changes). The loadSeqRef guard stops a late load(A) from overwriting B, but
  // it does nothing about the window before B's fetch lands — so A's resolved
  // pickup must not linger under B's name. The hook resets per-child state
  // synchronously on the studentId change, dropping the loaded flags so the tile
  // reverts to the neutral "unknown" state instead of showing A's time (#1725).
  it("drops the previous child's pickup synchronously when studentId changes", async () => {
    const today = berlinTodayISO();
    const todayWd = ((parseISODate(today).getDay() + 6) % 7) + 1;

    vi.spyOn(parentApi, "listSickDays").mockResolvedValue([]);
    vi.spyOn(parentApi, "listExcusedRequests").mockResolvedValue([]);
    vi.spyOn(parentApi, "listCareExceptions").mockImplementation((id: string) =>
      // Child A resolves; child B hangs so we observe the pre-fetch window.
      id === "1"
        ? Promise.resolve([] as CareException[])
        : new Promise<CareException[]>(() => {
            /* never resolves */
          }),
    );
    vi.spyOn(parentApi, "getChildCareSchedule").mockImplementation(
      (id: string) =>
        id === "1"
          ? Promise.resolve({
              weekdays: [{ weekday: todayWd, pickup: "16:00", modes: [] }],
              can_request: false,
              request_capabilities: {
                arrival: false,
                pickup: false,
                departure_mode: false,
              },
              today_absent: false,
            })
          : new Promise(() => {
              /* never resolves */
            }),
    );
    vi.spyOn(parentApi, "getChildFeatures").mockRejectedValue(
      new Error("no features"),
    );

    const { result, rerender } = renderHook(({ id }) => useChildCare(id), {
      initialProps: { id: "1" },
    });

    // Child A's base-plan pickup is resolved.
    await waitFor(() =>
      expect(result.current.todayPickup).toEqual({
        kind: "time",
        time: "16:00",
        changed: false,
      }),
    );

    // Switch to child B, whose fetches never resolve. The tile must NOT still
    // show A's 16:00 — the reset drops the loaded flags to "unknown".
    act(() => {
      rerender({ id: "2" });
    });
    expect(result.current.todayPickup).toEqual({ kind: "unknown" });
  });

  // A same-day sick report started for child A must NOT mark child B absent if
  // the guardian navigates A→B (reusing the hook instance) before the POST
  // resolves. The optimistic setTodayAbsent in reportSick is captured to A's
  // studentId; without the identity guard it would apply to B's reused instance
  // and — since A's own later SSE event is filtered by studentId — leave B
  // wrongly absent indefinitely (#1725 review).
  it("does not apply a resolving same-day report to a different child after a switch", async () => {
    const today = berlinTodayISO();
    const todayWd = ((parseISODate(today).getDay() + 6) % 7) + 1;

    vi.spyOn(parentApi, "listSickDays").mockResolvedValue([]);
    vi.spyOn(parentApi, "listExcusedRequests").mockResolvedValue([]);
    vi.spyOn(parentApi, "listCareExceptions").mockResolvedValue(
      [] as CareException[],
    );
    // Both children load a real (non-absent) pickup, so a stray setTodayAbsent
    // would visibly flip the tile to "absent".
    vi.spyOn(parentApi, "getChildCareSchedule").mockImplementation(
      (id: string) =>
        Promise.resolve({
          weekdays: [
            {
              weekday: todayWd,
              pickup: id === "1" ? "16:00" : "15:00",
              modes: [],
            },
          ],
          can_request: false,
          request_capabilities: {
            arrival: false,
            pickup: false,
            departure_mode: false,
          },
          today_absent: false,
        }),
    );
    vi.spyOn(parentApi, "getChildFeatures").mockRejectedValue(
      new Error("no features"),
    );
    // Defer the sick-note resolution so we can switch children mid-submit.
    let resolveSubmit!: (v: { status_days: StatusDay[] }) => void;
    vi.spyOn(parentApi, "submitSickNote").mockReturnValue(
      new Promise((res) => {
        resolveSubmit = res;
      }),
    );

    const { result, rerender } = renderHook(({ id }) => useChildCare(id), {
      initialProps: { id: "1" },
    });

    await waitFor(() =>
      expect(result.current.todayPickup).toEqual({
        kind: "time",
        time: "16:00",
        changed: false,
      }),
    );

    // Start a same-day sick report for child A (submit is pending).
    let reportPromise!: Promise<void>;
    act(() => {
      reportPromise = result.current.reportSick([today], "", "sick");
    });

    // Navigate to child B; B loads its own (non-absent) pickup.
    rerender({ id: "2" });
    await waitFor(() =>
      expect(result.current.todayPickup).toEqual({
        kind: "time",
        time: "15:00",
        changed: false,
      }),
    );

    // A's report now resolves with a today absence. The guard must drop it so B
    // keeps showing its own pickup rather than flipping to "absent".
    await act(async () => {
      resolveSubmit({
        status_days: [
          {
            id: "1",
            student_id: "1",
            date: today,
            status: "sick",
            reported_at: "2026-03-01T09:00:00Z",
            source: "parent",
          },
        ],
      });
      await reportPromise;
    });

    expect(result.current.todayPickup).toEqual({
      kind: "time",
      time: "15:00",
      changed: false,
    });
  });
});
