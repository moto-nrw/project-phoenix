import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SupervisionStudentGrid } from "./student-grid";
import type { ActiveSupervisionStudent } from "./view-model";
import type { BulkPickupTime } from "~/lib/pickup-schedule-api";

// #3889: without a block there is no list of expected or departed children,
// so the room's search only knows who is in the room. A miss must say so
// instead of looking like a broken search.
describe("SupervisionStudentGrid search miss (#3889)", () => {
  it("says the search only covers the children in the room", () => {
    const student = {
      id: "1",
      name: "Max Muster",
    } as unknown as ActiveSupervisionStudent;

    render(
      <SupervisionStudentGrid
        students={[student]}
        filteredStudents={[]}
        pickupTimesData={undefined}
        arrivalTimesData={undefined}
        trackingData={undefined}
        myGroupIds={[]}
        myGroupRooms={[]}
        now={new Date()}
        onOpenStudent={vi.fn()}
      />,
    );

    expect(screen.getByText("Keine Kinder gefunden")).toBeInTheDocument();
    expect(
      screen.getByText(/Gesucht wird nur unter den Kindern im Raum\./),
    ).toBeInTheDocument();
  });
});

// #3946: a one-day change of the Gehzeit or the Abholart marks the card.
describe("SupervisionStudentGrid one-day change (#3946)", () => {
  it("marks only the children whose way home changes today", () => {
    const changed = {
      id: "1",
      first_name: "Mia",
      second_name: "Schulz",
    } as unknown as ActiveSupervisionStudent;
    const usual = {
      id: "2",
      first_name: "Ben",
      second_name: "Kurz",
    } as unknown as ActiveSupervisionStudent;
    const modeChanged = {
      id: "3",
      first_name: "Lea",
      second_name: "Brandt",
    } as unknown as ActiveSupervisionStudent;
    const pickup = (
      studentId: string,
      isException: boolean,
      dayDepartureMode?: BulkPickupTime["dayDepartureMode"],
    ) =>
      ({
        studentId,
        date: "2026-09-09",
        weekdayName: "Mittwoch",
        pickupTime: "13:30",
        isException,
        dayDepartureMode,
        dayNotes: [],
      }) as BulkPickupTime;

    render(
      <SupervisionStudentGrid
        students={[changed, usual, modeChanged]}
        filteredStudents={[changed, usual, modeChanged]}
        pickupTimesData={
          new Map([
            ["1", pickup("1", true)],
            ["2", pickup("2", false)],
            ["3", pickup("3", false, "pickup")],
          ])
        }
        arrivalTimesData={undefined}
        trackingData={undefined}
        myGroupIds={[]}
        myGroupRooms={[]}
        now={new Date()}
        onOpenStudent={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", {
        name: "Mia Schulz, Gehzeit heute geändert - Details öffnen",
      }),
    ).toHaveAttribute("data-day-home-change", "time");
    expect(
      screen.getByRole("button", {
        name: "Lea Brandt, Abholart heute geändert: Wird abgeholt - Details öffnen",
      }),
    ).toHaveAttribute("data-day-home-change", "mode");
    expect(
      screen.getByRole("button", { name: "Ben Kurz - Details öffnen" }),
    ).not.toHaveAttribute("data-day-home-change");
  });
});
