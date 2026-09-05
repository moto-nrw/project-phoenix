import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CarePlanEditorModal } from "./care-plan-editor-modal";
import type { ArrivalDayData } from "~/lib/arrival-schedule-helpers";
import type { DayData as PickupDayData } from "~/lib/pickup-schedule-helpers";
import type { PickupAdjustmentPreview } from "~/lib/pickup-schedule-api";

const toastSuccess = vi.fn();
const toastError = vi.fn();

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: toastSuccess,
    error: toastError,
  }),
}));

// Der Editor laeuft als SlideOver (Vaul). Vaul rendert in jsdom nicht, deshalb
// steht hier dieselbe Struktur ohne Animationsschicht.
vi.mock("~/components/ui/slide-over", () => ({
  SlideOver: ({
    open,
    onOpenChange,
    children,
  }: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    children: React.ReactNode;
  }) =>
    open ? (
      <div>
        <button type="button" onClick={() => onOpenChange(false)}>
          Panel schließen
        </button>
        {children}
      </div>
    ) : null,
  SlideOverContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SlideOverHeader: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SlideOverFooter: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SlideOverTitle: ({ children }: { children: React.ReactNode }) => (
    <h2>{children}</h2>
  ),
  SlideOverDescription: ({ children }: { children: React.ReactNode }) => (
    <p>{children}</p>
  ),
  SlideOverCloseButton: (
    props: React.ButtonHTMLAttributes<HTMLButtonElement>,
  ) => <button type="button" {...props} />,
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    isOpen,
    title,
    children,
    onConfirm,
    onClose,
    confirmText,
    cancelText,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    onConfirm: () => void;
    onClose: () => void;
    confirmText: string;
    cancelText: string;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        <h2>{title}</h2>
        {children}
        <button type="button" onClick={onClose}>
          {cancelText}
        </button>
        <button type="button" onClick={onConfirm}>
          {confirmText}
        </button>
      </div>
    ) : null,
}));

const MONDAY = new Date("2026-05-25T00:00:00");

const baseArrivalDay: ArrivalDayData = {
  date: MONDAY,
  weekday: 1,
  isToday: true,
  showSick: false,
  showClassTrip: false,
  showExcused: false,
  exception: undefined,
  baseSchedule: {
    id: 1,
    student_id: 42,
    weekday: 1,
    weekday_name: "Montag",
    expected_arrival: "08:00",
    notes: "Haupteingang",
    created_by: 1,
    created_at: "2026-05-01T00:00:00Z",
    updated_at: "2026-05-01T00:00:00Z",
  },
  effectiveTime: "08:00",
  effectiveReason: "Haupteingang",
  isException: false,
  isAbsent: false,
  notes: [
    {
      id: 11,
      student_id: 42,
      note_date: "2026-05-25",
      content: "Kommt mit Oma",
      created_by: 1,
      created_at: "2026-05-01T00:00:00Z",
      updated_at: "2026-05-01T00:00:00Z",
    },
  ],
};

