import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockToastSuccess, mockToastError, mockCreate, mockUpdate, mockDelete } =
  vi.hoisted(() => ({
    mockToastSuccess: vi.fn(),
    mockToastError: vi.fn(),
    mockCreate: vi.fn(),
    mockUpdate: vi.fn(),
    mockDelete: vi.fn(),
  }));

// The date fields moved from native inputs to the kit picker; this stub keeps
// them settable via fireEvent.change and forwards min/max so the bound
// assertions below still pin what the component computes. Imported inside the
// factory because vi.mock is hoisted above the imports.
// Vaul (SlideOver) rendert in jsdom nichts. Derselbe Ersatz wie in
// components/ui/slide-over.test.tsx — die Struktur bleibt, nur die
// Animationsschicht fällt weg.
vi.mock("vaul", async () => {
  const React = await import("react");

  return {
    Drawer: {
      Root: ({
        children,
        open,
      }: {
        children: React.ReactNode;
        open?: boolean;
      }) => (open === false ? null : <div>{children}</div>),
      Portal: ({ children }: { children: React.ReactNode }) => <>{children}</>,
      Overlay: React.forwardRef<
        HTMLDivElement,
        React.HTMLAttributes<HTMLDivElement>
      >((props, ref) => <div ref={ref} {...props} />),
      Content: React.forwardRef<
        HTMLDivElement,
        React.HTMLAttributes<HTMLDivElement>
      >((props, ref) => <div ref={ref} {...props} />),
      Close: React.forwardRef<
        HTMLButtonElement,
        React.ButtonHTMLAttributes<HTMLButtonElement>
      >((props, ref) => <button ref={ref} {...props} />),
      Title: React.forwardRef<
        HTMLHeadingElement,
        React.HTMLAttributes<HTMLHeadingElement>
      >(({ children, ...props }, ref) => (
        <h2 ref={ref} {...props}>
          {children ?? "Titel"}
        </h2>
      )),
      Description: React.forwardRef<
        HTMLParagraphElement,
        React.HTMLAttributes<HTMLParagraphElement>
      >((props, ref) => <p ref={ref} {...props} />),
    },
  };
});

vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mockToastSuccess, error: mockToastError }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() }),
}));

vi.mock("~/lib/calendar-period-api", () => ({
  calendarPeriodService: {
    create: mockCreate,
    update: mockUpdate,
    delete: mockDelete,
  },
}));

import { CalendarPeriodModal } from "./calendar-period-modal";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";

const period: CalendarPeriod = {
  id: "5",
  tenantId: "1",
  name: "Schuljahr 2026/2027",
  periodType: "school_year",
  startDate: "2026-08-01",
  endDate: "2027-07-31",
  weekCycleLength: 2,
  weekCycleAnchor: "2026-08-03",
  isActive: true,
  createdAt: "2026-05-01T00:00:00Z",
  updatedAt: "2026-05-01T00:00:00Z",
};

