import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

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
        expect.objectContaining({ start_date: "2026-01-05" }),
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

    fireEvent.change(screen.getByLabelText("Wiederholung"), {
      target: { value: "weekly" },
    });
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
