import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SupervisionStudentGrid } from "./student-grid";
import type { ActiveSupervisionStudent } from "./view-model";

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
