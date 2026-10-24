import { describe, expect, it } from "vitest";

import type { SchoolCalendarResponse } from "~/lib/personal-calendar-api";
import {
  emptySchoolAppointmentDraft,
  schoolAppointmentDraftFromEvent,
  schoolAppointmentFieldErrors,
  schoolAppointmentRequest,
  schoolCalendarEvents,
} from "./school-calendar";

const response: SchoolCalendarResponse = {
  from: "2026-09-07",
  to: "2026-09-13",
  events: [
    {
      id: "appointment:3:2026-09-09",
      source: "appointment",
      appointment_id: "3",
      title: "Zoo-Ausflug",
      start_date: "2026-09-09",
      end_date: "2026-09-09",
      start_time: "08:30",
      end_time: "12:00",
      all_day: false,
      school_event_type: "ausflug",
      location: "Zoo",
      can_respond: false,
      can_edit: true,
    },
  ],
  closing_days: [
    {
      id: "4",
      name: "Brückentag",
      start_date: "2026-09-11",
      end_date: "2026-09-11",
    },
  ],
  holidays: [
    {
      id: "2",
      name: "Herbstferien",
      start_date: "2026-10-12",
      end_date: "2026-10-24",
    },
  ],
  classes: ["3b"],
};

describe("schoolCalendarEvents", () => {
  it("adds closing days and holidays as all-day entries", () => {
    const events = schoolCalendarEvents(response);

    expect(events.map((event) => event.id)).toEqual([
      "appointment:3:2026-09-09",
      "closing_day:4",
      "holiday:2",
    ]);
    expect(events[2]).toMatchObject({
      source: "holiday",
      title: "Herbstferien",
      start_date: "2026-10-12",
      end_date: "2026-10-24",
      all_day: true,
      can_edit: false,
      can_respond: false,
    });
  });

  it("returns no entries before the first answer", () => {
    expect(schoolCalendarEvents(undefined)).toEqual([]);
  });
});

describe("schoolAppointmentFieldErrors", () => {
  it("asks for a class when the parents of own classes are chosen", () => {
    const draft = {
      ...emptySchoolAppointmentDraft("2026-09-09"),
      title: "Ausflug",
      parents: "classes" as const,
    };

    expect(schoolAppointmentFieldErrors(draft, true)).toEqual({
      classes: "Bitte wählen Sie mindestens eine Klasse.",
    });
    // Beim Ändern liest das Backend die Eltern nicht.
    expect(schoolAppointmentFieldErrors(draft, false)).toEqual({});
  });

  it("rejects an end before the start on one day", () => {
    const draft = {
      ...emptySchoolAppointmentDraft("2026-09-09"),
      title: "Unterrichtsende",
      startTime: "12:00",
      endTime: "11:00",
    };

    expect(schoolAppointmentFieldErrors(draft, true)).toEqual({
      end_time: "Das Ende muss nach dem Beginn liegen.",
    });
  });

  it("names a missing title", () => {
    expect(
      schoolAppointmentFieldErrors(
        emptySchoolAppointmentDraft("2026-09-09"),
        true,
      ),
    ).toEqual({ title: "Bitte geben Sie einen Titel ein." });
  });
});

describe("schoolAppointmentRequest", () => {
  it("sends the chosen classes and the full day for an all-day entry", () => {
    const body = schoolAppointmentRequest({
      ...emptySchoolAppointmentDraft("2026-09-09"),
      title: "  Pädagogischer Tag  ",
      eventType: "paedagogischer_tag",
      allDay: true,
      parents: "classes",
      classes: ["3b"],
    });

    expect(body).toEqual({
      title: "Pädagogischer Tag",
      description: undefined,
      location: undefined,
      start_date: "2026-09-09",
      end_date: "2026-09-09",
      start_time: "00:00",
      end_time: "23:59",
      all_day: true,
      school_event_type: "paedagogischer_tag",
      parents: "classes",
      classes: ["3b"],
    });
  });

  it("drops classes unless the parents of own classes are chosen", () => {
    const body = schoolAppointmentRequest({
      ...emptySchoolAppointmentDraft("2026-09-09"),
      title: "Ausflug",
      parents: "school",
      classes: ["3b"],
    });

    expect(body.parents).toBe("school");
    expect(body.classes).toBeUndefined();
  });

  it("fills the edit form from an own Schultermin", () => {
    const draft = schoolAppointmentDraftFromEvent(response.events[0]!);

    expect(draft).toMatchObject({
      title: "Zoo-Ausflug",
      eventType: "ausflug",
      startTime: "08:30",
      endTime: "12:00",
      location: "Zoo",
      parents: "none",
    });
  });
});
