import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LocationBadge } from "./location-badge";
import { PresenceBadge } from "./presence-badge";

const student = {
  current_location: "Anwesend - Gruppenraum",
  not_arrival_today: true,
  not_arrival_reason: "Kommt heute nicht",
};

describe.each([
  [
    "detailed presence badge",
    <LocationBadge key="detailed" student={student} displayMode="roomName" />,
    "Gruppenraum",
  ],
  [
    "binary presence badge",
    <PresenceBadge key="binary" student={student} />,
    "Anwesend",
  ],
])("%s", (_name, badge, expectedLocation) => {
  it("keeps the actual location primary and shows the contradiction", () => {
    render(badge);

    expect(screen.getByText(expectedLocation)).toBeInTheDocument();
    expect(screen.getByText("Ungeplant anwesend")).toBeInTheDocument();
    expect(screen.queryByText("Kommt heute nicht")).not.toBeInTheDocument();
  });
});

it("shows the contradiction instead of the underlying absence reason", () => {
  render(
    <LocationBadge
      student={{ ...student, sick: true }}
      displayMode="roomName"
    />,
  );

  expect(screen.getByText("Gruppenraum")).toBeInTheDocument();
  expect(screen.getByText("Ungeplant anwesend")).toBeInTheDocument();
  expect(screen.queryByText("Krank")).not.toBeInTheDocument();
});
