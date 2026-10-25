import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  getMealOrders: vi.fn(),
  downloadMealMonth: vi.fn(),
  downloadMealDeliveries: vi.fn(),
}));

vi.mock("~/lib/meal-plan-api", () => ({
  getMealOrders: mocks.getMealOrders,
  downloadMealMonth: mocks.downloadMealMonth,
  downloadMealDeliveries: mocks.downloadMealDeliveries,
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { MealOrders } from "~/lib/meal-plan-api";
import { catalogText } from "~/test/error-catalog-text";
import { MealOrderWeek } from "./meal-order-week";

/** Opens the export menu of a week across two months and its month dialog. */
async function openMonthExport() {
  fireEvent.click(
    await screen.findByRole("button", { name: "Bestellung herunterladen" }),
  );
  fireEvent.click(screen.getByRole("menuitem", { name: "Exportieren…" }));
  return screen.findByRole("dialog");
}

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
    mocks.downloadMealDeliveries.mockResolvedValue(undefined);
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

  // docs/reference/exporte.md, Regel 2: ein Einstieg im Kartenkopf. Liegt die
  // Woche in einem Monat, hat die Monatsübersicht keine Option und steht mit
  // einem Eintrag je Format im Menü, ohne Dialog.
  it("downloads the month of a week inside one month from the menu", async () => {
    render(<MealOrderWeek mondayISO="2026-09-07" canExportMonth />, {
      wrapper: ToastProvider,
    });

    fireEvent.click(
      await screen.findByRole("button", { name: "Bestellung herunterladen" }),
    );
    expect(
      screen.getByText("Monatsübersicht September 2026"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Exportieren…" }),
    ).not.toBeInTheDocument();
    // Lieferliste zuerst, die Monatsübersicht darunter.
    const excelEntries = screen.getAllByRole("menuitem", { name: "Excel" });
    expect(excelEntries).toHaveLength(2);
    fireEvent.click(excelEntries[1]!);
    await waitFor(() =>
      expect(mocks.downloadMealMonth).toHaveBeenCalledWith("2026-09", "xlsx"),
    );
    expect(mocks.downloadMealDeliveries).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("reports a failed month export from the menu", async () => {
    mocks.downloadMealMonth.mockRejectedValueOnce(
      new ApiError("boom", 500, { code: "general.server" }),
    );
    render(<MealOrderWeek mondayISO="2026-09-07" canExportMonth />, {
      wrapper: ToastProvider,
    });

    fireEvent.click(
      await screen.findByRole("button", { name: "Bestellung herunterladen" }),
    );
    fireEvent.click(screen.getAllByRole("menuitem", { name: "PDF" })[1]!);
    expect(
      await screen.findByText(
        catalogText("general.server", "die Monatsübersicht"),
      ),
    ).toBeInTheDocument();
    expect(mocks.downloadMealMonth).toHaveBeenCalledWith("2026-09", "pdf");
  });

  it("keeps a failed month export in the open dialog", async () => {
    mocks.downloadMealMonth.mockRejectedValueOnce(
      new ApiError("boom", 500, { code: "general.server" }),
    );
    render(<MealOrderWeek mondayISO="2026-09-28" canExportMonth />, {
      wrapper: ToastProvider,
    });

    const dialog = await openMonthExport();
    expect(
      within(dialog).getByText("Monatsübersicht exportieren"),
    ).toBeInTheDocument();
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Herunterladen" }),
    );
    expect(
      await within(dialog).findByText(
        catalogText("general.server", "die Monatsübersicht"),
      ),
    ).toBeInTheDocument();
    expect(mocks.downloadMealMonth).toHaveBeenCalledWith("2026-09", "pdf");
  });

  // Die Lieferliste hat keine Optionen: ein Menüeintrag je Format.
  it("downloads the week's delivery list from the same menu", async () => {
    render(<MealOrderWeek mondayISO="2026-09-07" canExportMonth />, {
      wrapper: ToastProvider,
    });

    fireEvent.click(
      await screen.findByRole("button", { name: "Bestellung herunterladen" }),
    );
    expect(screen.getByText("Lieferliste der Woche")).toBeInTheDocument();
    // Die Lieferliste steht vor der Monatsübersicht: ihr Eintrag ist der erste.
    expect(
      screen.getAllByRole("menuitem", { name: "PDF" })[0],
    ).toBeInTheDocument();
    expect(
      screen.getAllByRole("menuitem", { name: "Excel" })[0],
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("menuitem", { name: "CSV" }));
    await waitFor(() =>
      expect(mocks.downloadMealDeliveries).toHaveBeenCalledWith(
        "2026-09-07",
        "2026-09-11",
        "csv",
      ),
    );
    // Kein Knopf je Format mehr im Kartenkopf.
    expect(
      screen.queryByRole("button", { name: /als (PDF|Excel)/ }),
    ).not.toBeInTheDocument();
  });

  it("shows a failed delivery list as a toast with the object", async () => {
    mocks.downloadMealDeliveries.mockRejectedValueOnce(
      new ApiError("boom", 500, { code: "general.server" }),
    );
    render(<MealOrderWeek mondayISO="2026-09-07" canExportMonth />, {
      wrapper: ToastProvider,
    });

    fireEvent.click(
      await screen.findByRole("button", { name: "Bestellung herunterladen" }),
    );
    fireEvent.click(screen.getAllByRole("menuitem", { name: "PDF" })[0]!);
    expect(
      await screen.findByText(catalogText("general.server", "die Lieferliste")),
    ).toBeInTheDocument();
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

    const dialog = await openMonthExport();
    expect(
      within(dialog).getByRole("group", { name: "Monat wählen" }),
    ).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Oktober" }));
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Herunterladen" }),
    );
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

    // Anker: die Bestellung der Woche ist geladen.
    await screen.findAllByText("Vom Caterer bestätigt");
    expect(
      screen.queryByRole("button", { name: "Bestellung herunterladen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /als PDF/ }),
    ).not.toBeInTheDocument();
  });
});
