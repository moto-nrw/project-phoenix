import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  BirthdayCelebration,
  BirthdayOverview,
} from "~/lib/birthdays-api";

const swr = vi.hoisted(() => ({
  keys: [] as (string | null)[],
  byKey: new Map<string, unknown>(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    swr.keys.push(key);
    const data = key ? swr.byKey.get(key) : undefined;
    return { data, error: undefined, isLoading: key !== null && !data };
  },
}));
vi.mock("~/lib/hooks/use-media-query", () => ({
  BELOW_SM: "(max-width: 639px)",
  useMediaQuery: () => false,
}));

import { BirthdaysBlock } from "./birthdays-block";

function child(
  id: string,
  date: string,
  overrides: Partial<BirthdayCelebration> = {},
): BirthdayCelebration {
  return {
    kind: "student",
    id,
    name: `Kind ${id}`,
    date,
    age: 7,
    isToday: date === "2026-09-09",
    ...overrides,
  };
}

function week(
  weekStart: string,
  weekEnd: string,
  celebrations: BirthdayCelebration[],
): BirthdayOverview {
  return {
    enabled: true,
    includeStaff: false,
    today: "2026-09-09",
    weekStart,
    weekEnd,
    earliestWeekStart: "2026-08-10",
    latestWeekStart: "2026-10-05",
    celebrations,
  };
}

const current = week("2026-09-07", "2026-09-13", [child("1", "2026-09-09")]);

beforeEach(() => {
  swr.keys = [];
  swr.byKey = new Map();
});

describe("BirthdaysBlock", () => {
  it("shows the current week without a request of its own", () => {
    render(<BirthdaysBlock current={current} currentLoading={false} />);

    expect(screen.getByText("Diese Woche · 07.09.–13.09.")).toBeInTheDocument();
    expect(screen.getByText("Kind 1")).toBeInTheDocument();
    expect(swr.keys.every((key) => key === null)).toBe(true);
    // Already on this week: the way back is shown but has nothing to do.
    expect(screen.getByRole("button", { name: "Diese Woche" })).toBeDisabled();
  });

  it("steps back to last week and home again", () => {
    swr.byKey.set(
      "birthday-overview:2026-08-31",
      week("2026-08-31", "2026-09-06", [child("9", "2026-09-02")]),
    );
    render(<BirthdaysBlock current={current} currentLoading={false} />);

    fireEvent.click(screen.getByRole("button", { name: "Vorherige Woche" }));

    expect(swr.keys).toContain("birthday-overview:2026-08-31");
    expect(
      screen.getByText("Letzte Woche · 31.08.–06.09."),
    ).toBeInTheDocument();
    expect(screen.getByText("Kind 9")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Diese Woche" }));
    expect(screen.getByText("Kind 1")).toBeInTheDocument();
  });

  it("stops at the bounds the server sends", () => {
    render(
      <BirthdaysBlock
        current={{ ...current, earliestWeekStart: "2026-09-07" }}
        currentLoading={false}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Vorherige Woche" }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Nächste Woche" })).toBeEnabled();
  });

  // A full week must not push today's birthday off the card, and nothing may
  // silently disappear: the rest is counted and opens the whole week.
  it("keeps today on the card and opens the whole week", () => {
    const busy = week("2026-09-07", "2026-09-13", [
      child("1", "2026-09-07"),
      child("2", "2026-09-07"),
      child("3", "2026-09-08"),
      child("4", "2026-09-08"),
      child("5", "2026-09-09"),
      child("6", "2026-09-10"),
      child("7", "2026-09-11"),
      child("8", "2026-09-12"),
    ]);
    render(<BirthdaysBlock current={busy} currentLoading={false} />);

    expect(screen.getByText("Kind 5")).toBeInTheDocument();
    expect(screen.queryByText("Kind 1")).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Noch 4 Geburtstage ansehen" }),
    );

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("Kind 1")).toBeInTheDocument();
    expect(within(dialog).getByText("Kind 8")).toBeInTheDocument();
  });

  it("names the empty week", () => {
    render(
      <BirthdaysBlock
        current={week("2026-09-07", "2026-09-13", [])}
        currentLoading={false}
      />,
    );

    expect(
      screen.getByText("Keine Geburtstage in dieser Woche"),
    ).toBeInTheDocument();
  });

  it("shows an error instead of an empty week when the initial request fails", () => {
    render(
      <BirthdaysBlock
        current={undefined}
        currentLoading={false}
        currentError={new Error("Birthday fetch failed: 500")}
      />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Die Geburtstage konnten nicht geladen werden. Bitte versuchen Sie es noch einmal.",
    );
    expect(
      screen.queryByText("Keine Geburtstage in dieser Woche"),
    ).not.toBeInTheDocument();
  });

  it("keeps loaded birthdays visible when a later refresh fails", () => {
    render(
      <BirthdaysBlock
        current={current}
        currentLoading={false}
        currentError={new Error("Birthday fetch failed: 500")}
      />,
    );

    expect(screen.getByText("Kind 1")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
