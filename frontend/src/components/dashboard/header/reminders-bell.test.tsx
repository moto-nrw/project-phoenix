import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { Reminder } from "~/lib/reminders-api";

const mockUseReminders = vi.fn();
vi.mock("~/lib/hooks/use-reminders", () => ({
  useReminders: () => mockUseReminders(),
}));

import { RemindersBell } from "./reminders-bell";

function reminder(over: Partial<Reminder> = {}): Reminder {
  return {
    type: "pickup_upcoming",
    student_id: "1",
    title: "Kind",
    due_time: "10:00",
    minutes_away: 10,
    ...over,
  };
}

function setReminders(value: {
  reminders?: Reminder[];
  count?: number;
  enabled?: boolean;
}) {
  mockUseReminders.mockReturnValue({
    reminders: value.reminders ?? [],
    count: value.count ?? value.reminders?.length ?? 0,
    enabled: value.enabled ?? true,
  });
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("RemindersBell", () => {
  it("renders nothing until at least one reminder type is enabled", () => {
    setReminders({ enabled: false, reminders: [reminder()], count: 1 });
    const { container } = render(<RemindersBell />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the bell without a badge when nothing is due", () => {
    setReminders({ enabled: true, reminders: [], count: 0 });
    render(<RemindersBell />);

    // aria-label has no count when there is nothing due.
    expect(
      screen.getByRole("button", { name: "Erinnerungen" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Keine aktiven Erinnerungen.")).toBeInTheDocument();
  });

  it("shows a count badge and lists the due reminders", () => {
    setReminders({
      enabled: true,
      count: 2,
      reminders: [
        reminder({
          type: "pickup_overdue",
          title: "Hannah Klein",
          subtitle: "Klasse 1b",
          minutes_away: -26,
          due_time: "10:44",
        }),
        reminder({
          type: "activity_start",
          student_id: undefined,
          activity_instance_id: "81",
          title: "Schach",
          minutes_away: 10,
          due_time: "11:10",
        }),
      ],
    });
    render(<RemindersBell />);

    expect(
      screen.getByRole("button", { name: "Erinnerungen (2)" }),
    ).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument(); // badge
    expect(screen.getByText("Hannah Klein")).toBeInTheDocument();
    expect(screen.getByText("Klasse 1b")).toBeInTheDocument();
    expect(screen.getByText("Schach")).toBeInTheDocument();
  });

  it("colors only the overdue label red", () => {
    setReminders({
      enabled: true,
      count: 2,
      reminders: [
        reminder({ title: "Hannah", type: "pickup_overdue", minutes_away: -5 }),
        reminder({
          title: "Schach",
          type: "activity_start",
          student_id: undefined,
          activity_instance_id: "81",
          minutes_away: 10,
        }),
      ],
    });
    render(<RemindersBell />);

    expect(screen.getByText("5 Min überfällig").className).toContain(
      "text-moto-red",
    );
    expect(screen.getByText("in 10 Min").className).toContain("text-gray-500");
  });

  it("previews at most five reminders and notes the overflow", () => {
    const many = Array.from({ length: 7 }, (_, i) =>
      reminder({
        student_id: String(i + 1),
        title: `Kind ${i}`,
        minutes_away: i + 1,
      }),
    );
    setReminders({ enabled: true, count: 7, reminders: many });
    render(<RemindersBell />);

    expect(screen.getByText("Kind 0")).toBeInTheDocument();
    expect(screen.getByText("Kind 4")).toBeInTheDocument();
    expect(screen.queryByText("Kind 5")).not.toBeInTheDocument();
    expect(screen.getByText("+2 weitere")).toBeInTheDocument();
  });

  it("links to the full reminders page and toggles open on click", () => {
    setReminders({ enabled: true, count: 1, reminders: [reminder()] });
    render(<RemindersBell />);

    // Tenant-aware href: in path-based routing mode the link must keep the
    // tenant segment (the global test mock resolves slug "test-tenant" / path
    // mode), otherwise the CTA drops the tenant and 404s on the bare host.
    const link = screen.getByRole("link", { name: "Alle ansehen" });
    expect(link).toHaveAttribute("href", "/test-tenant/reminders");

    const button = screen.getByRole("button", { name: "Erinnerungen (1)" });
    expect(button).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(button);
    expect(button).toHaveAttribute("aria-expanded", "true");
  });

  it("closes on Escape", () => {
    setReminders({ enabled: true, count: 1, reminders: [reminder()] });
    render(<RemindersBell />);
    const button = screen.getByRole("button", { name: "Erinnerungen (1)" });

    fireEvent.click(button);
    expect(button).toHaveAttribute("aria-expanded", "true");
    fireEvent.keyDown(document, { key: "Escape" });
    expect(button).toHaveAttribute("aria-expanded", "false");
  });

  it("closes on an outside click but stays open on an inside click", () => {
    setReminders({ enabled: true, count: 1, reminders: [reminder()] });
    render(<RemindersBell />);
    const button = screen.getByRole("button", { name: "Erinnerungen (1)" });

    fireEvent.click(button);
    // Click inside the dropdown — stays open.
    fireEvent.mouseDown(screen.getByText("Alle ansehen"));
    expect(button).toHaveAttribute("aria-expanded", "true");
    // Click outside — closes.
    fireEvent.mouseDown(document.body);
    expect(button).toHaveAttribute("aria-expanded", "false");
  });

  it("closes after navigating via 'Alle ansehen'", () => {
    setReminders({ enabled: true, count: 1, reminders: [reminder()] });
    render(<RemindersBell />);
    const button = screen.getByRole("button", { name: "Erinnerungen (1)" });

    fireEvent.click(button);
    fireEvent.click(screen.getByRole("link", { name: "Alle ansehen" }));
    expect(button).toHaveAttribute("aria-expanded", "false");
  });

  it("caps the badge at 99+", () => {
    setReminders({
      enabled: true,
      count: 150,
      reminders: [reminder({ title: "Kind" })],
    });
    render(<RemindersBell />);
    expect(screen.getByText("99+")).toBeInTheDocument();
  });
});
