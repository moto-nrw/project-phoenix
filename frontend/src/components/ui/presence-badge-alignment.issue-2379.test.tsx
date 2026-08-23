import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LocationBadge } from "./location-badge";
import { PresenceBadge } from "./presence-badge";

const student = {
  current_location: "Anwesend - Gruppenraum",
  not_arrival_today: true,
};

describe.each([
  [
    "detailed presence badge",
    <LocationBadge key="detailed" student={student} displayMode="roomName" />,
  ],
  [
    "simple detailed presence badge",
    <LocationBadge
      key="simple-detailed"
      student={student}
      displayMode="roomName"
      variant="simple"
    />,
  ],
  ["binary presence badge", <PresenceBadge key="binary" student={student} />],
  [
    "simple binary presence badge",
    <PresenceBadge key="simple-binary" student={student} variant="simple" />,
  ],
])("%s", (_name, badge) => {
  it("right-aligns stacked status pills", () => {
    const { container } = render(badge);

    expect(container.firstElementChild).toHaveClass("items-end");
  });
});
