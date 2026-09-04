import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { useState } from "react";
import { render, screen, fireEvent, act } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

import { SearchStudentCard } from "./search-student-card";
import { StudentCardClockProvider } from "./student-card-clock";
import type { Student } from "~/lib/api";

// The card renders the shared StudentCard, whose photo row asks the settings
// API whether Fotos are switched on. Not the subject here.
vi.mock("~/lib/hooks/use-student-photos-enabled", () => ({
  useStudentPhotosEnabled: () => ({ enabled: false, isLoading: false }),
}));

// Counts how often the card body actually runs, which is what the memo is
// about — the rendered DOM looks the same either way.
let cardRenders = 0;
vi.mock("./student-card", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./student-card")>();
  const Original = actual.StudentCard;
  return {
    ...actual,
    StudentCard: (props: React.ComponentProps<typeof Original>) => {
      cardRenders++;
      return <Original {...props} />;
    },
  };
});

const student = {
  id: "7",
  first_name: "Max",
  second_name: "Mustermann",
  school_class: "1a",
  group_name: "Bärengruppe",
  current_location: "Anwesend",
  has_full_access: true,
  arrival_time: "08:00",
  pickup_time: "16:00",
} as unknown as Student;

const EMPTY: string[] = [];

function renderCard(props: Partial<Parameters<typeof SearchStudentCard>[0]>) {
  return (
    <SearchStudentCard
      student={student}
      isToday
      checkinMode={false}
      checkinSelectMode={false}
      isCheckinSelected={false}
      isCheckinPending={false}
      userGroups={EMPTY}
      groupRooms={EMPTY}
      supervisedRooms={EMPTY}
      trackingData={undefined}
      onOpen={() => undefined}
      onCheckinClick={() => undefined}
      {...props}
    />
  );
}

describe("SearchStudentCard", () => {
  it("renders the child with class, group and the two time rows", () => {
    render(renderCard({}));

    expect(
      screen.getByLabelText("Max Mustermann - Tippen für mehr Infos"),
    ).toBeInTheDocument();
    expect(screen.getByText("1a")).toBeInTheDocument();
    expect(screen.getByText("Gruppe: Bärengruppe")).toBeInTheDocument();
    expect(screen.getByText(/Ankunftszeit: 08:00 Uhr/)).toBeInTheDocument();
    expect(screen.getByText(/Gehzeit: 16:00 Uhr/)).toBeInTheDocument();
  });

  // #2975: the card is memoised so that a page render — a filter toggle, a
  // check-in tap on ANOTHER child, a minute tick — stops at the card boundary
  // instead of rebuilding all 100 Kinderkarten.
  it("does not re-render when the surrounding page renders for other reasons", () => {
    const noop = vi.fn();

    function Page() {
      const [counter, setCounter] = useState(0);
      return (
        <div>
          <button type="button" onClick={() => setCounter((n) => n + 1)}>
            Seite neu rendern
          </button>
          <span>{counter}</span>
          <SearchStudentCard
            student={student}
            isToday
            checkinMode={false}
            checkinSelectMode={false}
            isCheckinSelected={false}
            isCheckinPending={false}
            userGroups={EMPTY}
            groupRooms={EMPTY}
            supervisedRooms={EMPTY}
            trackingData={undefined}
            onOpen={noop}
            onCheckinClick={noop}
          />
        </div>
      );
    }

    cardRenders = 0;
    render(<Page />);
    expect(cardRenders).toBe(1);

    fireEvent.click(screen.getByText("Seite neu rendern"));
    fireEvent.click(screen.getByText("Seite neu rendern"));

    expect(screen.getByText("2")).toBeInTheDocument();
    expect(cardRenders).toBe(1);
  });

  describe("minute clock", () => {
    beforeEach(() => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it("takes the time rows' clock from the list provider", () => {
      // 15:00 is before the 16:00 Gehzeit, 17:00 after it: if the row read the
      // provider's clock, the second render styles the row as overdue.
      const { rerender } = render(
        <StudentCardClockProvider now={new Date("2026-06-01T13:00:00Z")}>
          {renderCard({})}
        </StudentCardClockProvider>,
      );

      const atFifteen = screen.getByText(/Gehzeit: 16:00 Uhr/).outerHTML;

      act(() => {
        rerender(
          <StudentCardClockProvider now={new Date("2026-06-01T15:00:00Z")}>
            {renderCard({})}
          </StudentCardClockProvider>,
        );
      });

      expect(screen.getByText(/Gehzeit: 16:00 Uhr/).outerHTML).not.toBe(
        atFifteen,
      );
    });
  });
});
