import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import type { Reminder, RemindersResult } from "~/lib/reminders-api";

const mockUseReminders = vi.fn();
vi.mock("~/lib/hooks/use-reminders", () => ({
  useReminders: () => mockUseReminders(),
}));

import RemindersPage from "./page";

function set(value: {
  reminders?: Reminder[];
  count?: number;
  error?: unknown;
  isLoading?: boolean;
  data?: RemindersResult;
}) {
  mockUseReminders.mockReturnValue({
    reminders: value.reminders ?? [],
    count: value.count ?? value.reminders?.length ?? 0,
    error: value.error,
    isLoading: value.isLoading ?? false,
    data: value.data,
  });
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("RemindersPage", () => {
  it("shows the activation hint when reminder types are disabled", () => {
    set({
      reminders: [],
      count: 0,
      data: { reminders: [], count: 0, enabled: false },
    });
    render(<RemindersPage />);
    expect(screen.getByText("Keine aktiven Erinnerungen")).toBeInTheDocument();
    expect(
      screen.getByText(/Erinnerungstypen werden in den Einstellungen/),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Erinnerungen aktiviert/),
    ).not.toBeInTheDocument();
  });

  it("shows a compact success status when reminders are enabled but none are active", () => {
    set({
      reminders: [],
      count: 0,
      data: { reminders: [], count: 0, enabled: true },
    });
    render(<RemindersPage />);
    expect(
      screen.getByText(
        "Erinnerungen aktiviert. Aktuell gibt es keine aktiven Erinnerungen.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Erinnerungstypen werden in den Einstellungen/),
    ).not.toBeInTheDocument();
  });

  it("groups reminders by type and colors only overdue red", () => {
    set({
      count: 2,
      reminders: [
        {
          type: "pickup_overdue",
          student_id: "42",
          title: "Hannah Klein",
          subtitle: "Klasse 1b",
          due_time: "10:44",
          minutes_away: -5,
        },
        {
          type: "activity_start",
          activity_instance_id: "81",
          title: "Schach",
          due_time: "11:10",
          minutes_away: 10,
        },
      ],
    });
    render(<RemindersPage />);

    expect(screen.getByText("Überfällige Abholung")).toBeInTheDocument();
    expect(screen.getByText("Aktivitätsbeginn")).toBeInTheDocument();
    expect(screen.getByText("Hannah Klein")).toBeInTheDocument();
    expect(screen.getByText("Schach")).toBeInTheDocument();

    expect(screen.getByText("5 Min überfällig").className).toContain(
      "text-moto-red",
    );
    expect(screen.getByText("in 10 Min").className).not.toContain(
      "text-moto-red",
    );
  });

  it("renders an error alert when the fetch fails", () => {
    set({ error: new Error("nope"), reminders: [], count: 0 });
    render(<RemindersPage />);
    expect(
      screen.getByText("Erinnerungen konnten nicht geladen werden."),
    ).toBeInTheDocument();
  });

  it("shows a loading indicator before the first data arrives", () => {
    set({ reminders: [], count: 0, isLoading: true });
    render(<RemindersPage />);
    expect(
      screen.getByLabelText("Erinnerungen werden geladen…"),
    ).toBeInTheDocument();
  });
});
