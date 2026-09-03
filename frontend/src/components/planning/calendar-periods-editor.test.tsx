import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import type { Phase } from "~/lib/enrollment-phase-api";

const {
  mockListPeriods,
  mockListPhases,
  mockSetPhaseCalendarPeriod,
  mockToastSuccess,
  mockToastError,
} = vi.hoisted(() => ({
  mockListPeriods: vi.fn(),
  mockListPhases: vi.fn(),
  mockSetPhaseCalendarPeriod: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
}));

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

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mockToastSuccess, error: mockToastError }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() }),
}));

vi.mock("~/lib/calendar-period-api", () => ({
  calendarPeriodService: {
    list: mockListPeriods,
  },
}));

vi.mock("~/lib/enrollment-phase-api", () => ({
  listPhases: mockListPhases,
  setPhaseCalendarPeriod: mockSetPhaseCalendarPeriod,
}));

vi.mock("~/components/timetable/calendar-period-modal", () => ({
  CalendarPeriodModal: ({
    isOpen,
    onSaved,
    initial,
    usage,
    phaseLink,
  }: {
    isOpen: boolean;
    onSaved: (period: CalendarPeriod) => void;
    initial?: CalendarPeriod | null;
    usage?: {
      enrollmentPhaseCount: number;
      activityGroupCount: number;
      scheduleCount: number;
      studentEnrollmentCount: number;
      supervisorCount: number;
      activityInstanceCount: number;
    };
    phaseLink?: {
      phases: Phase[];
      onToggle: (phase: Phase, link: boolean) => Promise<void>;
    };
  }) =>
    isOpen ? (
      <div
        data-testid="calendar-period-modal"
        data-initial-name={initial?.name ?? ""}
        data-usage-total={
          usage
            ? usage.enrollmentPhaseCount +
              usage.activityGroupCount +
              usage.scheduleCount +
              usage.studentEnrollmentCount +
              usage.supervisorCount +
              usage.activityInstanceCount
            : 0
        }
      >
        <button type="button" onClick={() => initial && onSaved(initial)}>
          modal-save
        </button>
        <button
          type="button"
          onClick={() => {
            const phase = phaseLink?.phases[0];
            if (phase && phaseLink) void phaseLink.onToggle(phase, true);
          }}
        >
          modal-toggle-phase
        </button>
      </div>
    ) : null,
}));

import { TenantPage } from "~/components/ui/tenant-page";
import {
  CalendarPeriodsActions,
  CalendarPeriodsEditor,
  useCalendarPeriods,
} from "./calendar-periods-editor";

/**
 * Der Kopf (Titel, Statuszeile, Anlegen-Aktionen) liegt seit dem
 * Gerüst-Umbau in der Seite, der Inhaltsblock im Editor. Dieser Host stellt
 * dieselbe Zusammensetzung her wie calendar-periods/page.tsx, damit die Tests
 * weiter das prüfen, was die Seite wirklich rendert.
 */
