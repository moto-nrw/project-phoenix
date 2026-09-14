import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { releaseFakeTimers } from "~/test/clock";
import { CareWeeklyPlanEditForm } from "./care-weekly-plan-editor";
import type { PickupAdjustmentPreview } from "~/lib/pickup-schedule-api";

const toastSuccess = vi.fn();
const toastError = vi.fn();

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: toastSuccess,
    error: toastError,
  }),
}));

// Die Angebots-Entscheidung öffnet aus dem Speichern heraus ein FormModal. Das
// Kit-Modal rendert in jsdom nicht vollständig, deshalb steht hier dieselbe
// Struktur (Titel, Fehler-Slot, Inhalt, Fuß) ohne Portal.
vi.mock("~/components/ui/form-modal", () => ({
  FormModal: ({
    isOpen,
    title,
    children,
    footer,
    error,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
    error?: string | { message: string } | null;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        <h2>{title}</h2>
        {error ? (
          <div role="alert">
            {typeof error === "string" ? error : error.message}
          </div>
        ) : null}
        {children}
        <div>{footer}</div>
      </div>
    ) : null,
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        <h2>{title}</h2>
        {children}
        {footer}
      </div>
    ) : null,
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

type FormProps = React.ComponentProps<typeof CareWeeklyPlanEditForm>;

function renderForm(props: Partial<FormProps> = {}) {
  const onSubmitWeekly = vi.fn().mockResolvedValue(undefined);
  const onCancel = vi.fn();
  const onSaved = vi.fn();
  const renderFormElement = (nextProps: Partial<FormProps> = {}) => (
    <CareWeeklyPlanEditForm
      careDaysSource="weekly_plan"
      weeklyArrival={weeklyArrival}
      weeklyPickup={weeklyPickup}
      onSubmitWeekly={onSubmitWeekly}
      onCancel={onCancel}
      onSaved={onSaved}
      {...props}
      {...nextProps}
    />
  );
  const result = render(renderFormElement());
  return {
    onSubmitWeekly,
    onCancel,
    onSaved,
    renderForm: renderFormElement,
    ...result,
  };
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

describe("CareWeeklyPlanEditForm", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows the recurring plan with Speichern and Abbrechen below it", () => {
    renderForm();

    expect(
      screen.getByRole("form", { name: "Wochenplan bearbeiten" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Gilt ab sofort für alle kommenden Wochen/),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeEnabled();
  });

  it("leaves the edit state without saving on Abbrechen", () => {
    const { onCancel, onSubmitWeekly } = renderForm();

    fireEvent.change(
      screen.getByLabelText("Ankunft", { selector: "#weekly-arrival-1" }),
      { target: { value: "08:45" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onSubmitWeekly).not.toHaveBeenCalled();
  });

  it("returns to the display after a successful save", async () => {
    const { onSaved } = renderForm();

    save();

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(toastSuccess).toHaveBeenCalledWith("Wochenplan wurde gespeichert");
  });

  it("shows a save error in the alert on top and stays in the edit state", async () => {
    const onSubmitWeekly = vi
      .fn()
      .mockRejectedValue(new Error("Backend kaputt"));
    const { onSaved } = renderForm({ onSubmitWeekly });

    save();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Backend kaputt",
    );
    expect(toastError).not.toHaveBeenCalled();
    expect(onSaved).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
  });

  it("explains when a weekly change needs a staff profile", async () => {
    const onSubmitWeekly = vi
      .fn()
      .mockRejectedValue(new Error("staff_profile_required"));
    renderForm({ onSubmitWeekly });

    save();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Diese Zeit wurde von den Eltern gesetzt",
    );
  });

  it("toggles a care day when the visible checkbox is clicked", () => {
    renderForm();

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
    const { onSaved } = renderForm({ onSubmitWeekly });

    save();
    expect(
      await screen.findByRole("dialog", { name: "Wochenplan speichern" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Gehzeiten passen zu keinem Angebot"),
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
    expect(onSaved).toHaveBeenCalledTimes(1);
    expect(toastSuccess).toHaveBeenCalledWith(
      "Dauerhafte Ausnahme wurde gespeichert",
    );
  });

  it("does not resubmit the weekly form while the decision is open", async () => {
    const onSubmitWeekly = vi.fn().mockResolvedValueOnce({
      ...decisionPreview("preview-1"),
      matching_offerings: [],
    });
    renderForm({ onSubmitWeekly });

    save();
    const heading = await screen.findByText(
      "Angebot oder dauerhafte Ausnahme wählen",
    );
    expect(heading).toHaveFocus();
    fireEvent.submit(
      screen.getByRole("form", { name: "Wochenplan bearbeiten" }),
    );

    expect(onSubmitWeekly).toHaveBeenCalledTimes(1);
  });

  it("returns from the decision to the grid without saving", async () => {
    const onSubmitWeekly = vi.fn().mockResolvedValueOnce({
      ...decisionPreview("preview-1"),
      matching_offerings: [],
    });
    const { onSaved } = renderForm({ onSubmitWeekly });

    save();
    await screen.findByRole("dialog", { name: "Wochenplan speichern" });
    fireEvent.click(
      screen.getByRole("button", { name: "Zurück zum Wochenplan" }),
    );

    expect(
      screen.queryByRole("dialog", { name: "Wochenplan speichern" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
    expect(onSaved).not.toHaveBeenCalled();
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
    renderForm({ onSubmitWeekly });

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
    renderForm({ onSubmitWeekly });

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
    releaseFakeTimers();
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
    const { onSaved } = renderForm({ onSubmitWeekly });

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
    expect(onSaved).toHaveBeenCalledTimes(1);
    expect(toastSuccess).toHaveBeenCalledWith(
      "Angebot und Wochenplan wurden geändert",
    );
  });

  it("changes a booked arrival time without enabling an unbooked day", async () => {
    const { onSubmitWeekly } = renderForm({ careDaysSource: "bookings" });

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
    const { onSubmitWeekly } = renderForm({
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
    renderForm();

    expect(
      screen.getByLabelText("Ankunft", { selector: "#weekly-arrival-3" }),
    ).toBeDisabled();
  });

  it("resets an own arrival time to the class time", async () => {
    const { onSubmitWeekly } = renderForm({
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

  it("keeps a weekly draft when schedule inputs receive new references", () => {
    const { rerender, renderForm: renderFormElement } = renderForm();

    fireEvent.change(
      screen.getByLabelText("Ankunft", { selector: "#weekly-arrival-1" }),
      { target: { value: "08:45" } },
    );

    rerender(
      renderFormElement({
        weeklyArrival: [...weeklyArrival],
        weeklyPickup: [...weeklyPickup],
      }),
    );

    expect(
      screen.getByLabelText("Ankunft", { selector: "#weekly-arrival-1" }),
    ).toHaveValue("08:45");
  });

  it("writes the weekly plan from the edit state", async () => {
    const { onSubmitWeekly } = renderForm();

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
    const { onSubmitWeekly } = renderForm();

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

  it("rejects an invalid weekly pickup time", async () => {
    const { onSubmitWeekly } = renderForm({
      weeklyPickup: [{ weekday: 2, pickupTime: "99:99" }],
    });

    save();

    expect(
      await screen.findByText("Ungültige Abholzeit für Dienstag."),
    ).toBeInTheDocument();
    expect(onSubmitWeekly).not.toHaveBeenCalled();
  });

  it("confirms before an emptied weekly field deletes a time", async () => {
    const { onSubmitWeekly } = renderForm();

    fireEvent.change(
      screen.getByLabelText("Abholung", { selector: "#weekly-pickup-2" }),
      { target: { value: "" } },
    );
    expect(
      screen.getByText("Wird beim Speichern entfernt: Dienstag Abholung."),
    ).toBeInTheDocument();
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

  it("keeps the draft when the removal question is answered with Zurück", async () => {
    const { onSubmitWeekly } = renderForm();

    fireEvent.change(
      screen.getByLabelText("Abholung", { selector: "#weekly-pickup-2" }),
      { target: { value: "" } },
    );
    save();
    await screen.findByRole("dialog", { name: "Zeiten entfernen?" });
    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));

    expect(
      screen.queryByRole("dialog", { name: "Zeiten entfernen?" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByLabelText("Abholung", { selector: "#weekly-pickup-2" }),
    ).toHaveValue("");
    expect(onSubmitWeekly).not.toHaveBeenCalled();
  });
});
