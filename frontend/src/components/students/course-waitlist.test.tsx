import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  CourseWaitlistOverview,
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

describe("CourseWaitlistOverview", () => {
  it("listet die wartenden Kinder je AG in der Reihenfolge der Anfragen", () => {
    render(
      <CourseWaitlistOverview
        waitlists={[waitlist({})]}
        onOpenRequest={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Wartelisten" }),
    ).toBeInTheDocument();
    const theater = screen.getByRole("region", { name: "Theater-AG" });
    expect(
      within(theater).getByText("8 von 8 Plätzen vergeben · voll"),
    ).toBeInTheDocument();
    const rows = within(theater).getAllByRole("listitem");
    expect(rows.map((row) => row.textContent)).toEqual([
      expect.stringContaining("1.Lina Amini"),
      expect.stringContaining("2.Mia Berg"),
    ]);
    expect(within(theater).queryByText("Platz frei")).toBeNull();
    expect(within(theater).getByText("Lina Amini")).toBeInTheDocument();
  });

  it("markiert die AG und das erste Kind, wenn ein Platz frei ist", () => {
    render(
      <CourseWaitlistOverview
        waitlists={[freeTheater]}
        onOpenRequest={vi.fn()}
      />,
    );

    expect(screen.getByText("1 AG mit freiem Platz")).toBeInTheDocument();
    expect(
      screen.getByText("7 von 8 Plätzen vergeben · 1 frei"),
    ).toBeInTheDocument();
    const [first, second] = screen.getAllByRole("listitem");
    expect(first).toHaveTextContent("Als Nächstes dran");
    expect(second).not.toHaveTextContent("Als Nächstes dran");
  });

  it("öffnet die Anfrage des gewählten Kindes", () => {
    const onOpenRequest = vi.fn();
    render(
      <CourseWaitlistOverview
        waitlists={[waitlist({})]}
        onOpenRequest={onOpenRequest}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Anfrage von Mia Berg öffnen" }),
    );

    expect(onOpenRequest).toHaveBeenCalledWith("102");
  });

  it("zeigt keine Karte ohne wartendes Kind oder ohne Teilnehmergrenze", () => {
    const { container } = render(
      <div data-testid="overview">
        <CourseWaitlistOverview
          waitlists={[
            waitlist({ entries: [] }),
            waitlist({
              activityGroupId: "71",
              participantLimit: null,
              freeSlots: null,
            }),
          ]}
          onOpenRequest={vi.fn()}
        />
      </div>,
    );
    expect(screen.getByTestId("overview")).toBeInTheDocument();
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
