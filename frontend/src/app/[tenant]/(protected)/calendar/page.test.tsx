import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// The date fields moved from native inputs to the kit picker; this stub keeps
// them settable via fireEvent.change and forwards min/max so the bound
// assertions below still pin what the component computes. Imported inside the
// factory because vi.mock is hoisted above the imports.
vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

const {
  mockUseSWRAuth,
  mockCreateStaffAppointment,
  mockUpdateStaffAppointment,
  mockGetStaffAppointmentDetail,
  mockRespondStaffCalendar,
  mockToastSuccess,
  mockToastError,
  mockToastWarning,
  mockMutate,
  mockUseSession,
} = vi.hoisted(() => ({
  mockUseSWRAuth: vi.fn(),
  mockCreateStaffAppointment: vi.fn(),
  mockUpdateStaffAppointment: vi.fn(),
  mockGetStaffAppointmentDetail: vi.fn(),
  mockRespondStaffCalendar: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
  mockToastWarning: vi.fn(),
  mockMutate: vi.fn(),
  mockUseSession: vi.fn(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: mockUseSWRAuth,
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
    warning: mockToastWarning,
  }),
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

// Der Betreuungsplan-Tab (#2283) lädt die große Planner-View dynamisch;
// für die Seiten-Tests genügt ein Platzhalter.
// useSearchParams liefert außerhalb des App-Routers null; der Tab-Start hängt
// an den Plan-Parametern (#2621), also wird der Hook hier steuerbar gemacht.
const { mockUseSearchParams } = vi.hoisted(() => ({
  mockUseSearchParams: vi.fn<() => URLSearchParams | null>(() => null),
}));
vi.mock("next/navigation", async (importOriginal) => {
  const actual = await importOriginal<typeof import("next/navigation")>();
  return { ...actual, useSearchParams: () => mockUseSearchParams() };
});

vi.mock("~/components/timetable/betreuungsplan-view", () => ({
  BetreuungsplanView: () => <div data-testid="school-plan-view" />,
}));

vi.mock("~/lib/personal-calendar-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/personal-calendar-api")
  >("~/lib/personal-calendar-api");
  return {
    ...actual,
    getStaffCalendar: vi.fn(),
    getCalendarRecipientOptions: vi.fn(),
    createStaffAppointment: mockCreateStaffAppointment,
    updateStaffAppointment: mockUpdateStaffAppointment,
    getStaffAppointmentDetail: mockGetStaffAppointmentDetail,
    respondStaffCalendar: mockRespondStaffCalendar,
  };
});

import StaffCalendarPage from "./page";
import type {
  CalendarRecipientOptions,
  CalendarResponse,
} from "~/lib/personal-calendar-api";

const calendarResponse: CalendarResponse = {
  from: "2026-01-05",
  to: "2026-01-11",
  events: [
    {
      id: "appointment:1:2026-01-05",
      source: "appointment",
      title: "Teamplanung",
      start_date: "2026-01-05",
      end_date: "2026-01-05",
      start_time: "09:00",
      end_time: "10:00",
      all_day: false,
      response_status: "pending",
      recipient_id: "42",
      can_respond: true,
      can_edit: false,
    },
  ],
};

const recipientOptions: CalendarRecipientOptions = {
  staff: [{ id: "7", name: "Anna Mitarbeiterin" }],
  parents: [{ id: "8", name: "Sabine Elternteil" }],
  groups: [{ id: "9", name: "Gruppe A" }],
  classes: ["1a"],
  students: [{ id: "10", name: "Lara Beispiel", school_class: "1a" }],
};

function mockCalendarSWR() {
  mockUseSWRAuth.mockImplementation((key: unknown) => {
    const cacheKey = typeof key === "string" ? key : "";
    if (cacheKey.startsWith("calendar-recipient-options")) {
      return { data: recipientOptions, isLoading: false };
    }
    if (cacheKey.startsWith("staff-calendar")) {
      return {
        data: calendarResponse,
        error: null,
        isLoading: false,
        mutate: mockMutate,
      };
    }
    return { data: undefined, error: null, isLoading: false, mutate: vi.fn() };
  });
}

