import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LocationBadge } from "./location-badge";
import { PresenceBadge } from "./presence-badge";
import type { PresenceStudent } from "~/lib/presence-status";
import { LOCATION_STATUSES } from "~/lib/location-helper";

// #6572: the badges read today's plan from the child as it is loaded; no
// caller mixes `getStudentPresenceBadgePlanning` in by hand any more.
describe.each([
  [
    "LocationBadge",
    (student: PresenceStudent) => (
      <LocationBadge student={student} displayMode="roomName" />
    ),
  ],
  [
    "PresenceBadge",
    (student: PresenceStudent) => <PresenceBadge student={student} />,
  ],
])("%s", (_name, renderBadge) => {
  it("shows unplanned attendance from the day planning fields", () => {
    render(
      renderBadge({
        current_location: "Anwesend - Raum 101",
        day_planning_status: "comes_today",
        day_planning_reason: "unplanned_attendance",
        day_planning_label: "laut Plan nicht da",
      }),
    );

    expect(
      screen.getByText(LOCATION_STATUSES.UNPLANNED_PRESENT),
    ).toHaveAttribute("title", "laut Plan nicht da");
  });

  it("shows a planned absence at home from the day planning fields", () => {
    render(
      renderBadge({
        current_location: "Zuhause",
        day_planning_status: "not_coming_today",
        day_planning_label: "Arzttermin",
      }),
    );

    expect(screen.getByText(LOCATION_STATUSES.NOT_ARRIVAL)).toHaveAttribute(
      "title",
      "Arzttermin",
    );
    expect(screen.queryByText(LOCATION_STATUSES.HOME)).not.toBeInTheDocument();
  });
});

// #6663: the two badges disagree on the tooltip of a pill that replaced
// "Kommt heute nicht"; both keep their behaviour until it is decided.
describe("tooltip of a pill over a planned absence", () => {
  const student: PresenceStudent = {
    current_location: "Zuhause",
    sick: true,
    arrival_is_exception: true,
    arrival_notes: "Arzttermin",
  };

  it("LocationBadge shows the plan's reason on the Krank pill", () => {
    render(<LocationBadge student={student} displayMode="roomName" />);

    expect(screen.getByText(LOCATION_STATUSES.SICK)).toHaveAttribute(
      "title",
      "Arzttermin",
    );
  });

  it("PresenceBadge leaves the Krank pill without a tooltip", () => {
    render(<PresenceBadge student={student} />);

    expect(screen.getByText(LOCATION_STATUSES.SICK)).not.toHaveAttribute(
      "title",
    );
  });
});
