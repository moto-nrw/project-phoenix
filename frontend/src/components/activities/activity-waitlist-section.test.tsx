import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ActivityWaitlistSection } from "./activity-waitlist-section";
import type { CourseWaitlist } from "~/lib/course-waitlist-helpers";

const state = vi.hoisted(() => ({
  permissions: ["users:update"] as string[],
  waitlists: undefined as CourseWaitlist[] | undefined,
  error: undefined as unknown,
  keys: [] as (string | null)[],
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "7", roles: ["user"], permissions: state.permissions },
    },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    state.keys.push(key);
    if (key?.startsWith("activity-participants-")) {
      return {
        data: [{ name: "Emma Klein" }, { name: "Zeynep Çelik" }],
        error: undefined,
        mutate: vi.fn(),
      };
    }
    return {
      data: key ? state.waitlists : undefined,
      error: key ? state.error : undefined,
      mutate: vi.fn(),
    };
  },
}));

const theater: CourseWaitlist = {
  activityGroupId: "70",
  name: "Theater-AG",
  participantLimit: 8,
  participants: 8,
  freeSlots: 0,
  slotFreeForNext: false,
  entries: [
    {
      requestId: "11",
      studentId: "101",
      studentName: "Lina Amini",
      rank: 1,
      submittedAt: "2026-09-01T10:00:00Z",
    },
  ],
};

describe("ActivityWaitlistSection", () => {
  beforeEach(() => {
    state.permissions = ["users:update"];
    state.waitlists = [theater];
    state.error = undefined;
    state.keys = [];
  });

  it("zeigt Plätze, Teilnehmende und die Warteliste der AG", () => {
    render(<ActivityWaitlistSection activityId="70" />);

    expect(
      screen.getByRole("heading", { name: "Plätze und Warteliste" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("8 von 8 Plätzen vergeben · 0 frei"),
    ).toBeInTheDocument();
    expect(screen.getByText("Emma Klein, Zeynep Çelik")).toBeInTheDocument();
    expect(screen.getByRole("list", { name: "Warteliste" }).textContent).toBe(
      "1.Lina Amini",
    );
  });

  it("lädt ohne Recht zum Entscheiden nichts und zeigt nichts", () => {
    state.permissions = ["activities:read"];
    const { container } = render(
      <div data-testid="section">
        <ActivityWaitlistSection activityId="70" />
      </div>,
    );

    expect(screen.getByTestId("section")).toBeInTheDocument();
    expect(container.textContent).toBe("");
    expect(state.keys.every((key) => key === null)).toBe(true);
  });
});