const basePickupDay: PickupDayData = {
  date: MONDAY,
  weekday: 1,
  isToday: true,
  showSick: false,
  showClassTrip: false,
  showExcused: false,
  exception: undefined,
  baseSchedule: {
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
  offeringSchedule: {
    id: "0",
    studentId: "42",
    weekday: 1,
    weekdayName: "Montag",
    pickupTime: "15:30",
    source: "care_offering",
    careOfferingName: "Ganztagsbetreuung",
    createdBy: "0",
    createdAt: "0001-01-01T00:00:00Z",
    updatedAt: "0001-01-01T00:00:00Z",
  },
  effectiveTime: "15:00",
  effectiveNotes: "Bus",
  isException: false,
  notes: [
    {
      id: "21",
      studentId: "42",
      noteDate: "2026-05-25",
      content: "Traffic",
      createdBy: "1",
      createdAt: "2026-05-01T00:00:00Z",
      updatedAt: "2026-05-01T00:00:00Z",
    },
  ],
};

const weeklyArrival = [
  {
    weekday: 1,
    inCare: true,
    expected_arrival: "08:00",
    notes: "Haupteingang",
  },
  { weekday: 2, inCare: true, expected_arrival: "08:15", notes: null },
  // Not care days: since #2414 that is what an absent tick means, and it is
  // what an empty time meant before the split.
  { weekday: 3, inCare: false, expected_arrival: "", notes: null },
  { weekday: 4, inCare: false, expected_arrival: "", notes: null },
  { weekday: 5, inCare: false, expected_arrival: "", notes: null },
];

const weeklyPickup = [
  { weekday: 1, pickupTime: "15:00", notes: "Bus" },
  { weekday: 2, pickupTime: "16:00" },
  { weekday: 3, pickupTime: "" },
  { weekday: 4, pickupTime: "" },
  { weekday: 5, pickupTime: "" },
];

function renderEditor(
  props: Partial<React.ComponentProps<typeof CarePlanEditorModal>> = {},
) {
  const onSubmitException = vi.fn().mockResolvedValue(undefined);
  const onSubmitWeekly = vi.fn().mockResolvedValue(undefined);
  const onClose = vi.fn();
  const renderModal = (
    nextProps: Partial<React.ComponentProps<typeof CarePlanEditorModal>> = {},
  ) => (
    <CarePlanEditorModal
      isOpen
      careDaysSource="weekly_plan"
      onClose={onClose}
      date={MONDAY}
      arrivalDay={baseArrivalDay}
      pickupDay={basePickupDay}
      weeklyArrival={weeklyArrival}
      weeklyPickup={weeklyPickup}
      onSubmitException={onSubmitException}
      onSubmitWeekly={onSubmitWeekly}
      {...props}
      {...nextProps}
    />
  );
  const result = render(renderModal());
  return { onSubmitException, onSubmitWeekly, onClose, renderModal, ...result };
}

function save(): void {
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
}

function decisionPreview(token: string): PickupAdjustmentPreview {
  const catalogItem = (id: string, name: string) => ({
    offering_id: id,
    name,
    days_of_week_mode: "fixed" as const,
    available_days: ["mon", "tue"],
    selection_rule: "exactly_one",
    selection_group: "care",
    is_required: false,
    includes_lunch: false,
    includes_holiday_care: false,
    selected: false,
    selected_days: [],
    automatic: false,
    is_active: true,
    pickup_times: { mon: "14:30", tue: "14:30" },
    counts_as_care: true,
  });
  return {
    preview_token: token,
    effective_from: "2026-05-25",
    current_plan: "Mo 16:00 Uhr, Di 16:00 Uhr",
    proposed_plan: "Mo 14:30 Uhr, Di 14:30 Uhr",
    deviates_from_offering: true,
    resolution_required: true,
    matching_offerings: [
      {
        offering_id: "2",
        name: "Angebot A",
        selected_days: [],
        selections: [{ offering_id: "2", selected_days: [] }],
      },
      {
        offering_id: "3",
        name: "Angebot B",
        selected_days: [],
        selections: [{ offering_id: "3", selected_days: [] }],
      },
    ],
    offering_catalog: {
      phase_id: "10",
      phase_name: "Schuljahr",
      selection_mode: "required",
      earliest_effective_from: "2026-05-25",
      latest_effective_from: "2027-07-31",
      items: [catalogItem("2", "Angebot A"), catalogItem("3", "Angebot B")],
    },
  };
}

describe("CarePlanEditorModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("names itself an exception when opened from a day", () => {
    renderEditor();

    expect(screen.getByText("Ausnahme für Montag, 25.05.")).toBeInTheDocument();
    expect(screen.getByText(/Gilt nur an diesem Tag/)).toBeInTheDocument();
    // No scope control: the door you came through IS the reach.
    expect(screen.queryByRole("radio")).not.toBeInTheDocument();
  });

  it("shows the recurring plan when opened without a day", () => {
    renderEditor({ date: null, arrivalDay: null, pickupDay: null });

    expect(screen.getByText("Wochenplan bearbeiten")).toBeInTheDocument();
    expect(
      screen.getByText(/Gilt ab sofort für alle kommenden Wochen/),
    ).toBeInTheDocument();
  });

  it("toggles a care day when the visible checkbox is clicked", () => {
    renderEditor({ date: null, arrivalDay: null, pickupDay: null });

    const monday = screen.getByRole("checkbox", { name: "Montag" });
    expect(monday).toBeChecked();

    fireEvent.click(monday.nextElementSibling!);

    expect(monday).not.toBeChecked();
  });

  it("requires an explicit lasting exception when no offering matches", async () => {
    const onSubmitWeekly = vi
      .fn()
      .mockResolvedValueOnce({
        preview_token: "preview-1",
        effective_from: "2026-05-25",
        current_plan: "Mo 16:00 Uhr, Di 16:00 Uhr",
        proposed_plan: "Mo 13:45 Uhr, Di 13:45 Uhr",
        deviates_from_offering: true,
        resolution_required: true,
        matching_offerings: [],
      })
      .mockResolvedValueOnce(undefined);
    renderEditor({
      date: null,
      arrivalDay: null,
      pickupDay: null,
      onSubmitWeekly,
    });

    save();
    expect(
      await screen.findByText("Gehzeiten passen zu keinem Angebot"),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", {
        name: "Als dauerhafte Ausnahme speichern",
      }),
    );

    await waitFor(() => {
      expect(onSubmitWeekly).toHaveBeenLastCalledWith(
        expect.any(Object),
        expect.objectContaining({
          resolution: "exception",
          confirm: true,
        }),
      );
    });
  });

  it("does not resubmit the weekly form from the decision step", async () => {
    const onSubmitWeekly = vi.fn().mockResolvedValueOnce({
      ...decisionPreview("preview-1"),
      matching_offerings: [],
    });
    renderEditor({
      date: null,
      arrivalDay: null,
      pickupDay: null,
      onSubmitWeekly,
    });

    save();
    const heading = await screen.findByText(
      "Angebot oder dauerhafte Ausnahme wählen",
    );
    expect(heading).toHaveFocus();
    fireEvent.submit(heading.closest("form")!);

    expect(onSubmitWeekly).toHaveBeenCalledTimes(1);
  });

  it("clears offer A when the preview for offer B fails", async () => {
    const initial = decisionPreview("preview-1");
    const readyA = {
      ...decisionPreview("preview-2"),
      offering_consequences: {
        selections: [{ offering_id: "2", state: "booked" as const, days: [] }],
        manual_planning_conflicts: [],
        arrival_expectations_follow_bookings: true,
      },
    };
    const onSubmitWeekly = vi
      .fn()
      .mockResolvedValueOnce(initial)
      .mockResolvedValueOnce(readyA)
      .mockRejectedValueOnce(new Error("Angebot B ist nicht mehr verfügbar"));
    renderEditor({
      date: null,
      arrivalDay: null,
      pickupDay: null,
      onSubmitWeekly,
    });

    save();
    fireEvent.click(
      await screen.findByRole("button", { name: "Auf „Angebot A“ umbuchen" }),
    );
    await screen.findByRole("button", { name: "Angebot ändern und speichern" });
    fireEvent.click(
      screen.getByRole("button", { name: "Auf „Angebot B“ umbuchen" }),
    );

    await screen.findByText("Angebot B ist nicht mehr verfügbar");
    expect(
      screen.queryByRole("button", { name: "Angebot ändern und speichern" }),
    ).not.toBeInTheDocument();
    expect(
      onSubmitWeekly.mock.calls.some(([, adjustment]) => adjustment?.confirm),
    ).toBe(false);
  });

  it("keeps the current-date preview when switching from a future offering to an exception", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime("2026-05-25T12:00:00");
    const initial = decisionPreview("preview-current");
    const selected = {
      ...decisionPreview("preview-selected"),
      offering_consequences: {
        selections: [{ offering_id: "2", state: "booked" as const, days: [] }],
        manual_planning_conflicts: [],
        arrival_expectations_follow_bookings: true,
      },
    };
    const future = {
      ...selected,
      preview_token: "preview-future",
      effective_from: "2026-06-01",
      matching_offerings: [],
    };
    const onSubmitWeekly = vi
      .fn()
      .mockResolvedValueOnce(initial)
      .mockResolvedValueOnce(selected)
      .mockResolvedValueOnce(future)
      .mockResolvedValueOnce(undefined);
    renderEditor({
      date: null,
      arrivalDay: null,
      pickupDay: null,
      onSubmitWeekly,
    });

    save();
    fireEvent.click(
      await screen.findByRole("button", { name: "Auf „Angebot A“ umbuchen" }),
    );
    await screen.findByRole("button", { name: "Angebot ändern und speichern" });

    fireEvent.click(screen.getByRole("button", { name: "Gilt ab" }));
    fireEvent.click(screen.getByRole("button", { name: "Nächster Monat" }));
    fireEvent.click(
      await screen.findByRole("button", {
        name: /Montag, 1\. Juni 2026/i,
      }),
    );
    await waitFor(() => expect(onSubmitWeekly).toHaveBeenCalledTimes(3));

    expect(
      screen.queryByRole("button", { name: "Angebot ändern und speichern" }),
    ).not.toBeInTheDocument();

    expect(
      screen.getByRole("button", {
        name: "Als dauerhafte Ausnahme speichern",
      }),
    ).toBeDisabled();
    expect(onSubmitWeekly).toHaveBeenCalledTimes(3);
    vi.useRealTimers();
  });

  it("shows all exact matches and requires confirmation before changing the offering", async () => {
    const initialPreview = {
      preview_token: "preview-1",
      effective_from: "2026-05-25",
      current_plan: "Mo 16:00 Uhr, Di 16:00 Uhr",
      proposed_plan: "Mo 14:30 Uhr, Di 14:30 Uhr",
      deviates_from_offering: true,
      resolution_required: true,
      matching_offerings: [
        {
          offering_id: "2",
          name: "Bis 14:30",
          selected_days: [],
          selections: [
            { offering_id: "4", selected_days: [] },
            { offering_id: "2", selected_days: [] },
          ],
        },
        {
          offering_id: "3",
          name: "Flexibel bis 14:30",
          selected_days: ["mon", "tue"],
          selections: [
            { offering_id: "4", selected_days: [] },
            { offering_id: "3", selected_days: ["mon", "tue"] },
          ],
        },
      ],
      offering_catalog: {
        phase_id: "10",
        phase_name: "Schuljahr",
        selection_mode: "required",
        earliest_effective_from: "2026-05-25",
        latest_effective_from: "2027-07-31",
        items: [
          {
            offering_id: "1",
            name: "Bis 16:00",
            days_of_week_mode: "fixed" as const,
            available_days: ["mon", "tue"],
            selection_rule: "exactly_one",
            selection_group: "care",
            is_required: false,
            includes_lunch: false,
            includes_holiday_care: false,
            selected: true,
            selected_days: ["mon", "tue"],
            automatic: false,
            is_active: true,
            pickup_times: { mon: "16:00", tue: "16:00" },
            counts_as_care: true,
          },
          {
            offering_id: "2",
            name: "Bis 14:30",
            days_of_week_mode: "fixed" as const,
            available_days: ["mon", "tue"],
            selection_rule: "exactly_one",
            selection_group: "care",
            is_required: false,
            includes_lunch: false,
            includes_holiday_care: false,
            selected: false,
            selected_days: [],
            automatic: false,
            is_active: true,
            pickup_times: { mon: "14:30", tue: "14:30" },
            counts_as_care: true,
          },
          {
            offering_id: "3",
            name: "Flexibel bis 14:30",
            days_of_week_mode: "parent_choice" as const,
            available_days: ["mon", "tue"],
            selection_rule: "exactly_one",
            selection_group: "care",
            is_required: false,
            includes_lunch: false,
            includes_holiday_care: false,
            selected: false,
            selected_days: [],
            automatic: false,
            is_active: true,
            capacity: 2,
            free_slots: 0,
            pickup_times: { mon: "14:30", tue: "14:30" },
            counts_as_care: true,
          },
          {
            offering_id: "4",
            name: "Mittagessen",
            days_of_week_mode: "fixed" as const,
            available_days: ["mon", "tue"],
            selection_rule: "multiple",
            selection_group: "food",
            is_required: false,
            includes_lunch: true,
            includes_holiday_care: false,
            selected: true,
            selected_days: [],
            automatic: false,
            is_active: true,
            pickup_times: {},
            counts_as_care: false,
          },
        ],
      },
    };
    const confirmedPreview = {
      ...initialPreview,
      preview_token: "preview-2",
      offering_consequences: {
        selections: [
          { offering_id: "1", state: "removed" as const, days: [] },
          {
            offering_id: "2",
            state: "booked" as const,
            days: ["mon", "tue"],
          },
        ],
        manual_planning_conflicts: [],
        arrival_expectations_follow_bookings: true,
      },
      removed_manual_notes: [{ weekday: 1, note: "Abholung mit dem Bus" }],
    };
    const onSubmitWeekly = vi
      .fn()
      .mockResolvedValueOnce(initialPreview)
      .mockResolvedValueOnce(confirmedPreview)
      .mockResolvedValueOnce(undefined);
    renderEditor({
      date: null,
      arrivalDay: null,
      pickupDay: null,
      onSubmitWeekly,
    });

    save();
    expect(
      await screen.findByRole("button", { name: "Auf „Bis 14:30“ umbuchen" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", {
        name: "Auf „Flexibel bis 14:30“ umbuchen",
      }),
    ).toBeDisabled();
    expect(
      screen.getByText("Dieses Angebot hat keinen freien Platz."),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Auf „Bis 14:30“ umbuchen" }),
    );
    expect(
      await screen.findByText("Montag: Abholung mit dem Bus"),
    ).toBeInTheDocument();

    const saveOffering = await screen.findByRole("button", {
      name: "Angebot ändern und speichern",
    });
    expect(saveOffering).toBeDisabled();
    fireEvent.click(screen.getByText(/Ich bestätige: Das Angebot gilt ab/));
    expect(saveOffering).toBeEnabled();
    fireEvent.click(saveOffering);

    await waitFor(() => {
      expect(onSubmitWeekly).toHaveBeenLastCalledWith(
        expect.any(Object),
        expect.objectContaining({
          resolution: "offering",
          confirm: true,
          selections: [
            { offering_id: "4", selected_days: [] },
            { offering_id: "2", selected_days: [] },
          ],
        }),
      );
    });
  });

  it("changes a booked arrival time without enabling an unbooked day", async () => {
    const { onSubmitWeekly } = renderEditor({
      date: null,
      arrivalDay: null,
      pickupDay: null,
      careDaysSource: "bookings",
    });

    expect(
      screen.getByText(/Die Betreuungstage kommen aus den Buchungen/),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Montag")).toBeDisabled();
    expect(
      screen.getByLabelText("Ankunft", { selector: "#weekly-arrival-1" }),
    ).toBeEnabled();
    expect(
      screen.getByLabelText("Ankunft", { selector: "#weekly-arrival-3" }),
    ).toBeDisabled();

    fireEvent.change(
      screen.getByLabelText("Ankunft", { selector: "#weekly-arrival-1" }),
      { target: { value: "08:45" } },
    );
    save();

    await waitFor(() => {
      expect(onSubmitWeekly).toHaveBeenCalledWith({
        arrivalSchedules: [
          {
            weekday: 1,
            inCare: true,
            expected_arrival: "08:45",
            notes: "Haupteingang",
          },
          { weekday: 2, inCare: true, expected_arrival: "08:15", notes: null },
        ],
        pickupSchedules: [
          { weekday: 1, pickupTime: "15:00", notes: "Bus" },
          { weekday: 2, pickupTime: "16:00", notes: undefined },
        ],
      });
    });
  });

  it("keeps booked care days when clearing their own arrival time", async () => {
    const { onSubmitWeekly } = renderEditor({
      date: null,
      arrivalDay: null,
      pickupDay: null,
      careDaysSource: "bookings",
      weeklyArrival: [
        {
          weekday: 1,
          inCare: true,
          expected_arrival: "",
          classTime: "11:45",
          notes: null,
        },
        {
          weekday: 2,
          inCare: true,
          expected_arrival: "12:15",
          classTime: "11:45",
          notes: null,
        },
        ...weeklyArrival.slice(2),
      ],
    });

    save();

    await waitFor(() =>
      expect(onSubmitWeekly).toHaveBeenCalledWith(
        expect.objectContaining({
          arrivalSchedules: [
            expect.objectContaining({
              weekday: 1,
              expected_arrival: "",
              notes: null,
            }),
            expect.objectContaining({
              weekday: 2,
              expected_arrival: "12:15",
            }),
          ],
        }),
      ),
    );
  });

  it("disables arrival fields when a day is not in care", () => {
    renderEditor({ date: null, arrivalDay: null, pickupDay: null });

    expect(
      screen.getByLabelText("Ankunft", { selector: "#weekly-arrival-3" }),
    ).toBeDisabled();
  });

  it("resets an own arrival time to the class time", async () => {
    const { onSubmitWeekly } = renderEditor({
      date: null,
      arrivalDay: null,
      pickupDay: null,
      weeklyArrival: [
        {
          weekday: 1,
          inCare: true,
          expected_arrival: "08:00",
          classTime: "08:30",
          notes: null,
        },
        ...weeklyArrival.slice(1),
      ],
    });

    fireEvent.click(
      screen.getAllByRole("button", { name: "Klassenzeit nutzen" })[0]!,
    );
    expect(
      screen.getByLabelText("Ankunft", { selector: "#weekly-arrival-1" }),
    ).toHaveValue("");
    save();

    await waitFor(() =>
      expect(onSubmitWeekly).toHaveBeenCalledWith(
        expect.objectContaining({
          arrivalSchedules: expect.arrayContaining([
            expect.objectContaining({ weekday: 1, expected_arrival: "" }),
          ]),
        }),
      ),
    );
  });

  it("saves a day exception with a Grund", async () => {
    const { onSubmitException } = renderEditor();

    fireEvent.click(screen.getAllByRole("button", { name: "Andere Zeit" })[1]!);
    fireEvent.change(
      screen.getByLabelText("Uhrzeit", {
        selector: "#exception-abholung-time",
      }),
      { target: { value: "16:30" } },
    );
    fireEvent.change(
      screen.getByLabelText("Grund", {
        selector: "#exception-abholung-reason",
      }),
      { target: { value: "Oma holt ab" } },
    );
    save();

    await waitFor(() => {
      expect(onSubmitException).toHaveBeenCalledWith({
        date: "2026-05-25",
        arrival: null,
        pickup: { kind: "time", time: "16:30", reason: "Oma holt ab" },
      });
    });
  });

  it("can set 'Keine Abholung', which the old day editor could not express", async () => {
    const { onSubmitException } = renderEditor();

    fireEvent.click(screen.getByRole("button", { name: "Keine Abholung" }));
    save();

    await waitFor(() => {
      expect(onSubmitException).toHaveBeenCalledWith(
        expect.objectContaining({ pickup: { kind: "none", reason: null } }),
      );
    });
  });

  it("removes the override when a leg is set back to Regulär", async () => {
    const { onSubmitException } = renderEditor({
      pickupDay: {
        ...basePickupDay,
        exception: {
          id: "99",
          studentId: "42",
          exceptionDate: "2026-05-25",
          pickupTime: "16:30",
          reason: "Termin",
          source: "staff",
          createdBy: "1",
          createdAt: "2026-05-20T00:00:00Z",
          updatedAt: "2026-05-20T00:00:00Z",
        },
        isException: true,
      },
    });

    fireEvent.click(screen.getAllByRole("button", { name: "Regulär" })[1]!);
    save();

    await waitFor(() => {
      expect(onSubmitException).toHaveBeenCalledWith(
        expect.objectContaining({ pickup: { kind: "regular" } }),
      );
    });
  });

  it("refuses to save when nothing was changed", async () => {
    const { onSubmitException } = renderEditor();

    save();

    expect(
      await screen.findByText("Bitte zuerst eine Zeit ändern."),
    ).toBeInTheDocument();
    expect(onSubmitException).not.toHaveBeenCalled();
  });

  it("rejects an invalid time", async () => {
    const { onSubmitException } = renderEditor();

    fireEvent.click(screen.getAllByRole("button", { name: "Andere Zeit" })[0]!);
    fireEvent.change(
      screen.getByLabelText("Uhrzeit", { selector: "#exception-ankunft-time" }),
      { target: { value: "99:99" } },
    );
    save();

    expect(
      await screen.findByText("Bitte eine gültige Ankunftszeit eingeben."),
    ).toBeInTheDocument();
    expect(onSubmitException).not.toHaveBeenCalled();
  });

  it("asks before overwriting a parent-set time", async () => {
    const { onSubmitException } = renderEditor({
      pickupDay: {
        ...basePickupDay,
        exception: {
          id: "99",
          studentId: "42",
          exceptionDate: "2026-05-25",
          pickupTime: "15:00",
          reason: "Arzttermin",
          source: "guardian",
          createdBy: "0",
          createdAt: "2026-05-20T00:00:00Z",
          updatedAt: "2026-05-20T00:00:00Z",
        },
        isException: true,
      },
    });

    fireEvent.change(
      screen.getByLabelText("Uhrzeit", {
        selector: "#exception-abholung-time",
      }),
      { target: { value: "16:30" } },
    );
    save();

    expect(
      await screen.findByRole("dialog", {
        name: "Eltern-Angabe überschreiben?",
      }),
    ).toBeInTheDocument();
    expect(onSubmitException).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("button", { name: "Trotzdem überschreiben" }),
    );

    await waitFor(() => {
      expect(onSubmitException).toHaveBeenCalled();
    });
    expect(
      screen.queryByRole("dialog", { name: "Eltern-Angabe überschreiben?" }),
    ).not.toBeInTheDocument();
  });

  it("explains when a pickup exception needs a staff profile", async () => {
    const onSubmitException = vi
      .fn()
      .mockRejectedValue(new Error("staff_profile_required"));
    renderEditor({
      onSubmitException,
      pickupDay: {
        ...basePickupDay,
        exception: {
          id: "99",
          studentId: "42",
          exceptionDate: "2026-05-25",
          pickupTime: "15:00",
          reason: "Arzttermin",
          source: "guardian",
          createdBy: "0",
          createdAt: "2026-05-20T00:00:00Z",
          updatedAt: "2026-05-20T00:00:00Z",
        },
        isException: true,
      },
    });

    fireEvent.change(
      screen.getByLabelText("Uhrzeit", {
        selector: "#exception-abholung-time",
      }),
      { target: { value: "16:30" } },
    );
    save();
    fireEvent.click(
      await screen.findByRole("button", { name: "Trotzdem überschreiben" }),
    );

    expect(
      await screen.findByText(
        "Diese Zeit wurde von den Eltern gesetzt und kann nur von Mitarbeitenden mit Personalprofil geändert werden.",
      ),
    ).toBeInTheDocument();
    expect(onSubmitException).toHaveBeenCalled();
  });

  it("keeps an exception draft when weekly inputs receive new references", () => {
    const { rerender, renderModal } = renderEditor();

    fireEvent.click(screen.getAllByRole("button", { name: "Andere Zeit" })[1]!);
    fireEvent.change(
      screen.getByLabelText("Uhrzeit", {
        selector: "#exception-abholung-time",
      }),
      { target: { value: "16:30" } },
    );

    rerender(
      renderModal({
        weeklyArrival: [...weeklyArrival],
        weeklyPickup: [...weeklyPickup],
      }),
    );

    expect(
      screen.getByLabelText("Uhrzeit", {
        selector: "#exception-abholung-time",
      }),
    ).toHaveValue("16:30");
  });

  it("keeps an exception draft when day notes are refreshed", () => {
    const { rerender, renderModal } = renderEditor();

    fireEvent.click(screen.getAllByRole("button", { name: "Andere Zeit" })[1]!);
    fireEvent.change(
      screen.getByLabelText("Uhrzeit", {
        selector: "#exception-abholung-time",
      }),
      { target: { value: "16:30" } },
    );
    fireEvent.change(
      screen.getByLabelText("Grund", {
        selector: "#exception-abholung-reason",
      }),
      { target: { value: "Oma holt ab" } },
    );

    rerender(
      renderModal({
        arrivalDay: { ...baseArrivalDay, notes: [...baseArrivalDay.notes] },
        pickupDay: { ...basePickupDay, notes: [...basePickupDay.notes] },
      }),
    );

    expect(
      screen.getByLabelText("Uhrzeit", {
        selector: "#exception-abholung-time",
      }),
    ).toHaveValue("16:30");
    expect(
      screen.getByLabelText("Grund", {
        selector: "#exception-abholung-reason",
      }),
    ).toHaveValue("Oma holt ab");
  });

  it("keeps a weekly draft when schedule inputs receive new references", () => {
    const { rerender, renderModal } = renderEditor({
      date: null,
      arrivalDay: null,
      pickupDay: null,
    });

    fireEvent.change(
      screen.getByLabelText("Ankunft", { selector: "#weekly-arrival-1" }),
      { target: { value: "08:45" } },
    );

    rerender(
      renderModal({
        date: null,
        arrivalDay: null,
        pickupDay: null,
        weeklyArrival: [...weeklyArrival],
        weeklyPickup: [...weeklyPickup],
      }),
    );

    expect(
      screen.getByLabelText("Ankunft", { selector: "#weekly-arrival-1" }),
    ).toHaveValue("08:45");
  });

  it("shows a note mutation error and keeps the draft", async () => {
    const onCreateArrivalNote = vi
      .fn()
      .mockRejectedValue(new Error("Hinweis konnte nicht gespeichert werden"));
    renderEditor({ onCreateArrivalNote });

    const draft = screen.getByLabelText("Ankunft Hinweis hinzufügen");
    fireEvent.change(draft, { target: { value: "Bitte klingeln" } });
    fireEvent.click(draft.parentElement!.querySelector("button")!);

    expect(
      await screen.findByText("Hinweis konnte nicht gespeichert werden"),
    ).toBeInTheDocument();
    expect(toastError).toHaveBeenCalledWith(
      "Hinweis konnte nicht gespeichert werden",
    );
    expect(draft).toHaveValue("Bitte klingeln");
  });

  it("shows an error when resetting to the offering pickup time fails", async () => {
    const onResetPickupToOffering = vi
      .fn()
      .mockRejectedValue(new Error("request failed"));
    renderEditor({ onResetPickupToOffering });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Abholung auf Angebots-Gehzeit zurücksetzen",
      }),
    );

    expect(onResetPickupToOffering).toHaveBeenCalledWith(1, "2026-05-25");

    const message =
      "Die Abholung konnte nicht zurückgesetzt werden. Bitte versuchen Sie es noch einmal.";
    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(toastError).toHaveBeenCalledWith(message);
  });

  it("limits day notes to the API-supported length", () => {
    renderEditor();

    expect(screen.getByLabelText("Ankunft Hinweis hinzufügen")).toHaveAttribute(
      "maxlength",
      "500",
    );
    expect(screen.getByLabelText("Ankunft Hinweis")).toHaveAttribute(
      "maxlength",
      "500",
    );
  });

  it("prevents duplicate day-note creations while a request is pending", async () => {
    let resolveCreate: (() => void) | undefined;
    const onCreateArrivalNote = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          resolveCreate = resolve;
        }),
    );
    renderEditor({ onCreateArrivalNote });

    const draft = screen.getByLabelText("Ankunft Hinweis hinzufügen");
    fireEvent.change(draft, { target: { value: "Bitte klingeln" } });
    const addButton = draft.parentElement!.querySelector("button")!;
    fireEvent.click(addButton);
    fireEvent.click(addButton);

    expect(onCreateArrivalNote).toHaveBeenCalledOnce();
    expect(addButton).toBeDisabled();
    resolveCreate?.();

    await waitFor(() => {
      expect(draft).toHaveValue("");
    });
    fireEvent.change(draft, { target: { value: "Neue Notiz" } });
    expect(addButton).not.toBeDisabled();
  });

  it("does not save an edited day note when the exception editor is cancelled", () => {
    const onUpdateArrivalNote = vi.fn().mockResolvedValue(undefined);
    const { onClose } = renderEditor({ onUpdateArrivalNote });

    fireEvent.change(screen.getByLabelText("Ankunft Hinweis"), {
      target: { value: "Kommt mit dem Bus" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(onUpdateArrivalNote).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("clears day note drafts when the editor switches to another date", async () => {
    const onCreateArrivalNote = vi.fn().mockResolvedValue(undefined);
    const { rerender, renderModal } = renderEditor({ onCreateArrivalNote });
    const draft = screen.getByLabelText("Ankunft Hinweis hinzufügen");

    fireEvent.change(draft, { target: { value: "Nur Montag" } });
    rerender(
      renderModal({
        date: new Date("2026-05-26T00:00:00"),
        arrivalDay: {
          ...baseArrivalDay,
          date: new Date("2026-05-26T00:00:00"),
          notes: [],
        },
        pickupDay: {
          ...basePickupDay,
          date: new Date("2026-05-26T00:00:00"),
          notes: [],
        },
      }),
    );

    const nextDraft = screen.getByLabelText("Ankunft Hinweis hinzufügen");
    expect(nextDraft).toHaveValue("");

    fireEvent.change(nextDraft, { target: { value: "Nur Dienstag" } });
    fireEvent.click(nextDraft.parentElement!.querySelector("button")!);

    await waitFor(() => {
      expect(onCreateArrivalNote).toHaveBeenCalledWith(
        "2026-05-26",
        "Nur Dienstag",
      );
    });
  });

  it("writes the weekly plan from the plan view", async () => {
    const { onSubmitWeekly } = renderEditor({
      date: null,
      arrivalDay: null,
      pickupDay: null,
    });

    fireEvent.change(
      screen.getByLabelText("Ankunft", { selector: "#weekly-arrival-1" }),
      { target: { value: "08:45" } },
    );
    save();

    await waitFor(() => {
      expect(onSubmitWeekly).toHaveBeenCalledWith({
        arrivalSchedules: [
          {
            weekday: 1,
            inCare: true,
            expected_arrival: "08:45",
            notes: "Haupteingang",
          },
          { weekday: 2, inCare: true, expected_arrival: "08:15", notes: null },
        ],
        pickupSchedules: [
          { weekday: 1, pickupTime: "15:00", notes: "Bus" },
          { weekday: 2, pickupTime: "16:00", notes: undefined },
        ],
      });
    });
  });

  // Business rule changed with #2414: an arrival note hangs on the care day,
  // not on a time — the time may legitimately come from the class.
  it("rejects a weekly arrival note without a care day", async () => {
    const { onSubmitWeekly } = renderEditor({
      date: null,
      arrivalDay: null,
      pickupDay: null,
    });

    fireEvent.click(screen.getAllByRole("button", { name: "Notizen" })[2]!);
    fireEvent.change(
      screen.getByLabelText("Ankunftsnotiz (jede Woche)", {
        selector: "#weekly-arrival-notes-3",
      }),
      { target: { value: "Haupteingang" } },
    );
    save();

    expect(
      await screen.findByText(
        "Eine Ankunftsnotiz für Mittwoch braucht einen Betreuungstag.",
      ),
    ).toBeInTheDocument();
    expect(onSubmitWeekly).not.toHaveBeenCalled();
  });

  it("confirms before an emptied weekly field deletes a time", async () => {
    const { onSubmitWeekly } = renderEditor({
      date: null,
      arrivalDay: null,
      pickupDay: null,
    });

    fireEvent.change(
      screen.getByLabelText("Abholung", { selector: "#weekly-pickup-2" }),
      { target: { value: "" } },
    );
    save();

    expect(
      await screen.findByRole("dialog", { name: "Zeiten entfernen?" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Dienstag Abholung")).toBeInTheDocument();
    expect(onSubmitWeekly).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Trotzdem speichern" }));

    await waitFor(() => {
      expect(onSubmitWeekly).toHaveBeenCalledWith(
        expect.objectContaining({
          pickupSchedules: [{ weekday: 1, pickupTime: "15:00", notes: "Bus" }],
        }),
      );
    });
  });
});
