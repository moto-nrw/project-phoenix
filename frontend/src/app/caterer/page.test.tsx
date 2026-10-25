import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  fetchCatererSchools: vi.fn(),
  fetchCatererOrders: vi.fn(),
  downloadCatererDeliveries: vi.fn(),
  confirmCatererOrder: vi.fn(),
}));

vi.mock("~/lib/caterer-api", () => mocks);

import { ToastProvider } from "~/contexts/ToastContext";
import CatererOverviewPage from "./page";
import { setTestClock } from "~/test/clock";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function week(dayTotals: number[], change = 0) {
  return {
    school: { id: "1", name: "x" },
    deliveryAddress: "Hof",
    rhythm: "daily",
    detail: "total",
    days: [
      "2026-09-07",
      "2026-09-08",
      "2026-09-09",
      "2026-09-10",
      "2026-09-11",
    ].map((date, index) => ({
      date,
      closed: false,
      deadline: `${date}T07:00:00Z`,
      status: "ordered",
      orderId: String(index + 1),
      confirmedAt: null,
      total: dayTotals[index] ?? 0,
      classes: [],
      children: [],
      changes:
        index === 0 && change !== 0
          ? [
              {
                sequence: 1,
                child: { firstName: "", lastName: "", schoolClass: "" },
                delta: change,
                detectedAt: "",
              },
            ]
          : [],
    })),
  };
}

