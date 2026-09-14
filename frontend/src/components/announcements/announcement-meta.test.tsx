import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { Announcement } from "~/lib/parent-announcements-api";
import {
  announcementCollectionPath,
  AnnouncementStatusBadge,
  describeReminder,
  kindFromParam,
  kindOf,
  reminderStateOf,
  summarizeTargets,
  targetChips,
} from "./announcement-meta";

const base: Announcement = {
  id: "1",
  title: "Sommerfest",
  body: "Am Freitag.",
  priority: "info",
  requires_acknowledgement: false,
  send_email: false,
  status: "draft",
  active: true,
  created_at: "2026-09-01T10:00:00Z",
  updated_at: "2026-09-01T10:00:00Z",
  targets: [],
  response_type: "none",
  options: [],
  delivery_mode: "standard",
  email_audience: "portal_only",
};

describe("kindOf / kindFromParam", () => {
  it("maps polls, letters and plain announcements to their tab", () => {
    expect(kindOf(base)).toBe("announcement");
    expect(kindOf({ ...base, delivery_mode: "letter" })).toBe("letter");
    expect(kindOf({ ...base, response_type: "single_choice" })).toBe("poll");
  });

  it("reads the tab from the address and falls back to Mitteilungen", () => {
    expect(kindFromParam("umfragen")).toBe("poll");
    expect(kindFromParam("elternbriefe")).toBe("letter");
    expect(kindFromParam("mitteilungen")).toBe("announcement");
    expect(kindFromParam(null)).toBe("announcement");
    expect(kindFromParam("quatsch")).toBe("announcement");
  });

  it("builds the collection path with the tab", () => {
    expect(announcementCollectionPath("poll")).toBe(
      "/parent-announcements?art=umfragen",
    );
  });
});

describe("summarizeTargets", () => {
  it("collapses whole-school targets but keeps pending enrollments visible", () => {
    expect(
      summarizeTargets([
        { target_type: "school_all" },
        { target_type: "class" },
      ]),
    ).toBe("Ganze Schule");
    expect(
      summarizeTargets([
        { target_type: "school_all" },
        { target_type: "pending_enrollment" },
      ]),
    ).toBe("Ganze Schule, Offene Anmeldungen");
  });

  it("counts per target type", () => {
    expect(
      summarizeTargets([
        { target_type: "group", ref_id: "1" },
        { target_type: "group", ref_id: "2" },
        { target_type: "class", ref_text: "1a" },
      ]),
    ).toBe("2 Gruppen, 1 Klassen");
    expect(summarizeTargets([])).toBe("–");
  });
});

describe("targetChips", () => {
  it("labels targets with real names and counts single children", () => {
    expect(
      targetChips(
        [
          { target_type: "group", ref_id: "g1" },
          { target_type: "activity_group", ref_id: "a9" },
          { target_type: "class", ref_text: "2b" },
          { target_type: "student", ref_id: "s1" },
          { target_type: "student", ref_id: "s2" },
        ],
        [{ id: "g1", name: "Füchse" } as never],
        [{ id: "a9", name: "Fußball AG" } as never],
      ),
    ).toEqual(["Füchse", "Fußball AG", "Klasse 2b", "2 einzelne Kinder"]);
  });

  it("falls back to generic labels for unknown references", () => {
    expect(
      targetChips([{ target_type: "group", ref_id: "gone" }], [], []),
    ).toEqual(["Gruppe"]);
  });
});

describe("AnnouncementStatusBadge", () => {
  it("renders the German status label", () => {
    render(<AnnouncementStatusBadge status="published" />);
    expect(screen.getByText("Veröffentlicht")).toBeInTheDocument();
  });
});

describe("describeReminder (#3162)", () => {
  // The test clock is 2026-09-09 12:00 Berlin.
  it("says nothing without a reminder", () => {
    expect(reminderStateOf(base)).toBeNull();
    expect(describeReminder(base)).toBeNull();
  });

  it("announces a planned reminder with its Berlin day and clock", () => {
    const planned = { ...base, reminder_at: "2026-09-24T06:00:00Z" };
    expect(reminderStateOf(planned)).toBe("planned");
    expect(describeReminder(planned)).toBe(
      "Erinnerung am 24.09.2026, 08:00 Uhr",
    );
  });

  it("reports the moment the reminder actually went out", () => {
    const sent = {
      ...base,
      reminder_at: "2026-09-08T06:00:00Z",
      reminder_sent_at: "2026-09-08T06:03:00Z",
    };
    expect(reminderStateOf(sent)).toBe("sent");
    expect(describeReminder(sent)).toBe("Erinnert am 08.09.2026, 08:03 Uhr");
  });

  it("flags a reminder whose moment passed without a send", () => {
    const missed = { ...base, reminder_at: "2026-09-08T06:00:00Z" };
    expect(reminderStateOf(missed)).toBe("missed");
    expect(describeReminder(missed)).toBe(
      "Erinnerung am 08.09.2026, 08:00 Uhr nicht versendet",
    );
  });
});