describe("StaffCalendarPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSearchParams.mockReturnValue(null);
    // The page derives its week from `new Date()`; pin the clock into the
    // fixture week so events fall in the visible range (Date only, so waitFor's
    // real timers keep working). Fake the clock BEFORE any render.
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 0, 7));
    mockCalendarSWR();
    mockUseSession.mockReturnValue({
      data: { user: { permissions: ["calendar:own", "calendar:manage"] } },
      status: "authenticated",
    });
    mockRespondStaffCalendar.mockResolvedValue(undefined);
    mockCreateStaffAppointment.mockResolvedValue({ appointment: { id: "1" } });
    mockUpdateStaffAppointment.mockResolvedValue({ appointment: { id: "5" } });
    mockMutate.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows the personal calendar subscription below the calendar", () => {
    render(<StaffCalendarPage />);

    expect(
      screen.getByRole("heading", { name: "Kalender abonnieren" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Neue, geänderte und abgesagte Termine/),
    ).toBeInTheDocument();
  });

  it("keeps the calendar subscription available when the calendar fails", () => {
    mockUseSWRAuth.mockImplementation((key: unknown) => {
      const cacheKey = typeof key === "string" ? key : "";
      if (cacheKey.startsWith("calendar-recipient-options")) {
        return { data: recipientOptions, isLoading: false };
      }
      if (cacheKey.startsWith("staff-calendar")) {
        return {
          data: undefined,
          error: new Error("calendar failed"),
          isLoading: false,
          mutate: mockMutate,
        };
      }
      return {
        data: undefined,
        error: null,
        isLoading: false,
        mutate: vi.fn(),
      };
    });

    render(<StaffCalendarPage />);

    expect(
      screen.getByRole("heading", { name: "Kalender abonnieren" }),
    ).toBeInTheDocument();
    expect(screen.getByText("calendar failed")).toBeInTheDocument();
    expect(
      screen.queryByText("Keine Einträge in dieser Woche."),
    ).not.toBeInTheDocument();
  });

  it("edits a recurring appointment using the series base date, not the clicked occurrence", async () => {
    // A recurring appointment opened from its THIRD weekly occurrence
    // (2026-01-19); the persisted series anchor is 2026-01-05.
    mockUseSWRAuth.mockImplementation((key: unknown) => {
      const cacheKey = typeof key === "string" ? key : "";
      if (cacheKey.startsWith("calendar-recipient-options")) {
        return { data: recipientOptions, isLoading: false };
      }
      if (cacheKey.startsWith("staff-calendar")) {
        return {
          data: {
            from: "2026-01-19",
            to: "2026-01-25",
            events: [
              {
                id: "appointment:5:2026-01-19",
                source: "appointment",
                appointment_id: "5",
                occurrence_date: "2026-01-19",
                title: "Wöchentliche AG",
                start_date: "2026-01-19",
                end_date: "2026-01-19",
                start_time: "14:00",
                end_time: "15:00",
                all_day: false,
                recurring: true,
                can_respond: false,
                can_edit: true,
              },
            ],
          },
          error: null,
          isLoading: false,
          mutate: mockMutate,
        };
      }
      return {
        data: undefined,
        error: null,
        isLoading: false,
        mutate: vi.fn(),
      };
    });
    mockGetStaffAppointmentDetail.mockResolvedValue({
      appointment: {
        title: "Wöchentliche AG",
        start_date: "2026-01-05",
        end_date: "2026-01-05",
        all_day: false,
        overview_visibility: "organizer",
        delivery_mode: "informational",
        notify_guardians: true,
      },
      recurrence: {
        frequency: "weekly",
        interval_count: 1,
        weekdays: ["monday"],
      },
    });

    // This fixture's occurrence sits in the week of 2026-01-19.
    vi.setSystemTime(new Date(2026, 0, 21));
    render(<StaffCalendarPage />);

    // Management actions now live in the event detail sheet: open it first.
    fireEvent.click(
      screen.getAllByRole("button", { name: /Wöchentliche AG/ })[0]!,
    );
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    // The modal opens once the detail resolves; the date input shows the BASE
    // series date, not the clicked occurrence date.
    const startInput = (await screen.findByLabelText(
      "Startdatum",
    )) as HTMLInputElement;
    expect(startInput.value).toBe("2026-01-05");

    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() =>
      expect(mockUpdateStaffAppointment).toHaveBeenCalledWith(
        "5",
        expect.objectContaining({ start_date: "2026-01-05", send_email: true }),
      ),
    );
    // Never sends the occurrence date as the new series anchor.
    expect(mockUpdateStaffAppointment).not.toHaveBeenCalledWith(
      "5",
      expect.objectContaining({ start_date: "2026-01-19" }),
    );
  });

  it("renders events and stores RSVP responses", async () => {
    render(<StaffCalendarPage />);

    expect(screen.getAllByText("Teamplanung").length).toBeGreaterThan(0);
    fireEvent.click(screen.getAllByRole("button", { name: /Teamplanung/ })[0]!);
    fireEvent.click(screen.getByRole("button", { name: "Zusagen" }));

    await waitFor(() =>
      expect(mockRespondStaffCalendar).toHaveBeenCalledWith("42", "accepted"),
    );
    expect(mockMutate).toHaveBeenCalledOnce();
    expect(mockToastSuccess).toHaveBeenCalledWith("Termin zugesagt.");
  });

  it("creates a weekly recurring appointment for selected staff", async () => {
    render(<StaffCalendarPage />);

    fireEvent.click(screen.getByRole("button", { name: "Neuer Termin" }));
    fireEvent.change(screen.getByLabelText("Titel"), {
      target: { value: "Elterngespräch" },
    });
    fireEvent.click(screen.getByLabelText("Anna Mitarbeiterin"));
    expect(
      screen.getByText("Mitarbeitende: Anna Mitarbeiterin"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("Wiederholung"));
    fireEvent.click(screen.getByRole("option", { name: "Wöchentlich" }));
    fireEvent.click(screen.getByLabelText("Mo"));
    fireEvent.click(screen.getByRole("button", { name: "Termin speichern" }));

    await waitFor(() =>
      expect(mockCreateStaffAppointment).toHaveBeenCalledWith(
        expect.objectContaining({
          title: "Elterngespräch",
          start_date: expect.any(String),
          end_date: expect.any(String),
          delivery_mode: "rsvp_required",
          targets: [{ type: "staff", id: "7" }],
          recurrence: expect.objectContaining({
            frequency: "weekly",
            interval_count: 1,
            weekdays: ["monday"],
          }),
        }),
      ),
    );
    expect(mockToastSuccess).toHaveBeenCalledWith("Termin wurde erstellt.");
    expect(mockMutate).toHaveBeenCalled();
  });

  it("requires a target before creating an appointment", async () => {
    render(<StaffCalendarPage />);

    fireEvent.click(screen.getByRole("button", { name: "Neuer Termin" }));
    fireEvent.change(screen.getByLabelText("Titel"), {
      target: { value: "Ohne Ziel" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Termin speichern" }));

    expect(mockCreateStaffAppointment).not.toHaveBeenCalled();
    expect(mockToastWarning).toHaveBeenCalledWith(
      "Bitte mindestens ein Ziel auswählen.",
    );
  });

  it("requires both dates before creating an appointment", async () => {
    render(<StaffCalendarPage />);

    fireEvent.click(screen.getByRole("button", { name: "Neuer Termin" }));
    fireEvent.change(screen.getByLabelText("Titel"), {
      target: { value: "Ohne Startdatum" },
    });
    fireEvent.click(screen.getByLabelText("Anna Mitarbeiterin"));
    fireEvent.change(screen.getByLabelText("Startdatum"), {
      target: { value: "" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Termin speichern" }));

    expect(mockCreateStaffAppointment).not.toHaveBeenCalled();
    expect(mockToastWarning).toHaveBeenCalledWith(
      "Bitte Start- und Enddatum angeben.",
    );
  });

  // Eine Kalenderfläche (#2283): Nicht-Admins mit schedules:read bekommen
  // den Betreuungsplan als zweiten Tab; Admins und Konten ohne die
  // Permission sehen den Kalender unverändert ohne Tabs.
  it("shows the Betreuungsplan tab for non-admin staff with schedules:read", () => {
    mockUseSession.mockReturnValue({
      data: { user: { permissions: ["calendar:own", "schedules:read"] } },
      status: "authenticated",
    });

    render(<StaffCalendarPage />);

    expect(screen.getByRole("tab", { name: "Meine Termine" })).toBeVisible();
    expect(screen.getByRole("tab", { name: "Betreuungsplan" })).toBeVisible();
    // Ohne Plan-Parameter startet die Fläche bei den eigenen Terminen.
    expect(screen.getByRole("tab", { name: "Meine Termine" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("öffnet den Betreuungsplan-Tab bei einem geteilten Plan-Link (#2621)", () => {
    mockUseSession.mockReturnValue({
      data: { user: { permissions: ["calendar:own", "schedules:read"] } },
      status: "authenticated",
    });
    mockUseSearchParams.mockReturnValue(
      new URLSearchParams("view=tag&d=2026-08-24"),
    );

    render(<StaffCalendarPage />);

    expect(screen.getByRole("tab", { name: "Betreuungsplan" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("follows Plan-Parameter after client-side navigation", async () => {
    mockUseSession.mockReturnValue({
      data: { user: { permissions: ["calendar:own", "schedules:read"] } },
      status: "authenticated",
    });
    const { rerender } = render(<StaffCalendarPage />);

    mockUseSearchParams.mockReturnValue(
      new URLSearchParams("view=tag&d=2026-08-24"),
    );
    rerender(<StaffCalendarPage />);

    await waitFor(() =>
      expect(
        screen.getByRole("tab", { name: "Betreuungsplan" }),
      ).toHaveAttribute("aria-selected", "true"),
    );

    mockUseSearchParams.mockReturnValue(null);
    rerender(<StaffCalendarPage />);

    await waitFor(() =>
      expect(
        screen.getByRole("tab", { name: "Meine Termine" }),
      ).toHaveAttribute("aria-selected", "true"),
    );
  });

  // #2957: Tab von Hand wählen, Termin öffnen, Termin schließen — die Fläche
  // darf dabei nicht auf "Meine Termine" zurückspringen. Das Schließen räumt
  // `block` ab; ohne den beim Tab-Wechsel gesetzten Tag stünde die URL danach
  // ohne Plan-Parameter da.
  it("hält den Betreuungsplan-Tab über Öffnen und Schließen eines Termins (#2957)", async () => {
    mockUseSession.mockReturnValue({
      data: { user: { permissions: ["calendar:own", "schedules:read"] } },
      status: "authenticated",
    });
    window.history.replaceState(null, "", "/acme/kalender");
    mockUseSearchParams.mockImplementation(
      () => new URLSearchParams(window.location.search),
    );

    const { rerender } = render(<StaffCalendarPage />);

    // Die Seitenreiter des Tenant-Gerüsts sind einfache Buttons (click),
    // keine Radix-Tabs mehr (mousedown).
    fireEvent.click(screen.getByRole("tab", { name: "Betreuungsplan" }));
    rerender(<StaffCalendarPage />);

    await waitFor(() =>
      expect(
        screen.getByRole("tab", { name: "Betreuungsplan" }),
      ).toHaveAttribute("aria-selected", "true"),
    );
    const dayParam = new URLSearchParams(window.location.search).get("d");
    expect(dayParam).toMatch(/^\d{4}-\d{2}-\d{2}$/);

    // Termin öffnen: der Betreuungsplan schreibt `block` dazu.
    window.history.replaceState(null, "", `?d=${dayParam}&block=42`);
    rerender(<StaffCalendarPage />);
    expect(screen.getByRole("tab", { name: "Betreuungsplan" })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    // Termin schließen: `block` verschwindet, der Tag bleibt.
    window.history.replaceState(null, "", `?d=${dayParam}`);
    rerender(<StaffCalendarPage />);

    expect(screen.getByRole("tab", { name: "Betreuungsplan" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(new URLSearchParams(window.location.search).get("d")).toBe(dayParam);
  });

  it("clears Plan-Parameter when switching to Meine Termine", () => {
    mockUseSession.mockReturnValue({
      data: { user: { permissions: ["calendar:own", "schedules:read"] } },
      status: "authenticated",
    });
    window.history.replaceState(null, "", "?view=tag&d=2026-08-24");
    mockUseSearchParams.mockImplementation(
      () => new URLSearchParams(window.location.search),
    );

    render(<StaffCalendarPage />);
    expect(screen.getByRole("tab", { name: "Betreuungsplan" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    fireEvent.click(screen.getByRole("tab", { name: "Meine Termine" }));

    expect(window.location.search).toBe("");
    expect(screen.getByRole("tab", { name: "Meine Termine" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("shows no tabs for admins", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          roles: ["admin"],
          permissions: ["calendar:own", "schedules:read", "admin:*"],
        },
      },
      status: "authenticated",
    });

    render(<StaffCalendarPage />);

    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
  });

  it("shows no tabs without schedules:read", () => {
    mockUseSession.mockReturnValue({
      data: { user: { permissions: ["calendar:own"] } },
      status: "authenticated",
    });

    render(<StaffCalendarPage />);

    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
  });

  it("hides creation controls without calendar manage permission", () => {
    mockUseSession.mockReturnValue({
      data: { user: { permissions: ["calendar:own"] } },
      status: "authenticated",
    });

    render(<StaffCalendarPage />);

    expect(
      screen.queryByRole("button", { name: "Neuer Termin" }),
    ).not.toBeInTheDocument();
    expect(mockUseSWRAuth).not.toHaveBeenCalledWith(
      expect.stringMatching(/^calendar-recipient-options/),
      expect.any(Function),
    );
  });
});