function CalendarPeriodsHost() {
  const state = useCalendarPeriods();

  return (
    <TenantPage
      title="Kalenderzeiträume"
      stats={state.statusLine}
      statsLoading={state.loading}
      actions={<CalendarPeriodsActions state={state} />}
    >
      <CalendarPeriodsEditor state={state} />
    </TenantPage>
  );
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

function makePeriod(overrides: Partial<CalendarPeriod> = {}): CalendarPeriod {
  return {
    id: "5",
    tenantId: "1",
    name: "Original",
    periodType: "school_year",
    startDate: "2026-08-01",
    endDate: "2027-07-31",
    weekCycleLength: 1,
    weekCycleAnchor: null,
    isActive: true,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    enrollmentPhaseCount: 0,
    activityGroupCount: 0,
    scheduleCount: 0,
    studentEnrollmentCount: 0,
    supervisorCount: 0,
    activityInstanceCount: 0,
    ...overrides,
  };
}

function makePhase(overrides: Partial<Phase> = {}): Phase {
  return {
    id: "7",
    name: "Anmeldephase",
    kind: "school_year",
    service_start_date: "2026-08-01",
    service_end_date: "2027-07-31",
    calendar_period_id: null,
    show_status_reason_to_parent: false,
    care_overflow_mode: "waitlist",
    care_offering_selection_mode: "optional",
    is_active: true,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("CalendarPeriodsEditor", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListPhases.mockResolvedValue([makePhase()]);
    mockSetPhaseCalendarPeriod.mockResolvedValue(makePhase());
  });

  it("shows the mobile type separately without truncating the period range", async () => {
    mockListPeriods.mockResolvedValue([makePeriod({ periodType: "holiday" })]);

    render(<CalendarPeriodsHost />);

    const mobileType = (await screen.findAllByText("Ferien")).find(
      (element) => element.tagName === "P",
    );
    const mobileRange = (
      await screen.findAllByText("01.08.2026 – 31.07.2027")
    ).find((element) => element.tagName === "P");

    expect(mobileType).toBeInTheDocument();
    expect(mobileRange).toHaveClass("break-words");
    expect(mobileRange).not.toHaveClass("truncate");
  });

  it("does not show the create empty state after a failed period load", async () => {
    mockListPeriods.mockRejectedValue(new Error("Zeiträume nicht erreichbar"));

    render(<CalendarPeriodsHost />);

    expect(
      await screen.findByText("Zeiträume nicht erreichbar"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Noch keine Kalenderzeiträume"),
    ).not.toBeInTheDocument();
  });

  it("lets the period name wrap and keeps the action column narrow", async () => {
    mockListPeriods.mockResolvedValue([
      makePeriod({ name: "Sommerferienbetreuung" }),
    ]);

    render(<CalendarPeriodsHost />);

    // Ohne Umbruch an beliebiger Stelle zieht ein langes Wort die
    // Namensspalte über die Tabellenbreite hinaus (320px, #2033).
    const name = await screen.findByText("Sommerferienbetreuung");
    expect(name).toHaveClass("wrap-anywhere");
    expect(name).not.toHaveClass("truncate");
    expect(name.parentElement?.className).not.toMatch(/max-w-\[/);

    const actionCell = screen
      .getByRole("button", { name: "Bearbeiten" })
      .closest("td");
    expect(actionCell).toHaveClass("w-px");
  });

  it("allows header actions to wrap within a constrained content column", async () => {
    mockListPeriods.mockResolvedValue([makePeriod()]);

    render(<CalendarPeriodsHost />);

    const createPeriodButton = await screen.findByRole("button", {
      name: "Zeitraum anlegen",
    });
    const actions = createPeriodButton.parentElement;
    // Statuszeile statt Erklärsatz (Kopfkarten-Sweep): gezählte Zeiträume.
    const description = screen.getByText(/1 Zeitraum · 1 aktiv/);

    // Seit der Umstellung auf die Kopfkarte (PageIntro/SectionCard) umbrechen
    // die Aktionen auf jeder Breite, und der Textblock bleibt schrumpfbar.
    expect(actions).toHaveClass("flex-wrap");
    expect(actions).toHaveClass("shrink-0");
    expect(description.closest(".min-w-0")).not.toBeNull();
  });

  it("keeps the modal mounted while the post-save refresh is in flight", async () => {
    const refresh = deferred<CalendarPeriod[]>();
    mockListPeriods
      .mockResolvedValueOnce([makePeriod()])
      .mockReturnValueOnce(refresh.promise);

    render(<CalendarPeriodsHost />);

    fireEvent.click(await screen.findByRole("button", { name: "Bearbeiten" }));
    expect(screen.getByTestId("calendar-period-modal")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "modal-save" }));

    expect(mockListPeriods).toHaveBeenCalledTimes(2);
    expect(screen.getByTestId("calendar-period-modal")).toBeInTheDocument();
    expect(
      screen.queryByText("Kalenderzeiträume werden geladen..."),
    ).not.toBeInTheDocument();

    refresh.resolve([makePeriod()]);
  });

  it("refreshes usage and phase data without replacing the modal initial period", async () => {
    mockListPeriods
      .mockResolvedValueOnce([makePeriod()])
      .mockResolvedValueOnce([
        makePeriod({
          name: "Server Refresh",
          enrollmentPhaseCount: 1,
          activityGroupCount: 6,
          scheduleCount: 2,
          studentEnrollmentCount: 3,
          supervisorCount: 4,
          activityInstanceCount: 5,
        }),
      ]);
    mockListPhases
      .mockResolvedValueOnce([makePhase()])
      .mockResolvedValueOnce([makePhase({ calendar_period_id: "5" })]);

    render(<CalendarPeriodsHost />);

    fireEvent.click(await screen.findByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(screen.getByRole("button", { name: "modal-toggle-phase" }));

    await waitFor(() =>
      expect(screen.getByTestId("calendar-period-modal")).toHaveAttribute(
        "data-usage-total",
        "21",
      ),
    );
    expect(screen.getByTestId("calendar-period-modal")).toHaveAttribute(
      "data-initial-name",
      "Original",
    );
  });
});
