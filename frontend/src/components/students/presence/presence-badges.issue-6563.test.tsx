import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { PresenceStudent } from "~/lib/presence-status";
import { LocationBadge } from "./location-badge";
import { PresenceBadge } from "./presence-badge";

// #6563: LocationBadge and PresenceBadge show the same status by the same rules.
const badges = [
  [
    "LocationBadge",
    (student: PresenceStudent, showLocationSince = false) => (
      <LocationBadge
        student={student}
        displayMode="roomName"
        showLocationSince={showLocationSince}
      />
    ),
  ],
  [
    "PresenceBadge",
    (student: PresenceStudent, showLocationSince = false) => (
      <PresenceBadge student={student} showLocationSince={showLocationSince} />
    ),
  ],
] as const;

describe.each(badges)("%s (#6563)", (_name, renderBadge) => {
  it("keeps the pill within the width of its container", () => {
    render(
      <div className="w-24">
        {renderBadge({ current_location: "Anwesend" })}
      </div>,
    );

    const pill = screen.getByText("Anwesend");
    expect(pill.parentElement).toHaveClass("max-w-full", "min-w-0");
  });

  it("names the reason of „Kommt heute nicht“ on the pill", () => {
    render(
      renderBadge({
        current_location: "Zuhause",
        arrival_is_exception: true,
        arrival_notes: "Arzttermin",
      }),
    );

    expect(screen.getByTitle("Arzttermin")).toHaveTextContent(
      "Kommt heute nicht",
    );
  });

  it("names the reason of „Kommt heute nicht“ on the hint", () => {
    render(
      renderBadge({
        current_location: "Anwesend",
        arrival_is_exception: true,
        arrival_notes: "Arzttermin",
      }),
    );

    expect(screen.getByTitle("Arzttermin")).toHaveTextContent(
      "Ungeplant anwesend",
    );
  });

  // Same path, only the location differs: „seit“ belongs to a stay in a
  // room, at home or in class, never to Schulhof or Schule.
  it.each([
    ["Anwesend", "seit 10:15 Uhr"],
    ["Zuhause", "seit 10:15 Uhr"],
    ["Im Unterricht", "seit 10:15 Uhr"],
    ["Schulhof", null],
    ["Schule", null],
  ])("shows „seit“ by location status (%s)", (location, expected) => {
    render(
      renderBadge(
        { current_location: location, location_since: "2026-09-09T08:15:00Z" },
        true,
      ),
    );

    // Anker: the badge rendered its pill.
    expect(screen.getByText(location)).toBeInTheDocument();
    if (expected) {
      expect(screen.getByText(/^seit /)).toHaveTextContent(expected);
    } else {
      expect(screen.queryByText(/^seit /)).not.toBeInTheDocument();
    }
  });

  it.each([
    ["2026-09-09T08:15:00Z", "seit 10:15 Uhr"],
    ["kein Zeitpunkt", null],
  ])("shows „seit“ only for a readable time (%s)", (since, expected) => {
    render(
      renderBadge(
        { current_location: "Anwesend", location_since: since },
        true,
      ),
    );

    // Anker: the badge rendered its pill.
    expect(screen.getByText("Anwesend")).toBeInTheDocument();
    if (expected) {
      expect(screen.getByText(/^seit /)).toHaveTextContent(expected);
    } else {
      expect(screen.queryByText(/^seit /)).not.toBeInTheDocument();
    }
  });

  // „Kommt heute nicht“ keeps the since-time of the replaced location, as
  // LocationBadge showed it before #6563 (deliberately unchanged). A recorded
  // checkout to class shows only „Im Unterricht“ with its own time (#5710).
  it("keeps „seit“ under „Kommt heute nicht“ at home and under „Im Unterricht“ in class", () => {
    const { rerender } = render(
      renderBadge(
        {
          current_location: "Zuhause",
          location_since: "2026-09-09T08:15:00Z",
          arrival_is_exception: true,
        },
        true,
      ),
    );

    // Anker: „Kommt heute nicht“ replaces the home status.
    expect(screen.getByText("Kommt heute nicht")).toBeInTheDocument();
    expect(screen.getByText(/^seit /)).toHaveTextContent("seit 10:15 Uhr");

    rerender(
      renderBadge(
        {
          current_location: "Im Unterricht",
          location_since: "2026-09-09T08:15:00Z",
          arrival_is_exception: true,
        },
        true,
      ),
    );

    expect(screen.getByText("Im Unterricht")).toBeInTheDocument();
    expect(screen.queryByText("Kommt heute nicht")).not.toBeInTheDocument();
    expect(screen.getByText(/^seit /)).toHaveTextContent("seit 10:15 Uhr");
  });
});

// PresenceBadge shows Unterwegs and a bare room name as „Anwesend“. A room
// keeps its „seit“ like LocationBadge; Unterwegs hides it like LocationBadge.
it.each([
  ["Gruppenraum 1", "seit 10:15 Uhr"],
  ["Unterwegs", null],
])(
  "PresenceBadge shows „seit“ under Anwesend for %s like LocationBadge",
  (location, expected) => {
    render(
      <PresenceBadge
        student={{
          current_location: location,
          location_since: "2026-09-09T08:15:00Z",
        }}
        showLocationSince
      />,
    );

    // Anker: PresenceBadge maps both locations to Anwesend.
    expect(screen.getByText("Anwesend")).toBeInTheDocument();
    if (expected) {
      expect(screen.getByText(/^seit /)).toHaveTextContent(expected);
    } else {
      expect(screen.queryByText(/^seit /)).not.toBeInTheDocument();
    }
  },
);

it.each([
  ["ohne Ortsangabe", null, "Zuhause"],
  ["unbekannter Ort", "Unbekannt", "Anwesend"],
])(
  "PresenceBadge hides „seit“ for %s",
  (_description, currentLocation, expectedLabel) => {
    render(
      <PresenceBadge
        student={{
          current_location: currentLocation,
          location_since: "2026-09-09T08:15:00Z",
        }}
        showLocationSince
      />,
    );

    // Anker: Das Badge zeigt weiterhin den für den Binärmodus passenden Status.
    expect(screen.getByText(expectedLabel)).toBeInTheDocument();
    expect(screen.queryByText(/^seit /)).not.toBeInTheDocument();
  },
);
