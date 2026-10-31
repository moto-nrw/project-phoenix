import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ActivityWaitlistSection } from "./activity-waitlist-section";
import { ApiError } from "~/lib/api-error";
import type { CourseWaitlist } from "~/lib/course-waitlist-helpers";
import { catalogText } from "~/test/error-catalog-text";

const state = vi.hoisted(() => ({
  permissions: ["users:update"] as string[],
  waitlists: undefined as CourseWaitlist[] | undefined,
  error: undefined as unknown,
  participantError: undefined as unknown,
  waitlistMutate: vi.fn(),
  participantMutate: vi.fn(),
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
  activityWaitlistKeys: {
    participants: (activityId: string) => `activity-participants-${activityId}`,
  },
  courseWaitlistKeys: {
    activity: (activityId: string) => `course-waitlist-${activityId}`,
  },
  useSWRAuth: (key: string | null) => {
    state.keys.push(key);
    if (key?.startsWith("activity-participants-")) {
      return {
        data: [{ name: "Emma Klein" }, { name: "Zeynep Çelik" }],
        error: state.participantError,
        mutate: state.participantMutate,
      };
    }
    return {
      data: key ? state.waitlists : undefined,
      error: key ? state.error : undefined,
      mutate: state.waitlistMutate,
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
    state.permissions = ["users:update", "activities:read"];
    state.waitlists = [theater];
    state.error = undefined;
    state.participantError = undefined;
    state.waitlistMutate = vi.fn();
    state.participantMutate = vi.fn();
    state.keys = [];
  });

  it("zeigt Plätze, Teilnehmende und die Warteliste der AG", () => {
    render(<ActivityWaitlistSection activityId="70" />);

    expect(
      screen.getByRole("heading", { name: "Plätze und Warteliste" }),
    ).toBeInTheDocument();
    expect(
      screen
        .getByRole("heading", { name: "Plätze und Warteliste" })
        .closest("section"),
    ).toHaveClass("moto-content-surface");
    expect(
      screen.getByText(
        "Für die nächste Anfrage: 8 von 8 Plätzen vergeben · 0 frei",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Emma Klein, Zeynep Çelik")).toBeInTheDocument();
    expect(screen.getByRole("list", { name: "Warteliste" }).textContent).toBe(
      "1.Lina Amini",
    );
  });

  it("trennt heutige Teilnehmende von Plätzen für die nächste Anfrage", () => {
    state.waitlists = [
      {
        ...theater,
        participants: 0,
        freeSlots: 8,
        slotFreeForNext: true,
      },
    ];

    render(<ActivityWaitlistSection activityId="70" />);

    expect(
      screen.getByText(
        "Für die nächste Anfrage: 0 von 8 Plätzen vergeben · 8 frei",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Heute dabei:")).toBeInTheDocument();
    expect(screen.getByText("Emma Klein, Zeynep Çelik")).toBeInTheDocument();
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

  it("zeigt die Warteliste ohne Recht auf die Teilnehmerliste", () => {
    state.permissions = ["users:update"];

    render(<ActivityWaitlistSection activityId="70" />);

    expect(
      screen.getByText(
        "Für die nächste Anfrage: 8 von 8 Plätzen vergeben · 0 frei",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Emma Klein, Zeynep Çelik"),
    ).not.toBeInTheDocument();
    expect(state.keys).not.toContain("activity-participants-70");
  });

  it("zeigt keine unvollständige Warteliste, wenn Teilnehmende nicht laden", async () => {
    state.participantError = new ApiError("down", 503, {
      code: "general.unavailable",
    });

    render(<ActivityWaitlistSection activityId="70" />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Teilnehmenden der AG"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(
        "Für die nächste Anfrage: 8 von 8 Plätzen vergeben · 0 frei",
      ),
    ).not.toBeInTheDocument();
  });

  it("lädt beide Quellen erneut, wenn beide fehlgeschlagen sind", async () => {
    state.error = new ApiError("down", 503, { code: "general.unavailable" });
    state.participantError = new ApiError("down", 503, {
      code: "general.unavailable",
    });

    render(<ActivityWaitlistSection activityId="70" />);

    fireEvent.click(await screen.findByRole("button", { name: "Wiederholen" }));

    expect(state.waitlistMutate).toHaveBeenCalledOnce();
    expect(state.participantMutate).toHaveBeenCalledOnce();
  });
});
