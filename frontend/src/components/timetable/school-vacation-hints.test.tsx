import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { SchoolVacationHint } from "~/lib/school-vacations-api";

const state = vi.hoisted(() => ({
  hints: [] as unknown[],
  keys: [] as Array<string | null>,
  acknowledged: [] as Array<[string, string]>,
  acknowledgeError: null as Error | null,
  mutated: [] as string[],
  success: [] as string[],
  shownErrors: [] as unknown[],
  modal: null as null | { initialFrom: string; initialTo: string },
  today: "2026-09-09",
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    state.keys.push(key);
    return key === null ? {} : { data: state.hints };
  },
  useTenantMutateMatching: (keys: string[]) => () => {
    state.mutated.push(...keys);
    return Promise.resolve();
  },
}));

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => state.today,
}));

vi.mock("~/lib/school-vacations-api", () => ({
  SCHOOL_VACATION_HINTS_KEY: "timetable-school-vacation-hints",
  getSchoolVacationHints: vi.fn(),
  acknowledgeSchoolVacation: (start: string, end: string) => {
    state.acknowledged.push([start, end]);
    return state.acknowledgeError
      ? Promise.reject(state.acknowledgeError)
      : Promise.resolve();
  },
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: (message: string) => state.success.push(message),
  }),
  useApiErrorDisplay: () => ({
    show: (error: unknown) => {
      state.shownErrors.push(error);
      return Promise.resolve();
    },
  }),
}));

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/test-tenant${path}`,
}));

// The bulk cancellation (#3594) has its own tests; here it only has to be
// opened with the vacation as its range.
vi.mock("~/components/timetable/bulk-cancel-appointments-modal", () => ({
  BulkCancelAppointmentsModal: (props: {
    initialFrom: string;
    initialTo: string;
  }) => {
    state.modal = {
      initialFrom: props.initialFrom,
      initialTo: props.initialTo,
    };
    return <div data-testid="bulk-cancel-modal" />;
  },
}));

import {
  SchoolVacationHints,
  vacationHintMessage,
} from "./school-vacation-hints";

const christmas: SchoolVacationHint = {
  name: "Weihnachtsferien",
  startDate: "2026-12-23",
  endDate: "2027-01-06",
  cancelFrom: "2026-12-23",
  count: 12,
};

const autumn: SchoolVacationHint = {
  name: "Herbstferien",
  startDate: "2026-10-17",
  endDate: "2026-10-31",
  cancelFrom: "2026-10-20",
  count: 1,
};

describe("SchoolVacationHints (#5345)", () => {
  beforeEach(() => {
    state.hints = [];
    state.keys = [];
    state.acknowledged = [];
    state.acknowledgeError = null;
    state.mutated = [];
    state.success = [];
    state.shownErrors = [];
    state.modal = null;
    state.today = "2026-09-09";
  });

  it("names the vacation, its dates and the number of Regeltermine without claiming the OGS is closed", () => {
    state.hints = [christmas];
    render(<SchoolVacationHints enabled />);

    expect(state.keys).toContain("timetable-school-vacation-hints-2026-09-09");
    expect(
      screen.getByText("Termine in den Ferien prüfen"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Weihnachtsferien vom 23.12.2026 bis 06.01.2027: In dieser Zeit sind 12 Regeltermine geplant. Findet in dieser Zeit Betreuung statt? Wenn nicht, sagen Sie die Termine ab oder tragen Sie Schließtage ein.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", {
        name: "Schließtage eintragen: Weihnachtsferien",
      }),
    ).toHaveAttribute("href", "/test-tenant/calendar-periods");
  });

  it("counts a running vacation from today on", () => {
    expect(vacationHintMessage(autumn)).toBe(
      "Herbstferien vom 17.10.2026 bis 31.10.2026: In dieser Zeit ist noch 1 Regeltermin geplant. Findet in dieser Zeit Betreuung statt? Wenn nicht, sagen Sie die Termine ab oder tragen Sie Schließtage ein.",
    );
  });

  it("loads fresh hints when the Berlin calendar day changes", () => {
    state.hints = [christmas];
    const { rerender } = render(<SchoolVacationHints enabled />);

    state.today = "2026-09-10";
    rerender(<SchoolVacationHints enabled />);

    expect(state.keys).toContain("timetable-school-vacation-hints-2026-09-09");
    expect(state.keys).toContain("timetable-school-vacation-hints-2026-09-10");
  });

  it("words one Regeltermin and a single vacation day", () => {
    expect(
      vacationHintMessage({
        ...autumn,
        startDate: "2027-05-18",
        endDate: "2027-05-18",
        name: "Pfingstferien",
      }),
    ).toBe(
      "Pfingstferien am 18.05.2027: In dieser Zeit ist 1 Regeltermin geplant. Findet in dieser Zeit Betreuung statt? Wenn nicht, sagen Sie die Termine ab oder tragen Sie Schließtage ein.",
    );
  });

  it("opens the existing bulk cancellation with the vacation from its first day still ahead", () => {
    state.hints = [autumn];
    render(<SchoolVacationHints enabled />);

    fireEvent.click(
      screen.getByRole("button", { name: "Termine absagen: Herbstferien" }),
    );

    expect(screen.getByTestId("bulk-cancel-modal")).toBeInTheDocument();
    expect(state.modal).toEqual({
      initialFrom: "2026-10-20",
      initialTo: "2026-10-31",
    });
  });

  it("confirms planned care for exactly this vacation and reloads the hint", async () => {
    state.hints = [christmas];
    render(<SchoolVacationHints enabled />);

    fireEvent.click(
      screen.getByRole("button", {
        name: "Betreuung ist geplant: Weihnachtsferien",
      }),
    );

    await waitFor(() =>
      expect(state.mutated).toContain("timetable-school-vacation-hints"),
    );
    expect(state.acknowledged).toEqual([["2026-12-23", "2027-01-06"]]);
    expect(state.success).toEqual([
      "Gespeichert: Für „Weihnachtsferien“ ist Betreuung geplant. Der Hinweis ist ausgeblendet.",
    ]);
  });

  it("shows a failed confirmation through the error path", async () => {
    state.hints = [christmas];
    state.acknowledgeError = new Error("boom");
    render(<SchoolVacationHints enabled />);

    fireEvent.click(
      screen.getByRole("button", {
        name: "Betreuung ist geplant: Weihnachtsferien",
      }),
    );

    await waitFor(() =>
      expect(state.shownErrors).toEqual([state.acknowledgeError]),
    );
    expect(state.success).toEqual([]);
  });

  it("lists several vacations as rows of one card", () => {
    state.hints = [autumn, christmas];
    render(<SchoolVacationHints enabled showClosingDaysLink={false} />);

    expect(screen.getByTestId("school-vacation-hints")).toBeInTheDocument();
    expect(
      screen.getByText("vom 17.10.2026 bis 31.10.2026 · 1 Regeltermin"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("vom 23.12.2026 bis 06.01.2027 · 12 Regeltermine"),
    ).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: /^Betreuung ist geplant/ }),
    ).toHaveLength(2);
    expect(
      screen.queryByRole("link", { name: /^Schließtage eintragen/ }),
    ).not.toBeInTheDocument();
  });

  it("stays away without hints and without planning rights", () => {
    state.hints = [christmas];
    const { container, rerender } = render(<SchoolVacationHints enabled />);
    // Anker: mit Hinweis und Recht ist er da.
    expect(
      screen.getByText("Termine in den Ferien prüfen"),
    ).toBeInTheDocument();

    rerender(<SchoolVacationHints enabled={false} />);
    expect(container).toBeEmptyDOMElement();
    expect(state.keys.at(-1)).toBeNull();

    state.hints = [];
    rerender(<SchoolVacationHints enabled />);
    expect(container).toBeEmptyDOMElement();
  });
});
