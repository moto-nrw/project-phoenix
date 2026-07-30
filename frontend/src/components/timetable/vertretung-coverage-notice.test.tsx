import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { StaffScheduleAssignment } from "~/lib/shift-helpers";

import {
  VertretungCoverageNotice,
  buildCoverageNoticeMessage,
} from "./vertretung-coverage-notice";

function makeAssignment(
  overrides: Partial<StaffScheduleAssignment> = {},
): StaffScheduleAssignment {
  return {
    instanceId: "42",
    staffId: "11",
    date: "2026-07-15",
    startTime: "12:00",
    endTime: "14:00",
    activityTitle: "Mensa",
    roomId: "3",
    roomName: "Mensa",
    status: "planned",
    isAbsent: false,
    isSubstitute: false,
    absenceReason: null,
    coverageStatus: "uncovered",
    coverageReason: null,
    uncoveredIntervals: [{ startTime: "12:30", endTime: "14:00" }],
    ...overrides,
  };
}

const STAFF_NAMES = new Map([
  ["11", "Ada Staff"],
  ["12", "Bo Schmidt"],
  ["13", "Cem Yildiz"],
]);

describe("buildCoverageNoticeMessage", () => {
  it("nennt bei einem Einsatz Person und offenen Zeitraum im Singular", () => {
    const message = buildCoverageNoticeMessage(
      [makeAssignment()],
      STAFF_NAMES,
      "an diesem Tag",
      false,
    );

    expect(message).toBe(
      "1 Einsatz an diesem Tag ist nicht durch den Dienstplan abgedeckt: Ada Staff (12:30–14:00).",
    );
  });

  it("fasst ab dem dritten Einsatz zusammen", () => {
    const message = buildCoverageNoticeMessage(
      [
        makeAssignment(),
        makeAssignment({ staffId: "12", instanceId: "43" }),
        makeAssignment({ staffId: "13", instanceId: "44" }),
      ],
      STAFF_NAMES,
      "an diesem Tag",
      false,
    );

    expect(message).toBe(
      "3 Einsätze an diesem Tag sind nicht durch den Dienstplan abgedeckt: " +
        "Ada Staff (12:30–14:00), Bo Schmidt (12:30–14:00) und 1 weiterer Einsatz.",
    );
  });

  it("nennt in der Wochenansicht den Wochentag mit", () => {
    const message = buildCoverageNoticeMessage(
      [makeAssignment({ date: "2026-07-16" })],
      STAFF_NAMES,
      "in dieser Woche",
      true,
    );

    expect(message).toContain("Ada Staff (Do, 12:30–14:00)");
  });

  it("fällt ohne offenes Intervall auf das Einsatzfenster zurück", () => {
    const message = buildCoverageNoticeMessage(
      [makeAssignment({ uncoveredIntervals: [] })],
      STAFF_NAMES,
      "an diesem Tag",
      false,
    );

    expect(message).toContain("Ada Staff (12:00–14:00)");
  });

  it("nutzt den Personal-Platzhalter für unbekannte IDs", () => {
    const message = buildCoverageNoticeMessage(
      [makeAssignment({ staffId: "999" })],
      STAFF_NAMES,
      "an diesem Tag",
      false,
    );

    expect(message).toContain("Personal #999");
  });
});

describe("VertretungCoverageNotice", () => {
  it("rendert nichts ohne nicht abgedeckte Einsätze", () => {
    const { container } = render(
      <VertretungCoverageNotice
        assignments={[]}
        staffNames={STAFF_NAMES}
        scopeLabel="an diesem Tag"
        withWeekday={false}
        dienstplanHref="/acme/dienstplan?d=2026-07-15"
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("verlinkt mit 'Dienstplan öffnen' auf den Dienstplan", () => {
    render(
      <VertretungCoverageNotice
        assignments={[makeAssignment()]}
        staffNames={STAFF_NAMES}
        scopeLabel="an diesem Tag"
        withWeekday={false}
        dienstplanHref="/acme/dienstplan?d=2026-07-15"
      />,
    );

    const link = screen.getByRole("link", { name: "Dienstplan öffnen" });
    expect(link).toHaveAttribute("href", "/acme/dienstplan?d=2026-07-15");
  });

  it("meldet höflich, nicht assertiv — es ist keine Störung", () => {
    render(
      <VertretungCoverageNotice
        assignments={[makeAssignment()]}
        staffNames={STAFF_NAMES}
        scopeLabel="an diesem Tag"
        withWeekday={false}
        dienstplanHref="/acme/dienstplan"
      />,
    );

    expect(screen.getByRole("status")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
