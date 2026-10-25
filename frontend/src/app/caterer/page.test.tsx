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
  fetchCatererSchools: vi.fn(),
  fetchCatererOrders: vi.fn(),
  downloadCatererDeliveries: vi.fn(),
  confirmCatererOrder: vi.fn(),
}));

vi.mock("~/lib/caterer-api", () => mocks);

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
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

  it("prints from one button and offers the file formats in the menu", async () => {
    mocks.downloadCatererDeliveries.mockResolvedValue(undefined);
    render(<CatererOverviewPage />, { wrapper: ToastProvider });
    await screen.findByText("Bestellungen für Küche am Markt");

    // Anker: Drucken steht als einzige Aktion neben dem Zeitraum.
    expect(screen.getByRole("button", { name: "Drucken" })).toBeEnabled();
    expect(
      screen.queryByRole("button", { name: "PDF" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    const menu = await screen.findByRole("menu");
    expect(within(menu).getByText("Herunterladen")).toBeInTheDocument();
    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => item.textContent),
    ).toEqual(["PDF", "Excel", "CSV"]);

    fireEvent.click(within(menu).getByRole("menuitem", { name: "Excel" }));
    await waitFor(() =>
      expect(mocks.downloadCatererDeliveries).toHaveBeenCalledWith(
        "2026-09-07",
        "2026-09-11",
        "xlsx",
      ),
    );
  });

  it("loads inside the orders card instead of covering the page", async () => {
    const pending = deferred<{
      catererName: string;
      schools: { id: string; name: string }[];
    }>();
    mocks.fetchCatererSchools.mockReturnValueOnce(pending.promise);
    const { container } = render(<CatererOverviewPage />, {
      wrapper: ToastProvider,
    });

    // Anker: die Karte steht schon, die Tabelle zeigt ihr Skelett.
    expect(
      screen.getByRole("heading", { name: "Bestellungen" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Wird geladen…")).toBeInTheDocument();
    expect(container.querySelector(".fixed.inset-0")).toBeNull();

    await act(async () => {
      pending.resolve({
        catererName: "Küche am Markt",
        schools: [{ id: "1", name: "OGS Nord" }],
      });
    });
    expect(
      screen.getByText("Bestellungen für Küche am Markt"),
    ).toBeInTheDocument();
  });

  it("shows a failed load with its retry inside the orders card", async () => {
    mocks.fetchCatererSchools.mockRejectedValueOnce(new ApiError("down", 503));
    render(<CatererOverviewPage />, { wrapper: ToastProvider });

    const retry = await screen.findByRole("button", { name: "Wiederholen" });
    const card = screen
      .getByRole("heading", { name: "Bestellungen" })
      .closest("section")!;
    expect(within(card).getByRole("alert")).toContainElement(retry);

    fireEvent.click(retry);
    expect(
      await screen.findByText("Bestellungen für Küche am Markt"),
    ).toBeInTheDocument();
  });

  it("says in the orders card when no school is supplied", async () => {
    mocks.fetchCatererSchools.mockResolvedValueOnce({
      catererName: "Küche am Markt",
      schools: [],
    });
    render(<CatererOverviewPage />, { wrapper: ToastProvider });

    const sentence = await screen.findByText(
      "Sie beliefern zurzeit keine Schule. Sobald eine Schule Sie einlädt, sehen Sie hier ihre Bestellungen.",
    );
    const card = screen
      .getByRole("heading", { name: "Bestellungen für Küche am Markt" })
      .closest("section")!;
    expect(card).toContainElement(sentence);
  });

  it("retries the orders of one school from its card", async () => {
    let southFails = true;
    mocks.fetchCatererOrders.mockImplementation((schoolId: string) => {
      if (schoolId === "2" && southFails) {
        return Promise.reject(new ApiError("down", 503));
      }
      return Promise.resolve(
        schoolId === "1" ? week([10, 10, 10, 10, 10]) : week([5, 5, 5, 5, 5]),
      );
    });
    render(<CatererOverviewPage />, { wrapper: ToastProvider });
    await screen.findByText("Bestellungen für Küche am Markt");

    fireEvent.click(screen.getByRole("button", { name: "OGS Süd ausklappen" }));
    const south = screen
      .getByRole("heading", { name: "OGS Süd" })
      .closest("section")!;
    const retry = await within(south).findByRole("button", {
      name: "Wiederholen",
    });
    expect(
      within(south).queryByText(/laden Sie die Seite neu/),
    ).not.toBeInTheDocument();

    southFails = false;
    mocks.fetchCatererOrders.mockClear();
    fireEvent.click(retry);

    // Anker: die Bestellung der Schule steht jetzt in ihrer Karte.
    expect(
      await within(south).findAllByRole("button", {
        name: "Erhalten bestätigen",
      }),
    ).not.toHaveLength(0);
    expect(within(south).queryByRole("alert")).not.toBeInTheDocument();
    expect(mocks.fetchCatererOrders).toHaveBeenCalledTimes(1);
    expect(mocks.fetchCatererOrders).toHaveBeenCalledWith(
      "2",
      "2026-09-07",
      "2026-09-11",
    );
    expect(mocks.fetchCatererSchools).toHaveBeenCalledTimes(1);
  });

  it("shows the confirmation as a green badge where the button was", async () => {
    let confirmed = false;
    mocks.confirmCatererOrder.mockImplementation(() => {
      confirmed = true;
      return Promise.resolve();
    });
    mocks.fetchCatererOrders.mockImplementation((schoolId: string) => {
      const orders = week([10, 10, 10, 10, 10]);
      if (schoolId === "1" && confirmed) {
        orders.days = orders.days.map((day) => ({
          ...day,
          status: "confirmed",
        }));
      }
      return Promise.resolve(orders);
    });
    render(<CatererOverviewPage />, { wrapper: ToastProvider });
    await screen.findByText("Bestellungen für Küche am Markt");
    fireEvent.click(
      screen.getByRole("button", { name: "OGS Nord ausklappen" }),
    );
    const north = screen
      .getByRole("heading", { name: "OGS Nord" })
      .closest("section")!;
    const button = within(north).getAllByRole("button", {
      name: "Erhalten bestätigen",
    })[0]!;
    const cell = button.closest<HTMLElement>("td, dd")!;

    fireEvent.click(button);

    await waitFor(() =>
      expect(within(cell).getByText("Bestätigt")).toBeInTheDocument(),
    );
    expect(
      within(cell).queryByRole("button", { name: "Erhalten bestätigen" }),
    ).not.toBeInTheDocument();
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
      fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("menuitem", { name: "PDF" }));
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
      // Die Karte zeigt die Tabelle schon beim Laden; erst die Zeile
      // der Schule belegt, dass die Bestellungen da sind.
      await waitFor(() =>
        expect(
          within(screen.getAllByRole("table")[0]!).getByText("OGS Nord"),
        ).toBeInTheDocument(),
      );

      const olderSchools = deferred<{
        catererName: string;
        schools: { id: string; name: string }[];
      }>();
      mocks.fetchCatererSchools.mockReturnValueOnce(olderSchools.promise);
      fireEvent.click(screen.getByRole("button", { name: "Nächste Woche" }));
      fireEvent.click(screen.getByRole("button", { name: "Morgen" }));
      // Die Karte zeigt die Tabelle schon beim Laden; erst die Zeile
      // der Schule belegt, dass die Bestellungen da sind.
      await waitFor(() =>
        expect(
          within(screen.getAllByRole("table")[0]!).getByText("OGS Nord"),
        ).toBeInTheDocument(),
      );
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
    // Die Karte zeigt die Tabelle schon beim Laden; erst die Zeile
    // der Schule belegt, dass die Bestellungen da sind.
    await waitFor(() =>
      expect(
        within(screen.getAllByRole("table")[0]!).getByText("OGS Nord"),
      ).toBeInTheDocument(),
    );

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