describe("CatererOverviewPage (#3873)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.fetchCatererSchools.mockResolvedValue({
      catererName: "Küche am Markt",
      schools: [
        { id: "1", name: "OGS Nord" },
        { id: "2", name: "OGS Süd" },
      ],
    });
    mocks.fetchCatererOrders.mockImplementation((schoolId: string) =>
      Promise.resolve(
        schoolId === "1"
          ? week([10, 10, 10, 10, 10], 1)
          : week([5, 5, 5, 5, 5]),
      ),
    );
  });

  it("shows every school per day with sums per school and per day", async () => {
    render(<CatererOverviewPage />, { wrapper: ToastProvider });

    expect(
      await screen.findByText("Bestellungen für Küche am Markt"),
    ).toBeInTheDocument();
    // The test clock is Wednesday, 2026-09-09: this week opens.
    expect(mocks.fetchCatererOrders).toHaveBeenCalledWith(
      "1",
      "2026-09-07",
      "2026-09-11",
    );
    const table = (await screen.findAllByRole("table"))[0]!;
    const sum = within(table)
      .getByText("Summe", { selector: "span" })
      .closest("tr")!;
    expect(within(sum).getAllByText("15")).toHaveLength(5);
    expect(within(sum).getByText("75")).toBeInTheDocument();
    const north = within(table).getByText("OGS Nord").closest("tr")!;
    expect(within(north).getByText("50")).toBeInTheDocument();
    expect(within(north).getByText("+1")).toBeInTheDocument();
  });

  it("marks failed school sums and incomplete aggregate totals", async () => {
    mocks.fetchCatererOrders.mockImplementation((schoolId: string) =>
      schoolId === "1"
        ? Promise.resolve(week([10, 10, 10, 10, 10]))
        : Promise.reject(new Error("School orders unavailable")),
    );
    render(<CatererOverviewPage />, { wrapper: ToastProvider });
    await screen.findAllByText("Bestellung nicht geladen");
    const table = screen.getAllByRole("table")[0]!;
    const failed = within(table).getByText("OGS Süd").closest("tr")!;
    expect(within(failed).getByText("Nicht geladen")).toBeInTheDocument();
    const total = within(table)
      .getByText("Summe", { selector: "span" })
      .closest("tr")!;
    expect(within(total).getAllByText("Unvollständig")).toHaveLength(6);
    const loaded = within(table).getByText("OGS Nord").closest("tr")!;
    expect(within(loaded).getByText("50")).toBeInTheDocument();
  });

  it("advances reads and exports across Berlin midnight", async () => {
    vi.useFakeTimers();
    setTestClock("2026-09-09T23:59:30+02:00");
    await act(async () => {
      render(<CatererOverviewPage />, { wrapper: ToastProvider });
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Morgen" }));
    });
    expect(mocks.fetchCatererOrders).toHaveBeenLastCalledWith(
      "2",
      "2026-09-10",
      "2026-09-10",
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(mocks.fetchCatererOrders).toHaveBeenLastCalledWith(
      "2",
      "2026-09-11",
      "2026-09-11",
    );
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "PDF" }));
    });
    expect(mocks.downloadCatererDeliveries).toHaveBeenCalledWith(
      "2026-09-11",
      "2026-09-11",
      "pdf",
    );
  });

  it.each(["resolve", "reject"] as const)(
    "keeps the current period when an older school request settles last (%s)",
    async (outcome) => {
      render(<CatererOverviewPage />, { wrapper: ToastProvider });
      const table = (await screen.findAllByRole("table"))[0]!;
      expect(within(table).getByText("OGS Nord")).toBeInTheDocument();

      const olderSchools = deferred<{
        catererName: string;
        schools: { id: string; name: string }[];
      }>();
      mocks.fetchCatererSchools.mockReturnValueOnce(olderSchools.promise);
      fireEvent.click(screen.getByRole("button", { name: "Nächste Woche" }));
      fireEvent.click(screen.getByRole("button", { name: "Morgen" }));
      const currentTable = (await screen.findAllByRole("table"))[0]!;
      expect(within(currentTable).getByText("OGS Nord")).toBeInTheDocument();
      expect(mocks.fetchCatererOrders).toHaveBeenLastCalledWith(
        "2",
        "2026-09-10",
        "2026-09-10",
      );

      await act(async () => {
        if (outcome === "resolve") {
          olderSchools.resolve({
            catererName: "Veralteter Stand",
            schools: [{ id: "1", name: "Alte Schule" }],
          });
        } else {
          olderSchools.reject(new Error("Previous period failed"));
        }
      });

      expect(
        screen.getByText("Bestellungen für Küche am Markt"),
      ).toBeInTheDocument();
      expect(mocks.fetchCatererOrders).toHaveBeenLastCalledWith(
        "2",
        "2026-09-10",
        "2026-09-10",
      );
      const north = within(screen.getAllByRole("table")[0]!)
        .getByText("OGS Nord")
        .closest("tr")!;
      expect(within(north).getAllByText("10")).toHaveLength(2);
    },
  );
  it("keeps loading the current period when older orders arrive", async () => {
    render(<CatererOverviewPage />, { wrapper: ToastProvider });
    const table = (await screen.findAllByRole("table"))[0]!;
    expect(within(table).getByText("OGS Nord")).toBeInTheDocument();

    const olderOrders = deferred<ReturnType<typeof week>>();
    mocks.fetchCatererOrders.mockReturnValueOnce(olderOrders.promise);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Nächste Woche" }));
    });
    expect(mocks.fetchCatererOrders).toHaveBeenCalledWith(
      "1",
      "2026-09-14",
      "2026-09-18",
    );

    const currentSchools = deferred<{
      catererName: string;
      schools: { id: string; name: string }[];
    }>();
    mocks.fetchCatererSchools.mockReturnValueOnce(currentSchools.promise);
    fireEvent.click(screen.getByRole("button", { name: "Morgen" }));
    expect(screen.getByText("Wird geladen…")).toBeInTheDocument();

    await act(async () => {
      olderOrders.resolve(week([99, 99, 99, 99, 99]));
    });
    expect(screen.getByText("Wird geladen…")).toBeInTheDocument();

    await act(async () => {
      currentSchools.resolve({
        catererName: "Küche am Markt",
        schools: [{ id: "1", name: "OGS Nord" }],
      });
    });
    const north = within(screen.getAllByRole("table")[0]!)
      .getByText("OGS Nord")
      .closest("tr")!;
    expect(within(north).getAllByText("10")).toHaveLength(2);
  });

  it("keeps the selected period when a previous period's confirmation finishes", async () => {
    const confirmation = deferred<void>();
    mocks.confirmCatererOrder.mockReturnValueOnce(confirmation.promise);
    render(<CatererOverviewPage />, { wrapper: ToastProvider });
    await screen.findByText("Bestellungen für Küche am Markt");
    fireEvent.click(
      screen.getByRole("button", { name: "OGS Nord ausklappen" }),
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: "Erhalten bestätigen" })[0]!,
    );
    expect(mocks.confirmCatererOrder).toHaveBeenCalledWith("1", "1");

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Morgen" }));
    });
    expect(mocks.fetchCatererOrders).toHaveBeenLastCalledWith(
      "2",
      "2026-09-10",
      "2026-09-10",
    );
    await act(async () => {
      confirmation.resolve();
    });
    expect(mocks.fetchCatererOrders).toHaveBeenLastCalledWith(
      "2",
      "2026-09-10",
      "2026-09-10",
    );
    const north = within(screen.getAllByRole("table")[0]!)
      .getByText("OGS Nord")
      .closest("tr")!;
    expect(within(north).getAllByText("10")).toHaveLength(2);
  });
});
