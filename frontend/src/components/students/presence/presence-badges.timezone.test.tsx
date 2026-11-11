import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { LocationBadge } from "./location-badge";
import { PresenceBadge } from "./presence-badge";

// Runs in `school-date-dom`: only forked processes apply a runtime TZ change.
afterEach(() => vi.unstubAllEnvs());

const student = {
  current_location: "Anwesend",
  // 08:15 Berlin (CEST), 20:15 the evening before in Honolulu.
  location_since: "2026-09-09T06:15:00Z",
};

describe.each([
  [
    "LocationBadge",
    <LocationBadge
      key="detailed"
      student={student}
      displayMode="roomName"
      showLocationSince
    />,
  ],
  [
    "PresenceBadge",
    <PresenceBadge key="binary" student={student} showLocationSince />,
  ],
])("%s in a browser outside Germany (#6563)", (_name, badge) => {
  it("shows „seit“ in Berlin time", () => {
    vi.stubEnv("TZ", "Pacific/Honolulu");
    // The zone change took effect, or the case below proves nothing.
    expect(new Date(student.location_since).getHours()).toBe(20);

    render(badge);

    expect(screen.getByText("seit 08:15 Uhr")).toBeInTheDocument();
  });
});
