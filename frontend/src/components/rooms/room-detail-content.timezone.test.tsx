import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { RoomDetailContent } from "./room-detail-content";

// Runs in `school-date-dom`: only forked processes apply a runtime TZ change.
afterEach(() => vi.unstubAllEnvs());

vi.mock("./students-in-room-section", () => ({
  StudentsInRoomSection: () => null,
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "test-tenant",
  useTenantSafe: () => ({ tenantSlug: "test-tenant", tenant: null }),
  useNFCEnabled: vi.fn(() => true),
}));

const session = (
  sessionId: string,
  startedAt: string,
  activityName: string,
) => ({
  sessionId,
  startedAt,
  endedAt: null,
  durationMinutes: null,
  activityName,
  supervisorName: "",
  studentCount: 0,
});

describe("RoomDetailContent in a browser outside Germany (#6471)", () => {
  it("groups the room history by the Berlin school day", () => {
    vi.stubEnv("TZ", "Pacific/Honolulu");
    // 01:30 Berlin on Wednesday 21.10.2026 is still Tuesday 13:30 in
    // Honolulu; 12:00 Berlin is Wednesday 00:00 there.
    const early = "2026-10-20T23:30:00Z";
    const noon = "2026-10-21T10:00:00Z";
    // The zone change took effect, or the case below proves nothing.
    expect(new Date(early).getDate()).toBe(20);
    expect(new Date(noon).getDate()).toBe(21);

    render(
      <RoomDetailContent
        room={{ id: "1", name: "Testraum", isOccupied: false }}
        history={[
          session("a", early, "Frühdienst"),
          session("b", noon, "Mittag"),
        ]}
      />,
    );

    expect(
      screen
        .getAllByRole("heading", { level: 3, name: /Oktober 2026/ })
        .map((heading) => heading.textContent),
    ).toEqual(["Mittwoch, 21. Oktober 2026"]);
    expect(screen.getByText("Beginn: 01:30")).toBeInTheDocument();
    expect(screen.getByText("Beginn: 12:00")).toBeInTheDocument();
  });
});
