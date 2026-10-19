import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  getMealOrders: vi.fn(),
  downloadMealMonth: vi.fn(),
}));

vi.mock("~/lib/meal-plan-api", () => ({
  getMealOrders: mocks.getMealOrders,
  downloadMealMonth: mocks.downloadMealMonth,
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { MealOrderWeek } from "./meal-order-week";

describe("MealOrderWeek", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getMealOrders.mockResolvedValue({
      rhythm: "daily",
      detail: "by_class",
      days: [
        {
          date: "2026-09-07",
          closed: false,
          deadline: "2026-09-07T07:00:00Z",
          status: "confirmed",
          confirmedAt: "2026-09-07T08:00:00Z",
          total: 12,
          classes: [
            { schoolClass: "2a", count: 7 },
            { schoolClass: "3b", count: 5 },
          ],
          changes: [
            { sequence: 1, schoolClass: "2a", delta: 1 },
            { sequence: 2, schoolClass: "2a", delta: 1 },
            { sequence: 3, schoolClass: "3b", delta: -1 },
          ],
        },
        {
          date: "2026-09-08",
          closed: false,
          deadline: "2026-09-08T07:00:00Z",
          status: "open",
          confirmedAt: null,
          total: 10,
          classes: [{ schoolClass: "2a", count: 10 }],
          changes: [],
        },
        {
          date: "2026-09-09",
          closed: true,
          deadline: "2026-09-09T07:00:00Z",
          status: "ordered",
          confirmedAt: null,
          total: 0,
          classes: [],
          changes: [],
        },
      ],
    });
    mocks.downloadMealMonth.mockResolvedValue(undefined);
  });

  it("shows each day's state, numbers and late changes", async () => {
    render(<MealOrderWeek mondayISO="2026-09-07" canExportMonth />, {
      wrapper: ToastProvider,
    });

    expect(
      await screen.findAllByText("Vom Caterer bestätigt"),
    ).not.toHaveLength(0);
    expect(mocks.getMealOrders).toHaveBeenCalledWith(
      "2026-09-07",
      "2026-09-11",
    );
    expect(screen.getAllByText("2a: 7 · 3b: 5")).not.toHaveLength(0);
    expect(screen.getAllByText("+2 · −1")).not.toHaveLength(0);
    expect(screen.getAllByText(/^Offen bis /)).not.toHaveLength(0);
    expect(screen.getAllByText("OGS geschlossen")).not.toHaveLength(0);
  });

  it("downloads the month of the shown week", async () => {
    render(<MealOrderWeek mondayISO="2026-09-28" canExportMonth />, {
      wrapper: ToastProvider,
    });

    fireEvent.click(
      await screen.findByRole("button", { name: /Monat als Excel/ }),
    );
    await waitFor(() =>
      expect(mocks.downloadMealMonth).toHaveBeenCalledWith("2026-09", "xlsx"),
    );
  });

  it("offers the month only with the lists permission", async () => {
    render(<MealOrderWeek mondayISO="2026-09-07" canExportMonth={false} />, {
      wrapper: ToastProvider,
    });

    await screen.findAllByText("Vom Caterer bestätigt");
    expect(
      screen.queryByRole("button", { name: /Monat als PDF/ }),
    ).not.toBeInTheDocument();
  });
});
