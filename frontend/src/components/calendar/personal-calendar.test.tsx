import { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PersonalCalendar, PersonalCalendarChrome } from "./personal-calendar";
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

// RSVP, export, and management controls now live in the event detail sheet.
// Open it by clicking the event surface (there are desktop + mobile copies;
// either sets the same selected event).
function openEvent(title: string) {
  fireEvent.click(
    screen.getAllByRole("button", { name: new RegExp(title) })[0]!,
  );
}

/**
 * Bedienband und Raster sind zwei Komponenten: das Band sitzt in der
 * Kopfkarte der Seite, das Raster darunter. Beide teilen sich den
 * Wochenend-Schalter, deshalb hält dieser Wrapper den Zustand — genau wie
 * die Kalenderseite.
 */
function CalendarWithChrome(
  props: Readonly<{
    events: CalendarEvent[];
    weekStart: Date;
    onWeekChange: (date: Date) => void;
    onCreate?: () => void;
  }>,
) {
  const [showWeekend, setShowWeekend] = useState(false);
  return (
    <>
      <PersonalCalendarChrome
        events={props.events}
        weekStart={props.weekStart}
        onWeekChange={props.onWeekChange}
        onCreate={props.onCreate}
        showWeekend={showWeekend}
        onShowWeekendChange={setShowWeekend}
      />
      <PersonalCalendar
        events={props.events}
        weekStart={props.weekStart}
        showWeekend={showWeekend}
      />
    </>
  );
}

