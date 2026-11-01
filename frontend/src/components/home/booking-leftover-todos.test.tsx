import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { BookingLeftover } from "~/lib/booking-leftover";

const state = vi.hoisted(() => ({
  leftovers: [] as unknown[],
  keys: [] as Array<string | null>,
}));

vi.mock("~/lib/timetable-api", () => ({ timetableService: {} }));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    state.keys.push(key);
    if (key === null) return {};
    if (key.startsWith("booking-leftovers-")) return { data: state.leftovers };
    return {
      data: [
        {
          id: "4",
          name: "Schuljahr",
          isActive: true,
          startDate: "2026-08-01",
          endDate: "2027-07-31",
        },
      ],
    };
  },
}));

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/test-tenant${path}`,
}));

import { BookingLeftoverTodos } from "./booking-leftover-todos";

function leftover(studentId: string, kept = false): BookingLeftover {
  return {
    templateId: "9",
    templateName: "Hausaufgabenbetreuung",
    studentId,
    studentName: `Kind ${studentId}`,
    weekdays: [3],
    bookingEndedOn: "2026-09-07",
    from: "2026-09-10",
    kept,
  };
}

describe("BookingLeftoverTodos (#5102)", () => {
  beforeEach(() => {
    state.leftovers = [];
    state.keys = [];
  });

  it("counts the open children only and links to the Regeltermine", () => {
    state.leftovers = [leftover("1"), leftover("2"), leftover("3", true)];
    render(<BookingLeftoverTodos enabled />);

    expect(state.keys).toContain("booking-leftovers-4");
    expect(
      screen.getByText(
        "2 Kinder stehen noch in Regelterminen, obwohl ihre Buchung weggefallen ist.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Kinder prüfen" })).toHaveAttribute(
      "href",
      "/test-tenant/betreuungsplan?view=serien",
    );
  });

  it("stays away when everything is decided or without planning rights", () => {
    state.leftovers = [leftover("1", true)];
    const { container, rerender } = render(<BookingLeftoverTodos enabled />);
    expect(container).toBeEmptyDOMElement();

    state.leftovers = [leftover("1")];
    rerender(<BookingLeftoverTodos enabled={false} />);
    expect(container).toBeEmptyDOMElement();
  });
});
