import {
  act,
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
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
import { setTestClock, releaseFakeTimers } from "~/test/clock";

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
    vi.resetAllMocks();
    sessionState.data = {
      user: { id: "caterer", scope: "caterer" },
      expires: "2099-01-01",
    } as Session;
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

  it("shares schools and orders on return without a loading replacement", async () => {
    const view = render(<CatererOverviewPage />);
    await screen.findByText("Bestellungen für Küche am Markt");
    expect(
      within(screen.getAllByRole("table")[0]!).getByText("OGS Nord"),
    ).toBeVisible();
    view.rerender(<div />);
    view.rerender(<CatererOverviewPage />);
    expect(screen.getByText("Bestellungen für Küche am Markt")).toBeVisible();
    expect(
      within(screen.getAllByRole("table")[0]!).getByText("OGS Nord"),
    ).toBeVisible();
    expect(mocks.fetchCatererSchools).toHaveBeenCalledTimes(1);
    expect(mocks.fetchCatererOrders).toHaveBeenCalledTimes(2);
  });

  it("shows cached schools immediately and discovers newly supplied schools on a later visit", async () => {
    vi.useFakeTimers();
    try {
      const view = render(<CatererOverviewPage />);
      await act(async () => vi.advanceTimersByTimeAsync(50));
      expect(screen.getByText("Bestellungen für Küche am Markt")).toBeVisible();
      expect(
        within(screen.getAllByRole("table")[0]!).getByText("OGS Nord"),
      ).toBeVisible();
      view.rerender(<div />);
      await act(async () => vi.advanceTimersByTimeAsync(6000));
      releaseFakeTimers();
      const pending = deferred<{
        catererName: string;
        schools: { id: string; name: string }[];
      }>();
      mocks.fetchCatererSchools.mockReturnValueOnce(pending.promise);
      view.rerender(<CatererOverviewPage />);
      expect(screen.getByText("Bestellungen für Küche am Markt")).toBeVisible();
      expect(
        within(screen.getAllByRole("table")[0]!).getByText("OGS Nord"),
      ).toBeVisible();
      await waitFor(() =>
        expect(mocks.fetchCatererSchools).toHaveBeenCalledTimes(2),
      );
      await act(async () => {
        pending.resolve({
          catererName: "Küche am Markt",
          schools: [
            { id: "1", name: "OGS Nord" },
            { id: "2", name: "OGS Süd" },
            { id: "3", name: "OGS Ost" },
          ],
        });
      });
      expect(
        await screen.findByRole("heading", { name: "OGS Ost" }),
      ).toBeVisible();
      expect(mocks.fetchCatererOrders).toHaveBeenCalledWith(
        "3",
        "2026-09-07",
        "2026-09-11",
      );
    } finally {
      if (vi.isFakeTimers()) releaseFakeTimers();
    }
  });

  it("confirms optimistically without a skeleton or school reload, then reads only that school's answer", async () => {
    const pending = deferred<void>();
    mocks.confirmCatererOrder.mockReturnValueOnce(pending.promise);
    render(<CatererOverviewPage />);
    await screen.findByText("Bestellungen für Küche am Markt");
    fireEvent.click(
      screen.getByRole("button", { name: "OGS Nord ausklappen" }),
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: "Erhalten bestätigen" })[0]!,
    );
    expect(screen.getAllByText("Bestätigt").length).toBeGreaterThan(0);
    expect(
      within(screen.getAllByRole("table")[0]!).getByText("OGS Nord"),
    ).toBeVisible();
    expect(screen.queryByText("Wird geladen…")).not.toBeInTheDocument();
    expect(mocks.fetchCatererSchools).toHaveBeenCalledTimes(1);
    expect(mocks.fetchCatererOrders).toHaveBeenCalledTimes(2);
    await act(async () => pending.resolve());
    expect(mocks.confirmCatererOrder).toHaveBeenCalledWith("1", "1");
    expect(mocks.fetchCatererSchools).toHaveBeenCalledTimes(1);
    expect(mocks.fetchCatererOrders).toHaveBeenCalledTimes(3);
    expect(mocks.fetchCatererOrders).toHaveBeenLastCalledWith(
      "1",
      "2026-09-07",
      "2026-09-11",
    );
  });

  it.each(["success", "failure"])(
    "never resurrects an unconfirmed inactive week after a successful POST and GET %s",
    async (refresh) => {
      let confirmed = false;
      const loaded = (from: string, to: string) => {
        const data = week([10, 10, 10, 10, 10]);
        return {
          ...data,
          days: data.days
            .filter((day) => day.date >= from && day.date <= to)
            .map((day) => ({
              ...day,
              status:
                confirmed && day.orderId === "4" ? "confirmed" : day.status,
            })),
        };
      };
      mocks.fetchCatererOrders.mockImplementation(
        (_id: string, from: string, to: string) =>
          Promise.resolve(loaded(from, to)),
      );
      const post = deferred<void>();
      mocks.confirmCatererOrder.mockImplementation(async () => {
        await post.promise;
        confirmed = true;
      });
      render(<CatererOverviewPage />);
      await screen.findByText("Bestellungen für Küche am Markt");
      fireEvent.click(screen.getByRole("button", { name: "Morgen" }));
      await waitFor(() =>
        expect(mocks.fetchCatererOrders).toHaveBeenCalledTimes(4),
      );
      fireEvent.click(
        screen.getByRole("button", { name: "OGS Nord ausklappen" }),
      );
      const north = screen
        .getByRole("heading", { name: "OGS Nord" })
        .closest("section")!;
      const confirm = within(north).getAllByRole("button", {
        name: "Erhalten bestätigen",
      })[0]!;
      expect(confirm).toBeVisible();
      if (refresh === "failure")
        mocks.fetchCatererOrders.mockRejectedValueOnce(
          new ApiError("down", 503),
        );
      fireEvent.click(confirm);
      await act(async () => post.resolve());
      expect(within(north).getAllByText("Bestätigt").length).toBeGreaterThan(0);
      const next = deferred<ReturnType<typeof loaded>>();
      mocks.fetchCatererOrders.mockReturnValue(next.promise);
      fireEvent.click(screen.getByRole("button", { name: "Diese Woche" }));
      expect(screen.getByText("Wird geladen…")).toBeVisible();
      await act(async () => next.resolve(loaded("2026-09-07", "2026-09-11")));
      expect(within(north).getAllByText("Bestätigt").length).toBeGreaterThan(0);
      expect(mocks.confirmCatererOrder).toHaveBeenCalledTimes(1);
      expect(mocks.fetchCatererSchools).toHaveBeenCalledTimes(1);
    },
  );

  it("does not reopen cached orders after another window refused school access", async () => {
    render(<CatererOverviewPage />);
    expect(
      await screen.findByText("Bestellungen für Küche am Markt"),
    ).toBeVisible();
    expect(
      within(screen.getAllByRole("table")[0]!).getByText("OGS Nord"),
    ).toBeVisible();
    mocks.fetchCatererOrders.mockRejectedValueOnce(
      new ApiError("revoked", 403, { code: "meals.not_supplied" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Morgen" }));
    await waitFor(() =>
      expect(mocks.fetchCatererOrders).toHaveBeenCalledTimes(4),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "OGS Nord ausklappen" }),
    );
    const north = screen
      .getByRole("heading", { name: "OGS Nord" })
      .closest("section")!;
    expect(await within(north).findByRole("alert")).toHaveTextContent(
      "Sie beliefern diese Schule nicht mehr",
    );
    mocks.fetchCatererOrders.mockImplementation(
      () => new Promise(() => undefined),
    );
    fireEvent.click(screen.getByRole("button", { name: "Diese Woche" }));
    expect(screen.getByText("Wird geladen…")).toBeVisible();
    expect(
      within(north).queryByRole("button", { name: "Erhalten bestätigen" }),
    ).not.toBeInTheDocument();
  });

  it("rolls back a failed confirmation without removing the loaded orders", async () => {
    const pending = deferred<void>();
    mocks.confirmCatererOrder.mockReturnValueOnce(pending.promise);
    render(<CatererOverviewPage />);
    await screen.findByText("Bestellungen für Küche am Markt");
    fireEvent.click(
      screen.getByRole("button", { name: "OGS Nord ausklappen" }),
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: "Erhalten bestätigen" })[0]!,
    );
    expect(screen.getAllByText("Bestätigt").length).toBeGreaterThan(0);
    await act(async () => pending.reject(new ApiError("down", 503)));
    expect(
      screen.getAllByRole("button", { name: "Erhalten bestätigen" }).length,
    ).toBeGreaterThan(0);
    expect(
      within(screen.getAllByRole("table")[0]!).getByText("OGS Nord"),
    ).toBeVisible();
    expect(screen.queryByText("Bestätigt")).not.toBeInTheDocument();
    expect(mocks.fetchCatererSchools).toHaveBeenCalledTimes(1);
    expect(mocks.fetchCatererOrders).toHaveBeenCalledTimes(2);
  });

  it.each(["account", "logout", "other-tab-account", "other-tab-logout"])(
    "withdraws old schools, orders and pending confirmations on %s",
    async (change) => {
      const pending = deferred<void>();
      mocks.confirmCatererOrder.mockReturnValueOnce(pending.promise);
      const view = render(<CatererOverviewPage />);
      expect(
        await screen.findByText("Bestellungen für Küche am Markt"),
      ).toBeVisible();
      fireEvent.click(
        screen.getByRole("button", { name: "OGS Nord ausklappen" }),
      );
      fireEvent.click(
        screen.getAllByRole("button", { name: "Erhalten bestätigen" })[0]!,
      );
      mocks.fetchCatererSchools.mockImplementation(
        () => new Promise(() => undefined),
      );
      if (change.startsWith("other-tab")) {
        act(() =>
          globalThis.dispatchEvent(
            new StorageEvent("storage", {
              key: "moto:personal-storage-owner",
              newValue: change.endsWith("logout") ? null : "B",
            }),
          ),
        );
      } else {
        sessionState.data =
          change === "logout"
            ? null
            : ({
                user: { id: "B", scope: "caterer" },
                expires: "2099-01-01",
              } as Session);
        view.rerender(<CatererOverviewPage />);
      }
      expect(
        screen.queryByText("Bestellungen für Küche am Markt"),
      ).not.toBeInTheDocument();
      expect(screen.queryByText("OGS Nord")).not.toBeInTheDocument();
      await act(async () => pending.resolve());
      expect(
        screen.queryByText("Bestellungen für Küche am Markt"),
      ).not.toBeInTheDocument();
      expect(screen.queryByText("OGS Nord")).not.toBeInTheDocument();
      expect(mocks.fetchCatererOrders).toHaveBeenCalledTimes(2);
    },
  );

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

  it("preserves both schools when failed orders are retried concurrently", async () => {
    mocks.fetchCatererOrders.mockRejectedValueOnce(new ApiError("north", 503));
    mocks.fetchCatererOrders.mockRejectedValueOnce(new ApiError("south", 503));
    render(<CatererOverviewPage />);
    await screen.findByText("Bestellungen für Küche am Markt");
    fireEvent.click(
      screen.getByRole("button", { name: "OGS Nord ausklappen" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "OGS Süd ausklappen" }));
    const north = screen
      .getByRole("heading", { name: "OGS Nord" })
      .closest("section")!;
    const south = screen
      .getByRole("heading", { name: "OGS Süd" })
      .closest("section")!;
    const retryNorth = await within(north).findByRole("button", {
      name: "Wiederholen",
    });
    const retrySouth = await within(south).findByRole("button", {
      name: "Wiederholen",
    });
    const first = deferred<ReturnType<typeof week>>();
    const second = deferred<ReturnType<typeof week>>();
    mocks.fetchCatererOrders
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise);
    fireEvent.click(retryNorth);
    fireEvent.click(retrySouth);
    await act(async () => first.resolve(week([10, 10, 10, 10, 10])));
    await act(async () => second.resolve(week([5, 5, 5, 5, 5])));
    const table = within(screen.getAllByRole("table")[0]!);
    expect(
      within(table.getByText("OGS Nord").closest("tr")!).getByText("50"),
    ).toBeVisible();
    expect(
      within(table.getByText("OGS Süd").closest("tr")!).getByText("25"),
    ).toBeVisible();
    expect(table.getByText("75")).toBeVisible();
    expect(
      within(north).queryByRole("button", { name: "Wiederholen" }),
    ).not.toBeInTheDocument();
    expect(
      within(south).queryByRole("button", { name: "Wiederholen" }),
    ).not.toBeInTheDocument();
  });

  it("keeps a successful confirmation when its refresh fails and lets the school retry", async () => {
    mocks.confirmCatererOrder.mockResolvedValueOnce(undefined);
    render(<CatererOverviewPage />);
    await screen.findByText("Bestellungen für Küche am Markt");
    fireEvent.click(
      screen.getByRole("button", { name: "OGS Nord ausklappen" }),
    );
    const north = screen
      .getByRole("heading", { name: "OGS Nord" })
      .closest("section")!;
    mocks.fetchCatererOrders.mockRejectedValueOnce(
      new ApiError("refresh down", 503),
    );
    fireEvent.click(
      within(north).getAllByRole("button", { name: "Erhalten bestätigen" })[0]!,
    );
    const retry = await within(north).findByRole("button", {
      name: "Wiederholen",
    });
    expect(within(north).getAllByText("Bestätigt").length).toBeGreaterThan(0);
    expect(
      within(screen.getAllByRole("table")[0]!).getByText("OGS Nord"),
    ).toBeVisible();
    const fresh = week([10, 10, 10, 10, 10]);
    fresh.days[0]!.status = "confirmed";
    mocks.fetchCatererOrders.mockResolvedValueOnce(fresh);
    fireEvent.click(retry);
    await waitFor(() =>
      expect(mocks.fetchCatererOrders).toHaveBeenCalledTimes(4),
    );
    expect(within(north).getAllByText("Bestätigt").length).toBeGreaterThan(0);
    expect(mocks.confirmCatererOrder).toHaveBeenCalledTimes(1);
    expect(mocks.fetchCatererSchools).toHaveBeenCalledTimes(1);
  });

  it("drops confirmed orders when the subsequent refresh refuses school access", async () => {
    mocks.confirmCatererOrder.mockResolvedValueOnce(undefined);
    render(<CatererOverviewPage />);
    await screen.findByText("Bestellungen für Küche am Markt");
    fireEvent.click(
      screen.getByRole("button", { name: "OGS Nord ausklappen" }),
    );
    const north = screen
      .getByRole("heading", { name: "OGS Nord" })
      .closest("section")!;
    expect(
      within(north).getAllByRole("button", { name: "Erhalten bestätigen" })
        .length,
    ).toBeGreaterThan(0);
    mocks.fetchCatererOrders.mockRejectedValueOnce(
      new ApiError("revoked", 403, { code: "meals.not_supplied" }),
    );
    fireEvent.click(
      within(north).getAllByRole("button", { name: "Erhalten bestätigen" })[0]!,
    );
    const messages = await screen.findAllByText(
      "Sie beliefern diese Schule nicht mehr. Bitte fragen Sie die Schule.",
    );
    expect(messages[0]).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Erhalten bestätigen" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Bestätigt")).not.toBeInTheDocument();
    expect(mocks.confirmCatererOrder).toHaveBeenCalledTimes(1);
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
    "keeps the current period when an older order request settles last (%s)",
    async (outcome) => {
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
          olderOrders.resolve(week([99, 99, 99, 99, 99]));
        } else {
          olderOrders.reject(new Error("Previous period failed"));
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

    const currentOrders = deferred<ReturnType<typeof week>>();
    mocks.fetchCatererOrders.mockReturnValueOnce(currentOrders.promise);
    fireEvent.click(screen.getByRole("button", { name: "Morgen" }));
    expect(screen.getByText("Wird geladen…")).toBeInTheDocument();

    await act(async () => {
      olderOrders.resolve(week([99, 99, 99, 99, 99]));
    });
    expect(screen.getByText("Wird geladen…")).toBeInTheDocument();

    await act(async () => {
      currentOrders.resolve(week([10, 10, 10, 10, 10]));
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

vi.unmock("swr");
import { SWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import type { Session } from "next-auth";
import type { ReactElement } from "react";
const sessionState = vi.hoisted(() => ({ data: null as Session | null }));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: sessionState.data,
    status: sessionState.data ? "authenticated" : "unauthenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({ useTenantSlugSafe: () => null }));
function render(ui: ReactElement, _options?: unknown) {
  const cache = new Map();
  return rtlRender(ui, {
    wrapper: ({ children }) => (
      <SWRConfig value={{ provider: () => cache, shouldRetryOnError: false }}>
        <ToastProvider>
          <SWRScopeBoundary />
          {children}
        </ToastProvider>
      </SWRConfig>
    ),
  });
}

import { resumeSWRSession } from "~/lib/swr/scope";
afterEach(() => resumeSWRSession());
