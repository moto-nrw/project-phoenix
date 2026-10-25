import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
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
import type { MealOrders } from "~/lib/meal-plan-api";
import { MealOrderWeek } from "./meal-order-week";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

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
    // The test catalog mock leaves ICU plurals unrendered; the words remain.
    expect(screen.getAllByText(/Kinder mehr.*Kind weniger/)).not.toHaveLength(
      0,
    );
    // An open day: a short badge, the deadline in its own column.
    expect(screen.getAllByText("Offen")).not.toHaveLength(0);
    expect(screen.getAllByText(/08\.09\., 09:00/)).not.toHaveLength(0);
    expect(screen.getAllByText("OGS geschlossen")).not.toHaveLength(0);
  });

  it("downloads the month of the shown week", async () => {
    render(<MealOrderWeek mondayISO="2026-09-07" canExportMonth />, {
      wrapper: ToastProvider,
    });

    expect(
      screen.getByRole("button", {
        name: "Lieferliste der Woche herunterladen",
      }),
    ).toBeInTheDocument();
    fireEvent.click(
      await screen.findByRole("button", { name: "September als Excel" }),
    );
    await waitFor(() =>
      expect(mocks.downloadMealMonth).toHaveBeenCalledWith("2026-09", "xlsx"),
    );
    expect(
      screen.queryByRole("group", { name: "Monat wählen" }),
    ).not.toBeInTheDocument();
  });

  it.each([
    { outcome: "success", latestSettled: true },
    { outcome: "failure", latestSettled: true },
    { outcome: "success", latestSettled: false },
    { outcome: "failure", latestSettled: false },
  ])(
    "ignores stale $outcome responses (latest settled: $latestSettled)",
    async ({ outcome, latestSettled }) => {
      const oldRequest = deferred<MealOrders>();
      const currentRequest = deferred<MealOrders>();
      const currentOrders: MealOrders = {
        rhythm: "daily",
        detail: "by_class",
        days: [
          {
            date: "2026-09-14",
            closed: false,
            deadline: "2026-09-14T07:00:00Z",
            status: "open",
            confirmedAt: null,
            total: 23,
            classes: [{ schoolClass: "4c", count: 23 }],
            changes: [],
          },
        ],
      };
      mocks.getMealOrders
        .mockReturnValueOnce(oldRequest.promise)
        .mockReturnValueOnce(currentRequest.promise);

      const { rerender } = render(
        <MealOrderWeek mondayISO="2026-09-07" canExportMonth />,
        { wrapper: ToastProvider },
      );
      rerender(<MealOrderWeek mondayISO="2026-09-14" canExportMonth />);
      expect(mocks.getMealOrders).toHaveBeenLastCalledWith(
        "2026-09-14",
        "2026-09-18",
      );
      if (latestSettled) {
        await act(async () => currentRequest.resolve(currentOrders));
        expect(screen.getAllByText("4c: 23")).not.toHaveLength(0);
      }

      await act(async () => {
        if (outcome === "success") {
          oldRequest.resolve({ ...currentOrders, days: [] });
        } else {
          oldRequest.reject(new Error("Superseded request failed"));
        }
      });

      if (!latestSettled) {
        expect(screen.getByText("Wird geladen…")).toBeInTheDocument();
        await act(async () => currentRequest.resolve(currentOrders));
      }
      // Anker: die Bestellungen der aktuell gewählten Woche bleiben sichtbar.
      expect(screen.getAllByText("4c: 23")).not.toHaveLength(0);
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    },
  );

  // #3873: a week across two months offers both months.
  it("lets a week across two months choose the month", async () => {
    render(<MealOrderWeek mondayISO="2026-09-28" canExportMonth />, {
      wrapper: ToastProvider,
    });

    fireEvent.click(await screen.findByRole("button", { name: "Oktober" }));
    fireEvent.click(screen.getByRole("button", { name: "Oktober als PDF" }));
    await waitFor(() =>
      expect(mocks.downloadMealMonth).toHaveBeenCalledWith("2026-10", "pdf"),
    );
  });

  it("marks past days without an order as not ordered", async () => {
    mocks.getMealOrders.mockResolvedValue({
      rhythm: "daily",
      detail: "by_class",
      days: [
        {
          date: "2026-09-07",
          closed: false,
          deadline: "2026-09-07T07:00:00Z",
          status: "unordered",
          confirmedAt: null,
          total: 0,
          classes: [],
          changes: [],
        },
      ],
    });
    render(<MealOrderWeek mondayISO="2026-09-07" canExportMonth={false} />, {
      wrapper: ToastProvider,
    });

    expect(await screen.findAllByText("Nicht bestellt")).not.toHaveLength(0);
    expect(screen.queryByText("Offen")).not.toBeInTheDocument();
    expect(screen.queryByText(/07\.09\., 09:00/)).not.toBeInTheDocument();
  });

  it("offers exports only with the lists permission", async () => {
    render(<MealOrderWeek mondayISO="2026-09-07" canExportMonth={false} />, {
      wrapper: ToastProvider,
    });

    await screen.findAllByText("Vom Caterer bestätigt");
    expect(
      screen.queryByRole("button", {
        name: "Lieferliste der Woche herunterladen",
      }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /als PDF/ }),
    ).not.toBeInTheDocument();
  });
});