describe("CalendarPeriodModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // create/update return { period, warnings }; with empty warnings the
    // modal closes right after saving, otherwise it stays open and lists them.
    mockCreate.mockResolvedValue({ period, warnings: [] });
    mockUpdate.mockResolvedValue({ period, warnings: [] });
    mockDelete.mockResolvedValue(undefined);
  });

  it("creates a period using defaults and validates date/cycle rules", async () => {
    const onClose = vi.fn();
    const onSaved = vi.fn();
    render(
      <CalendarPeriodModal
        isOpen
        onClose={onClose}
        onSaved={onSaved}
        createDefaults={{
          name: "Schuljahr 2026/2027",
          startDate: "2026-08-01",
          endDate: "2027-07-31",
          weekCycleLength: "1",
        }}
      />,
    );

    fireEvent.change(screen.getByLabelText("Enddatum*"), {
      target: { value: "2026-07-31" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Enddatum muss nach dem Startdatum liegen.",
    );

    fireEvent.change(screen.getByLabelText("Enddatum*"), {
      target: { value: "2027-07-31" },
    });
    fireEvent.change(screen.getByLabelText("Wiederholung in Wochen"), {
      target: { value: "2" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bei einer Wiederholung über mehrere Wochen ist das Startdatum der Wiederholung erforderlich.",
    );

    fireEvent.change(screen.getByLabelText("Startdatum der Wiederholung*"), {
      target: { value: "2026-08-03" },
    });
    fireEvent.click(screen.getByLabelText(/Zeitraum im Plan verwenden/));
    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));

    await waitFor(() =>
      expect(mockCreate).toHaveBeenCalledWith({
        name: "Schuljahr 2026/2027",
        period_type: "school_year",
        start_date: "2026-08-01",
        end_date: "2027-07-31",
        week_cycle_length: 2,
        is_active: false,
        week_cycle_anchor: "2026-08-03",
      }),
    );
    expect(mockToastSuccess).toHaveBeenCalledWith(
      'Kalenderzeitraum "Schuljahr 2026/2027" angelegt',
    );
    expect(onSaved).toHaveBeenCalledWith(period);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("updates and deletes an existing period with confirmation", async () => {
    const onClose = vi.fn();
    const onSaved = vi.fn();
    const onDeleted = vi.fn();
    const { rerender } = render(
      <CalendarPeriodModal
        isOpen
        onClose={onClose}
        onSaved={onSaved}
        onDeleted={onDeleted}
        initial={period}
      />,
    );

    fireEvent.change(screen.getByLabelText("Bezeichnung*"), {
      target: { value: "Neues Schuljahr" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(mockUpdate).toHaveBeenCalledWith(
        "5",
        expect.objectContaining({ name: "Neues Schuljahr" }),
      ),
    );
    expect(onSaved).toHaveBeenCalledWith(period);

    rerender(
      <CalendarPeriodModal
        isOpen
        onClose={onClose}
        onSaved={onSaved}
        onDeleted={onDeleted}
        initial={period}
      />,
    );
    // Löschen bestätigt die ConfirmDeleteModal statt einer Umschaltung der
    // Fusszeile (#3110).
    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));
    const dialog = screen.getByRole("dialog", {
      name: "Kalenderzeitraum löschen",
    });
    expect(
      within(dialog).getByText(/Beim Löschen werden bestehende Verknüpfungen/),
    ).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Abbrechen" }));
    await waitFor(() =>
      expect(
        screen.queryByRole("dialog", { name: "Kalenderzeitraum löschen" }),
      ).not.toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));
    const reopened = screen.getByRole("dialog", {
      name: "Kalenderzeitraum löschen",
    });
    fireEvent.click(within(reopened).getByRole("button", { name: "Löschen" }));
    fireEvent.click(
      within(reopened).getByRole("button", { name: "Endgültig löschen" }),
    );
    await waitFor(() => expect(mockDelete).toHaveBeenCalledWith("5"));
    expect(onDeleted).toHaveBeenCalledWith(period);
  });

  it("manages phase links from the period side via the phaseLink prop", async () => {
    const onToggle = vi.fn().mockResolvedValue(undefined);
    render(
      <CalendarPeriodModal
        isOpen
        onClose={vi.fn()}
        onSaved={vi.fn()}
        initial={period}
        phaseLink={{
          phases: [
            {
              id: "7",
              name: "Demo Anmeldung",
              calendar_period_id: period.id,
              is_active: true,
            },
            {
              id: "8",
              name: "Ferienbetreuung",
              calendar_period_id: "99",
              is_active: false,
            },
          ],
          onToggle,
        }}
      />,
    );

    expect(screen.getByText("Verknüpfte Anmeldephasen")).toBeInTheDocument();
    expect(
      screen.getByText("Mit anderem Zeitraum verknüpft"),
    ).toBeInTheDocument();
    expect(screen.getByText("Inaktiv")).toBeInTheDocument();

    const checkboxes = screen.getAllByRole("checkbox");
    // [0] = "Zeitraum im Plan verwenden", [1] = linked phase, [2] = other
    expect(checkboxes[1]).toBeChecked();
    expect(checkboxes[2]).not.toBeChecked();

    // Die Checkbox ändert nur den Entwurf; geschrieben wird mit „Speichern“
    // (#3112, Bauart 2 Regel 4).
    fireEvent.click(checkboxes[1]!);
    expect(checkboxes[1]).not.toBeChecked();
    expect(onToggle).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(onToggle).toHaveBeenCalledWith(
        expect.objectContaining({ id: "7" }),
        false,
      ),
    );
    expect(onToggle).toHaveBeenCalledTimes(1);
    expect(mockUpdate).toHaveBeenCalledTimes(1);
  });

  it("keeps only failed phase links in the draft after saving the period", async () => {
    const onClose = vi.fn();
    const onSaved = vi.fn();
    const onToggle = vi
      .fn()
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error("Zweite Verknüpfung fehlgeschlagen"))
      .mockResolvedValueOnce(undefined);
    render(
      <CalendarPeriodModal
        isOpen
        onClose={onClose}
        onSaved={onSaved}
        initial={period}
        phaseLink={{
          phases: [
            {
              id: "7",
              name: "Demo Anmeldung",
              calendar_period_id: period.id,
              is_active: true,
            },
            {
              id: "8",
              name: "Ferienbetreuung",
              calendar_period_id: period.id,
              is_active: true,
            },
          ],
          onToggle,
        }}
      />,
    );

    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[1]!);
    fireEvent.click(checkboxes[2]!);
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Zweite Verknüpfung fehlgeschlagen",
    );
    expect(onSaved).toHaveBeenCalledWith(period);
    expect(onClose).not.toHaveBeenCalled();
    expect(onToggle).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({ id: "7" }),
      false,
    );
    expect(onToggle).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({ id: "8" }),
      false,
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(onToggle).toHaveBeenCalledTimes(3);
    expect(onToggle).toHaveBeenLastCalledWith(
      expect.objectContaining({ id: "8" }),
      false,
    );
    expect(onSaved).toHaveBeenCalledTimes(1);
  });

  it("does not write a phase link that was toggled back before Speichern", async () => {
    const onToggle = vi.fn().mockResolvedValue(undefined);
    render(
      <CalendarPeriodModal
        isOpen
        onClose={vi.fn()}
        onSaved={vi.fn()}
        initial={period}
        phaseLink={{
          phases: [
            {
              id: "7",
              name: "Demo Anmeldung",
              calendar_period_id: period.id,
              is_active: true,
            },
          ],
          onToggle,
        }}
      />,
    );

    const checkbox = screen.getAllByRole("checkbox")[1]!;
    fireEvent.click(checkbox);
    fireEvent.click(checkbox);
    expect(checkbox).toBeChecked();

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(mockUpdate).toHaveBeenCalledTimes(1));
    expect(onToggle).not.toHaveBeenCalled();
  });

  it("hides the phase link section in create mode", () => {
    render(
      <CalendarPeriodModal
        isOpen
        onClose={vi.fn()}
        onSaved={vi.fn()}
        phaseLink={{
          phases: [
            {
              id: "7",
              name: "Demo Anmeldung",
              calendar_period_id: null,
              is_active: true,
            },
          ],
          onToggle: vi.fn(),
        }}
      />,
    );

    expect(
      screen.queryByText("Verknüpfte Anmeldephasen"),
    ).not.toBeInTheDocument();
  });

  it("names the concrete usage in the delete warning when usage is provided", () => {
    render(
      <CalendarPeriodModal
        isOpen
        onClose={vi.fn()}
        onSaved={vi.fn()}
        onDeleted={vi.fn()}
        initial={period}
        usage={{
          enrollmentPhaseCount: 1,
          activityGroupCount: 2,
          scheduleCount: 3,
          studentEnrollmentCount: 0,
          supervisorCount: 0,
          activityInstanceCount: 0,
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));
    expect(
      screen.getByText(
        /Dieser Zeitraum wird von 1 Anmeldephase und 2 Aktivitätsvorlagen und 3 Regelterminen? verwendet/,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Betreuungsangebot-Verknüpfung ungültig würde/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Personalzuordnungen.*Löschen verhindern/),
    ).toBeInTheDocument();
  });

  it("keeps the modal open and lists warnings when the save overlaps active periods", async () => {
    const onClose = vi.fn();
    const onSaved = vi.fn();
    mockCreate.mockResolvedValueOnce({
      period,
      warnings: [
        {
          code: "overlapping_active_periods",
          message:
            'Der Zeitraum überschneidet sich mit dem aktiven Zeitraum "Schuljahr 2025/2026".',
          overlappingPeriodIds: ["3"],
          overlappingPeriodNames: ["Schuljahr 2025/2026"],
        },
      ],
    });

    render(
      <CalendarPeriodModal
        isOpen
        onClose={onClose}
        onSaved={onSaved}
        createDefaults={{
          name: "Schuljahr 2026/2027",
          startDate: "2026-08-01",
          endDate: "2027-07-31",
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));

    expect(
      await screen.findByText(
        'Der Zeitraum überschneidet sich mit dem aktiven Zeitraum "Schuljahr 2025/2026".',
      ),
    ).toBeInTheDocument();
    expect(onSaved).toHaveBeenCalledWith(period);
    expect(mockToastSuccess).toHaveBeenCalledWith(
      'Kalenderzeitraum "Schuljahr 2026/2027" angelegt',
    );
    expect(onClose).not.toHaveBeenCalled();
    expect(
      screen.queryByRole("button", { name: "Anlegen" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Schließen" }));
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("lets a follow-up correction save as an update after a warning-bearing create", async () => {
    const onClose = vi.fn();
    mockCreate.mockResolvedValueOnce({
      period,
      warnings: [
        {
          code: "overlapping_active_periods",
          message:
            'Der Zeitraum überschneidet sich mit dem aktiven Zeitraum "Schuljahr 2025/2026".',
          overlappingPeriodIds: ["3"],
          overlappingPeriodNames: ["Schuljahr 2025/2026"],
        },
      ],
    });

    render(
      <CalendarPeriodModal
        isOpen
        onClose={onClose}
        onSaved={vi.fn()}
        createDefaults={{
          name: "Schuljahr 2026/2027",
          startDate: "2026-08-01",
          endDate: "2027-07-31",
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));
    await screen.findByRole("button", { name: "Schließen" });

    // Editing the form dismisses the warning footer and re-enables saving …
    fireEvent.change(screen.getByLabelText("Bezeichnung*"), {
      target: { value: "Schuljahr 2026/2027 B" },
    });

    // … and the re-submit updates the just-created period instead of
    // creating a duplicate.
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(mockUpdate).toHaveBeenCalledWith(
        "5",
        expect.objectContaining({ name: "Schuljahr 2026/2027 B" }),
      ),
    );
    expect(mockCreate).toHaveBeenCalledTimes(1);
    expect(mockToastSuccess).toHaveBeenCalledWith(
      'Kalenderzeitraum "Schuljahr 2026/2027" aktualisiert',
    );
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("surfaces API failures", async () => {
    mockCreate.mockRejectedValueOnce(new Error("Periode ueberlappt"));
    render(
      <CalendarPeriodModal
        isOpen
        onClose={vi.fn()}
        onSaved={vi.fn()}
        createDefaults={{
          name: "Schuljahr",
          startDate: "2026-08-01",
          endDate: "2027-07-31",
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Periode ueberlappt",
    );
    // Der Alert oben im Panel ist die einzige Meldung (Bauart 2 Regel 5).
    expect(mockToastError).not.toHaveBeenCalled();
  });
});
