import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import type { Reminder, RemindersResult } from "~/lib/reminders-api";

const mockUseReminders = vi.fn();
vi.mock("~/lib/hooks/use-reminders", () => ({
  useReminders: () => mockUseReminders(),
}));

const mockReplace = vi.fn();
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ replace: mockReplace }),
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
  it("shows the empty state when nothing is due", () => {
    set({ reminders: [], count: 0 });
    render(<RemindersPage />);
    expect(screen.getByText("Keine aktiven Erinnerungen")).toBeInTheDocument();
  });

  it("groups reminders by type and colors only overdue red", () => {
    set({
      count: 2,
      reminders: [
        {
          type: "pickup_overdue",
          title: "Hannah Klein",
          subtitle: "Klasse 1b",
          due_time: "10:44",
          minutes_away: -5,
        },
        {
          type: "activity_start",
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
      "text-[#FF3130]",
    );
    expect(screen.getByText("in 10 Min").className).not.toContain(
      "text-[#FF3130]",
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
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it("does not redirect while data has not loaded yet (no data)", () => {
    set({ reminders: [], count: 0 });
    render(<RemindersPage />);
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it("redirects to the dashboard when the feature is disabled", () => {
    set({
      reminders: [],
      count: 0,
      data: { reminders: [], count: 0, enabled: false },
    });
    const { container } = render(<RemindersPage />);
    expect(mockReplace).toHaveBeenCalledWith("/");
    // Nothing rendered while the redirect is in flight.
    expect(container).toBeEmptyDOMElement();
  });

  it("renders the page when the feature is enabled", () => {
    set({
      reminders: [],
      count: 0,
      data: { reminders: [], count: 0, enabled: true },
    });
    render(<RemindersPage />);
    expect(screen.getByText("Keine aktiven Erinnerungen")).toBeInTheDocument();
    expect(mockReplace).not.toHaveBeenCalled();
  });
});
