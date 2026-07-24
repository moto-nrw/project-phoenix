import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockGetParentCalendar,
  mockRespondParentCalendar,
  mockToastSuccess,
  mockToastError,
} = vi.hoisted(() => ({
  mockGetParentCalendar: vi.fn(),
  mockRespondParentCalendar: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
  }),
}));

vi.mock("~/lib/personal-calendar-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/personal-calendar-api")
  >("~/lib/personal-calendar-api");
  return {
    ...actual,
    getParentCalendar: mockGetParentCalendar,
    respondParentCalendar: mockRespondParentCalendar,
  };
});

import ParentCalendarPage from "./page";
import type { CalendarResponse } from "~/lib/personal-calendar-api";

const calendarResponse: CalendarResponse = {
  from: "2026-01-05",
  to: "2026-01-11",
  events: [
    {
      id: "appointment:1:2026-01-05",
      source: "appointment",
      title: "Elterngespräch",
      start_date: "2026-01-05",
      end_date: "2026-01-05",
      start_time: "10:00",
      end_time: "10:30",
      all_day: false,
      response_status: "pending",
      recipient_id: "77",
      can_respond: true,
      can_edit: false,
    },
    {
      id: "timetable:1:9",
      source: "timetable",
      title: "Betreuung Gruppe A",
      student_name: "Lara Beispiel",
      school_name: "Testschule",
      start_date: "2026-01-06",
      end_date: "2026-01-06",
      start_time: "14:00",
      end_time: "16:00",
      all_day: false,
      can_respond: false,
      can_edit: false,
    },
  ],
};

describe("ParentCalendarPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // The page derives its week from `new Date()`; pin the clock into the
    // fixture week so the events fall in the visible range (Date only, so
    // waitFor's real timers keep working). Fake the clock BEFORE any render.
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 0, 7));
    mockGetParentCalendar.mockResolvedValue(calendarResponse);
    mockRespondParentCalendar.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("loads parent calendar events and stores RSVP responses", async () => {
    render(<ParentCalendarPage />);

    expect(await screen.findAllByText("Elterngespräch")).not.toHaveLength(0);
    expect(screen.getAllByText("Betreuung Gruppe A").length).toBeGreaterThan(0);
    expect(mockGetParentCalendar).toHaveBeenCalledWith(
      expect.any(Date),
      expect.any(Date),
    );

    // RSVP now lives in the event detail sheet: open the appointment first.
    fireEvent.click(
      screen.getAllByRole("button", { name: /Elterngespräch/ })[0]!,
    );
    fireEvent.click(screen.getByRole("button", { name: "Absagen" }));

    await waitFor(() =>
      expect(mockRespondParentCalendar).toHaveBeenCalledWith("77", "declined"),
    );
    expect(mockGetParentCalendar).toHaveBeenCalledTimes(2);
    expect(mockToastSuccess).toHaveBeenCalledWith("Termin abgesagt.");
  });

  it("shows load and response errors", async () => {
    mockGetParentCalendar.mockRejectedValueOnce(new Error("backend down"));
    render(<ParentCalendarPage />);

    expect(await screen.findByText("backend down")).toBeInTheDocument();

    mockGetParentCalendar.mockResolvedValue(calendarResponse);
    mockRespondParentCalendar.mockRejectedValueOnce(new Error("no access"));
    render(<ParentCalendarPage />);
    expect(await screen.findAllByText("Elterngespräch")).not.toHaveLength(0);

    fireEvent.click(
      screen.getAllByRole("button", { name: /Elterngespräch/ })[0]!,
    );
    fireEvent.click(screen.getByRole("button", { name: "Zusagen" }));
    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith("no access"),
    );
  });
});
