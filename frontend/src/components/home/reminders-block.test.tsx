import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { Reminder, RemindersResult } from "~/lib/reminders-api";

const hook = vi.hoisted(() => ({
  reminders: [] as Reminder[],
  error: undefined as Error | undefined,
  isLoading: false,
  data: undefined as RemindersResult | undefined,
}));

vi.mock("~/lib/hooks/use-reminders", () => ({
  useReminders: () => hook,
}));
vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/test-tenant${path}`,
}));

import { RemindersBlock } from "./reminders-block";

function reminder(overrides: Partial<Reminder> = {}): Reminder {
  return {
    type: "pickup_upcoming",
    student_id: "42",
    title: "Mia Berger",
    subtitle: "Abholung",
    due_time: "15:30",
    minutes_away: 10,
    ...overrides,
  };
}

describe("RemindersBlock (#2180)", () => {
  beforeEach(() => {
    hook.reminders = [];
    hook.error = undefined;
    hook.isLoading = false;
    hook.data = { reminders: [], count: 0, enabled: true };
  });

  it("zeigt Titel, Untertitel und Zeit einer Erinnerung", () => {
    hook.reminders = [reminder()];

    render(<RemindersBlock />);

    expect(screen.getByText("Mia Berger")).toBeInTheDocument();
    expect(screen.getByText("Abholung")).toBeInTheDocument();
    expect(screen.getByText("15:30")).toBeInTheDocument();
  });

  // Wer auf eine Zeile tippt, will zu dem Kind — nicht auf eine Liste, die er
  // dort erneut durchsuchen muss.
  it("führt eine Erinnerung zu einem Kind auf dessen Seite", () => {
    hook.reminders = [reminder()];

    render(<RemindersBlock />);

    expect(
      screen.getByRole("link", { name: "Mia Berger: Kind öffnen" }),
    ).toHaveAttribute("href", "/test-tenant/students/42");
  });

  // Eine Erinnerung ohne Kind (Aktivitätsbeginn) hat kein Ziel. Dann darf die
  // Zeile auch nicht anfassbar aussehen.
  it("lässt eine Erinnerung ohne Kind eine Anzeige bleiben", () => {
    hook.reminders = [
      reminder({
        type: "activity_start",
        student_id: undefined,
        activity_instance_id: "9",
        title: "Kreativ-AG beginnt",
      }),
    ];

    render(<RemindersBlock />);

    expect(screen.getByText("Kreativ-AG beginnt")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /Kind öffnen/ }),
    ).not.toBeInTheDocument();
  });

  it("sagt es, wenn nichts ansteht", () => {
    render(<RemindersBlock />);

    expect(screen.getByText("Nichts steht an")).toBeInTheDocument();
  });

  it("unterscheidet einen Ladefehler von einem ruhigen Tag", () => {
    hook.error = new Error("boom");

    render(<RemindersBlock />);

    expect(
      screen.getByText(/konnten nicht geladen werden/),
    ).toBeInTheDocument();
    expect(screen.queryByText("Nichts steht an")).not.toBeInTheDocument();
  });
});
