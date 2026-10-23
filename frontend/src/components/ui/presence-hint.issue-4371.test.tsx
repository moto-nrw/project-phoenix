import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LocationBadge } from "./location-badge";
import { PresenceBadge } from "./presence-badge";
import { LOCATION_STATUSES } from "~/lib/location-helper";

// #4371: a child with a location AND an exception shows exactly one pill (the
// location); the exception stays readable as a text line below it.
describe("one pill per child", () => {
  it("shows Unterwegs as the pill and Ungeplant anwesend as text", () => {
    const { container } = render(
      <LocationBadge
        student={{
          current_location: "Unterwegs",
          not_arrival_today: true,
          not_arrival_reason: "Laut Plan kommt das Kind heute nicht",
        }}
        displayMode="roomName"
      />,
    );

    const pills = container.querySelectorAll(".rounded-full");
    expect(pills).toHaveLength(1);
    expect(pills[0]).toHaveTextContent(LOCATION_STATUSES.TRANSIT);

    const hint = screen.getByText(LOCATION_STATUSES.UNPLANNED_PRESENT);
    expect(hint.closest(".rounded-full")).toBeNull();
    expect(hint).toHaveAttribute(
      "title",
      "Laut Plan kommt das Kind heute nicht",
    );
  });

  it("keeps a room pill and a sick hint in detailed mode", () => {
    const { container } = render(
      <LocationBadge
        student={{ current_location: "Anwesend - Raum 101", sick: true }}
        displayMode="roomName"
      />,
    );

    const pills = container.querySelectorAll(".rounded-full");
    expect(pills).toHaveLength(1);
    expect(pills[0]).toHaveTextContent("Raum 101");
    expect(
      screen.getByText(LOCATION_STATUSES.SICK).closest(".rounded-full"),
    ).toBeNull();
  });

  it("keeps one pill in binary mode with a Schulhof child on a class trip", () => {
    const { container } = render(
      <PresenceBadge
        student={{ current_location: "Schulhof", class_trip: true }}
      />,
    );

    const pills = container.querySelectorAll(".rounded-full");
    expect(pills).toHaveLength(1);
    expect(pills[0]).toHaveTextContent(LOCATION_STATUSES.SCHOOLYARD);
    expect(
      screen.getByText(LOCATION_STATUSES.CLASS_TRIP).closest(".rounded-full"),
    ).toBeNull();
  });

  it("still replaces the pill when the child is at home", () => {
    const { container } = render(
      <LocationBadge
        student={{ current_location: "Zuhause", sick: true }}
        displayMode="roomName"
      />,
    );

    const pills = container.querySelectorAll(".rounded-full");
    expect(pills).toHaveLength(1);
    expect(pills[0]).toHaveTextContent(LOCATION_STATUSES.SICK);
    expect(container.querySelector("[data-sick-indicator]")).toBeNull();
  });

  it("matches the large location pill's text size", () => {
    const { container } = render(
      <LocationBadge
        student={{ current_location: "Anwesend - Raum 101", sick: true }}
        displayMode="roomName"
        size="lg"
      />,
    );

    expect(container.querySelector("[data-sick-indicator]")).toHaveClass(
      "text-sm",
    );
  });

  it("matches the large binary presence pill's text size", () => {
    const { container } = render(
      <PresenceBadge
        student={{ current_location: "Anwesend", sick: true }}
        size="lg"
      />,
    );

    expect(container.querySelector("[data-sick-indicator]")).toHaveClass(
      "text-sm",
    );
  });
});