describe("PersonalCalendar", () => {
  it("renders shift events with the Dienst badge", () => {
    render(
      <PersonalCalendar events={[shift]} weekStart={new Date(2026, 0, 5)} />,
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
        events={[appointment, timetable]}
        weekStart={new Date(2026, 0, 5)}
        onRespond={onRespond}
      />,
    );

    // Den Seitentitel trägt die Kopfkarte der Seite (`TenantPage`), nicht
    // dieses Raster — es gibt hier bewusst keine eigene <h1> mehr.
    expect(screen.queryByRole("heading", { level: 1 })).not.toBeInTheDocument();
    expect(screen.getAllByText("Staff meeting").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Betreuung Gruppe A").length).toBeGreaterThan(0);
    // The pending response badge is shown on the event surface.
    expect(screen.getAllByText("Offen").length).toBeGreaterThan(0);

    // RSVP lives in the detail sheet; it closes after each response.
    openEvent("Staff meeting");
    fireEvent.click(screen.getByRole("button", { name: "Zusagen" }));
    expect(onRespond).toHaveBeenCalledWith("42", "accepted");
    openEvent("Staff meeting");
    fireEvent.click(screen.getByRole("button", { name: "Absagen" }));
    expect(onRespond).toHaveBeenCalledWith("42", "declined");
  });

  it("splits the column when an inflated short event visually overlaps the next one", () => {
    // 30-Minuten-Termin mit RSVP: die gerenderte Karte ist höher als das
    // Zeitfenster und ragt über 10:00 hinaus — der Folgetermin darf sie nicht
    // überdecken, beide müssen sich die Spaltenbreite teilen.
    const shortRsvp: CalendarEvent = {
      ...appointment,
      id: "appointment:3:2026-01-05",
      appointment_id: "3",
      title: "Kurzer RSVP-Termin",
      start_time: "09:00",
      end_time: "09:30",
    };
    const followUp: CalendarEvent = {
      ...timetable,
      id: "timetable:10",
      timetable_id: "10",
      title: "Direkt danach",
      start_date: "2026-01-05",
      end_date: "2026-01-05",
      start_time: "10:00",
      end_time: "11:00",
    };
    render(
      <PersonalCalendar
        events={[shortRsvp, followUp]}
        weekStart={new Date(2026, 0, 5)}
        onRespond={vi.fn()}
      />,
    );

    const shortBlock = screen
      .getAllByText("Kurzer RSVP-Termin")[0]!
      .closest("button");
    const followBlock = screen
      .getAllByText("Direkt danach")[0]!
      .closest("button");
    expect(shortBlock?.style.width).toBe("calc(50% - 4px)");
    expect(followBlock?.style.width).toBe("calc(50% - 4px)");
  });

  it("hides weekend days by default and reveals them via the Sa/So toggle", () => {
    const weekendEvent: CalendarEvent = {
      id: "appointment:2:2026-01-10",
      source: "appointment",
      appointment_id: "2",
      title: "Wochenend-Termin",
      start_date: "2026-01-10",
      end_date: "2026-01-10",
      start_time: "09:00",
      end_time: "10:00",
      all_day: false,
      can_respond: false,
      can_edit: false,
    };
    render(
      <CalendarWithChrome
        events={[appointment, weekendEvent]}
        weekStart={new Date(2026, 0, 5)}
        onWeekChange={vi.fn()}
      />,
    );

    expect(screen.queryByText("Wochenend-Termin")).not.toBeInTheDocument();

    const toggle = screen.getByRole("button", { name: "Sa/So (1)" });
    expect(toggle).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(toggle);

    expect(screen.getAllByText("Wochenend-Termin").length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Sa/So" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("supports week navigation and create action", () => {
    const onWeekChange = vi.fn();
    const onCreate = vi.fn();
    render(
      <CalendarWithChrome
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

  it("marks cancelled events, hides RSVP, and offers .ics download", () => {
    const onRespond = vi.fn();
    const cancelled: CalendarEvent = {
      ...appointment,
      cancelled: true,
    };
    render(
      <PersonalCalendar
        events={[cancelled]}
        weekStart={new Date(2026, 0, 5)}
        onRespond={onRespond}
        icsHrefBase="/api/parent/calendar/appointments"
      />,
    );

    expect(screen.getAllByText("Abgesagt").length).toBeGreaterThan(0);
    // RSVP actions are hidden once cancelled.
    expect(screen.queryByRole("button", { name: "Zusagen" })).toBeNull();
    // Cancelled events are not exportable.
    expect(
      screen.queryByRole("link", { name: "Zum Kalender hinzufügen" }),
    ).toBeNull();
  });

  it("renders an add-to-calendar link for active appointments", () => {
    render(
      <PersonalCalendar
        events={[appointment]}
        weekStart={new Date(2026, 0, 5)}
        icsHrefBase="/api/parent/calendar/appointments"
      />,
    );

    openEvent("Staff meeting");
    const link = screen.getByRole("link", {
      name: "Zum Kalender hinzufügen",
    });
    expect(link).toHaveAttribute(
      "href",
      "/api/parent/calendar/appointments/1/ics",
    );
  });

  it("shows organizer manage actions and wires them", () => {
    const onEdit = vi.fn();
    const onCancel = vi.fn();
    const onDelete = vi.fn();
    const editable: CalendarEvent = { ...appointment, can_edit: true };
    render(
      <PersonalCalendar
        events={[editable]}
        weekStart={new Date(2026, 0, 5)}
        onEdit={onEdit}
        onCancel={onCancel}
        onDelete={onDelete}
      />,
    );

    openEvent("Staff meeting");
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    expect(onEdit).toHaveBeenCalledWith(editable);
    openEvent("Staff meeting");
    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));
    expect(onDelete).toHaveBeenCalledWith(editable);
  });

  it("hides the edit action for cancelled appointments", () => {
    const cancelled: CalendarEvent = {
      ...appointment,
      can_edit: true,
      cancelled: true,
    };
    render(
      <PersonalCalendar
        events={[cancelled]}
        weekStart={new Date(2026, 0, 5)}
        onEdit={vi.fn()}
        onCancel={vi.fn()}
        onDelete={vi.fn()}
      />,
    );

    // The backend rejects edits to cancelled appointments, so the button is gone;
    // Löschen stays available.
    openEvent("Staff meeting");
    expect(screen.queryByRole("button", { name: "Bearbeiten" })).toBeNull();
    expect(screen.getByRole("button", { name: "Löschen" })).toBeInTheDocument();
  });

  it("shows times instead of ganztg. for multi-day timed events in the agenda", () => {
    const multiDayTimed: CalendarEvent = {
      ...appointment,
      id: "appointment:4:2026-01-05",
      appointment_id: "4",
      title: "Klassenfahrt",
      start_date: "2026-01-05",
      end_date: "2026-01-07",
      start_time: "09:00",
      end_time: "15:00",
      all_day: false,
    };
    render(
      <PersonalCalendar
        events={[multiDayTimed]}
        weekStart={new Date(2026, 0, 5)}
      />,
    );

    // The mobile agenda must respect all_day: a timed multi-day appointment
    // renders its start/end times, never the ganztg. label.
    expect(screen.queryByText("ganztg.")).not.toBeInTheDocument();
    expect(screen.getAllByText("09:00").length).toBeGreaterThan(0);
    expect(screen.getAllByText("15:00").length).toBeGreaterThan(0);
  });

  it.each(["week", "month"] as const)(
    "shows the full time range for multi-day timed events in the desktop %s view",
    (viewMode) => {
      const multiDayTimed: CalendarEvent = {
        ...appointment,
        id: "appointment:4:2026-01-05",
        appointment_id: "4",
        title: "Klassenfahrt",
        start_date: "2026-01-05",
        end_date: "2026-01-07",
        start_time: "09:00",
        end_time: "15:00",
        all_day: false,
      };
      render(
        <PersonalCalendar
          events={[multiDayTimed]}
          referenceDate={new Date(2026, 0, 5)}
          viewMode={viewMode}
        />,
      );

      // EventPill renders only on desktop. The mobile agenda uses separate
      // start/end spans, so this exact range proves the desktop surface keeps
      // the time information in both layouts.
      expect(screen.getAllByText("09:00–15:00").length).toBeGreaterThan(0);
    },
  );

  it("shows empty and error states", () => {
    render(
      <PersonalCalendar
        events={[]}
        weekStart={new Date(2026, 0, 5)}
        error="Kalender konnte nicht geladen werden."
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
