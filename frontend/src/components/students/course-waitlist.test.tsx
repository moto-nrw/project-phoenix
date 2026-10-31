import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  CourseWaitlistHints,
  CourseWaitlistProvider,
  CourseWaitlistSummary,
  RequestWaitlistPlaces,
} from "./course-waitlist";
import type { CourseWaitlist } from "~/lib/course-waitlist-helpers";

function waitlist(overrides: Partial<CourseWaitlist>): CourseWaitlist {
  return {
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
      {
        requestId: "12",
        studentId: "102",
        studentName: "Mia Berg",
        rank: 2,
        submittedAt: "2026-09-02T10:00:00Z",
      },
    ],
    ...overrides,
  };
}

const freeTheater = waitlist({
  participants: 7,
  freeSlots: 1,
  slotFreeForNext: true,
});

describe("CourseWaitlistHints", () => {
  it("names the next child when a place is free", () => {
    render(<CourseWaitlistHints waitlists={[freeTheater]} />);
    expect(
      screen.getByText(
        /In „Theater-AG“ ist ein Platz frei\. Als Nächstes ist Lina Amini dran\./,
      ),
    ).toBeInTheDocument();
  });

  it("collects several free places in one card", () => {
    render(
      <CourseWaitlistHints
        waitlists={[
          freeTheater,
          waitlist({
            activityGroupId: "71",
            name: "Fußball-AG",
            freeSlots: 2,
            slotFreeForNext: true,
          }),
        ]}
      />,
    );
    expect(screen.getByText("Freie Plätze in AGs")).toBeInTheDocument();
    expect(screen.getAllByRole("listitem")).toHaveLength(2);
  });

  it("stays out of the way while every AG is full", () => {
    const { container } = render(
      <div data-testid="hints">
        <CourseWaitlistHints waitlists={[waitlist({})]} />
      </div>,
    );
    expect(screen.getByTestId("hints")).toBeInTheDocument();
    expect(container.textContent).toBe("");
  });
});

describe("RequestWaitlistPlaces", () => {
  it("shows the rank on a full AG", () => {
    render(
      <CourseWaitlistProvider
        waitlists={[waitlist({})]}
        refresh={() => undefined}
      >
        <RequestWaitlistPlaces requestId="12" />
      </CourseWaitlistProvider>,
    );
    expect(screen.getByText("Warteliste, Platz 2")).toBeInTheDocument();
    expect(
      screen.getByText("Die AG ist voll. Wird ein Platz frei, steht es hier."),
    ).toBeInTheDocument();
  });

  it("tells the team the first child can move up", () => {
    render(
      <CourseWaitlistProvider
        waitlists={[freeTheater]}
        refresh={() => undefined}
      >
        <RequestWaitlistPlaces requestId="11" />
      </CourseWaitlistProvider>,
    );
    expect(screen.getByText("Platz frei")).toBeInTheDocument();
    expect(
      screen.getByText("Dieses Kind ist als Nächstes dran."),
    ).toBeInTheDocument();
  });

  it("keeps an AG without a limit out of the card", () => {
    const { container } = render(
      <div data-testid="places">
        <CourseWaitlistProvider
          waitlists={[waitlist({ participantLimit: null, freeSlots: null })]}
          refresh={() => undefined}
        >
          <RequestWaitlistPlaces requestId="11" />
        </CourseWaitlistProvider>
      </div>,
    );
    expect(screen.getByTestId("places")).toBeInTheDocument();
    expect(container.textContent).toBe("");
  });
});

describe("CourseWaitlistSummary", () => {
  it("lists places and the ranked queue", () => {
    render(<CourseWaitlistSummary waitlist={freeTheater} />);
    expect(
      screen.getByText("7 von 8 Plätzen vergeben · 1 frei"),
    ).toBeInTheDocument();
    const list = screen.getByRole("list", { name: "Warteliste" });
    expect(list.textContent).toBe("1.Lina Amini2.Mia Berg");
  });

  it("says when nobody waits", () => {
    render(
      <CourseWaitlistSummary
        waitlist={waitlist({ entries: [], freeSlots: 3, participants: 5 })}
      />,
    );
    expect(
      screen.getByText("Niemand wartet auf einen Platz."),
    ).toBeInTheDocument();
  });
});
