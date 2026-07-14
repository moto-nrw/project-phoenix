import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { DeviationHistorySlideOver } from "./deviation-history-slide-over";
import { useSWRAuth } from "~/lib/swr";
import type {
  DeviationHistoryEvent,
  EnrichedInstance,
} from "~/lib/timetable-types";

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    getDeviationHistory: vi.fn(),
  },
}));

const mockUseSWRAuth = vi.mocked(useSWRAuth);

function makeInstance(): EnrichedInstance {
  return {
    id: "42",
    date: "2026-09-22",
    startTime: "14:00",
    endTime: "15:00",
    title: "Malen-AG",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityGroupId: "7",
    activityType: "activity",
    roomId: "1",
    roomName: "Atelier",
    staff: [],
    studentIds: [],
    students: [],
    staffCount: 1,
    absentStaffCount: 0,
    expectedStudentsCount: 0,
    presentStudentsCount: 0,
  } as unknown as EnrichedInstance;
}

function makeEvent(
  overrides: Partial<DeviationHistoryEvent>,
): DeviationHistoryEvent {
  return {
    id: "1",
    occurrenceDate: "2026-09-22",
    startTime: "14:00",
    eventType: "absence",
    occurredAt: "2026-09-21T09:30:00+02:00",
    ...overrides,
  };
}

function mockEvents(events: DeviationHistoryEvent[]) {
  mockUseSWRAuth.mockReturnValue({
    data: { events },
    isLoading: false,
    error: undefined,
    mutate: vi.fn(),
    isValidating: false,
  } as unknown as ReturnType<typeof useSWRAuth>);
}

describe("DeviationHistorySlideOver", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders protocol entries with actor and reason", () => {
    mockEvents([
      makeEvent({
        eventType: "substitution",
        subjectStaffId: "11",
        subjectStaffName: "Anna Alt",
        relatedStaffId: "12",
        relatedStaffName: "Bernd Neu",
        actorName: "Clara Chef",
        reason: "krank",
      }),
    ]);

    render(
      <DeviationHistorySlideOver
        instance={makeInstance()}
        open
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("Vertretung zugewiesen")).toBeInTheDocument();
    expect(
      screen.getByText("Bernd Neu vertritt Anna Alt."),
    ).toBeInTheDocument();
    expect(screen.getByText(/Begründung: krank/)).toBeInTheDocument();
    expect(screen.getByText(/Clara Chef/)).toBeInTheDocument();
  });

  it("falls back to 'Unbekanntes Konto' when the actor is gone", () => {
    mockEvents([makeEvent({ subjectStaffName: "Anna Alt" })]);

    render(
      <DeviationHistorySlideOver
        instance={makeInstance()}
        open
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText(/Unbekanntes Konto/)).toBeInTheDocument();
  });

  it("shows the empty state when nothing is protocolled", () => {
    mockEvents([]);

    render(
      <DeviationHistorySlideOver
        instance={makeInstance()}
        open
        onClose={vi.fn()}
      />,
    );

    expect(
      screen.getByText(/sind noch keine Änderungen protokolliert/),
    ).toBeInTheDocument();
  });
});
