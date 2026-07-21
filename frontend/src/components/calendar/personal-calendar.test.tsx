import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PersonalCalendar } from "./personal-calendar";
import type { CalendarEvent } from "~/lib/personal-calendar-api";

const appointment: CalendarEvent = {
  id: "appointment:1:2026-01-05",
  source: "appointment",
  appointment_id: "1",
  occurrence_date: "2026-01-05",
  title: "Staff meeting",
  description: "Discuss weekly planning",
  location: "Room 1",
  start_date: "2026-01-05",
  end_date: "2026-01-05",
  start_time: "09:00",
  end_time: "10:00",
  all_day: false,
  delivery_mode: "rsvp_required",
  response_status: "pending",
  recipient_id: "42",
  can_respond: true,
  can_edit: false,
};

const timetable: CalendarEvent = {
  id: "timetable:9",
  source: "timetable",
  timetable_id: "9",
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
};

const shift: CalendarEvent = {
  id: "shift:5",
  source: "shift",
  title: "Frühdienst",
  description: "Vertretung Gruppe B",
  start_date: "2026-01-07",
  end_date: "2026-01-07",
  start_time: "08:00",
  end_time: "16:00",
  all_day: false,
  can_respond: false,
  can_edit: false,
};

describe("PersonalCalendar", () => {
  it("renders shift events with the Dienst badge", () => {
    render(
      <PersonalCalendar
        title="Mein Kalender"
        subtitle="Termine und Dienstplan"
        events={[shift]}
        weekStart={new Date(2026, 0, 5)}
        onWeekChange={vi.fn()}
      />,
    );

    expect(screen.getAllByText("Frühdienst").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Dienst").length).toBeGreaterThan(0);
    expect(
      screen.queryByRole("button", { name: "Zusagen" }),
    ).not.toBeInTheDocument();
  });

  it("renders appointment and timetable events with RSVP actions", () => {
    const onRespond = vi.fn();
    render(
      <PersonalCalendar
        title="Mein Kalender"
        subtitle="Termine und Betreuung"
        events={[appointment, timetable]}
        weekStart={new Date(2026, 0, 5)}
        onWeekChange={vi.fn()}
        onRespond={onRespond}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Mein Kalender" }),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Staff meeting").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Betreuung Gruppe A").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Termin").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Betreuung").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Offen").length).toBeGreaterThan(0);

    fireEvent.click(screen.getAllByRole("button", { name: "Zusagen" })[0]!);
    expect(onRespond).toHaveBeenCalledWith("42", "accepted");
    fireEvent.click(screen.getAllByRole("button", { name: "Absagen" })[0]!);
    expect(onRespond).toHaveBeenCalledWith("42", "declined");
  });

  it("supports week navigation and create action", () => {
    const onWeekChange = vi.fn();
    const onCreate = vi.fn();
    render(
      <PersonalCalendar
        title="Mein Kalender"
        events={[]}
        weekStart={new Date(2026, 0, 5)}
        onWeekChange={onWeekChange}
        onCreate={onCreate}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Nächste Woche" }));
    expect(onWeekChange).toHaveBeenCalledWith(new Date(2026, 0, 12));

    fireEvent.click(screen.getByRole("button", { name: "Vorherige Woche" }));
    expect(onWeekChange).toHaveBeenCalledWith(new Date(2025, 11, 29));

    fireEvent.click(screen.getByRole("button", { name: "Neuer Termin" }));
    expect(onCreate).toHaveBeenCalledOnce();
  });

  it("shows empty and error states", () => {
    render(
      <PersonalCalendar
        title="Familienkalender"
        events={[]}
        weekStart={new Date(2026, 0, 5)}
        error="Kalender konnte nicht geladen werden."
        onWeekChange={vi.fn()}
      />,
    );

    expect(
      screen.getByText("Kalender konnte nicht geladen werden."),
    ).toBeInTheDocument();
    expect(
      screen.getAllByText("Keine Einträge in dieser Woche.").length,
    ).toBeGreaterThan(0);
  });
});
