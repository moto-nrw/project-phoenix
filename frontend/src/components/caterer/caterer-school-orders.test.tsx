import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  confirmCatererOrder: vi.fn(),
  onConfirmed: vi.fn(),
}));

vi.mock("~/lib/caterer-api", () => ({
  confirmCatererOrder: mocks.confirmCatererOrder,
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { CatererSchoolOrders } from "./caterer-school-orders";

const school = { id: "42", name: "OGS am Markt" };

function day(date: string, overrides: Record<string, unknown> = {}) {
  return {
    date,
    closed: false,
    deadline: `${date}T07:00:00Z`,
    status: "ordered",
    orderId: "7",
    confirmedAt: null,
    total: 2,
    classes: [],
    children: [],
    changes: [],
    ...overrides,
  };
}

function renderOrders(
  orders: Parameters<typeof CatererSchoolOrders>[0]["orders"],
) {
  return render(
    <CatererSchoolOrders
      school={school}
      orders={orders}
      onConfirmed={mocks.onConfirmed}
    />,
    { wrapper: ToastProvider },
  );
}

/** The desktop table row whose first cell names it („Stand“, „Klasse 1a“). */
function tableRow(label: string): HTMLTableRowElement {
  return screen
    .getAllByText(label)
    .map((cell) => cell.closest("tr"))
    .find(Boolean)!;
}

/** The cell of a desktop row under a day's column header. */
function dayCell(label: string, day: string): HTMLElement {
  const header = screen.getByRole("columnheader", { name: day });
  const column = [...header.closest("tr")!.children].indexOf(header);
  return tableRow(label).children[column] as HTMLElement;
}

function change(sequence: number, schoolClass: string, delta: number) {
  return {
    sequence,
    child: { firstName: "", lastName: "", schoolClass },
    delta,
    detectedAt: "2026-09-07T08:00:00Z",
  };
}

describe("CatererSchoolOrders", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.confirmCatererOrder.mockResolvedValue(undefined);
    mocks.onConfirmed.mockResolvedValue(undefined);
  });

  it("shows numbers only, and one confirmation per order (ADR 0077)", async () => {
    // A weekly order spans two days with one order ID.
    renderOrders({
      school,
      deliveryAddress: "Schulstraße 1",
      rhythm: "weekly",
      detail: "total",
      days: [day("2026-09-07"), day("2026-09-08")],
    } as never);

    expect(screen.queryByText(/^Klasse /)).not.toBeInTheDocument();
    expect(screen.queryByText("Namen")).not.toBeInTheDocument();
    expect(
      within(dayCell("Essen", "Mo., 07.09.")).getByText("2"),
    ).toBeInTheDocument();
    const buttons = await screen.findAllByRole("button", {
      name: "Erhalten bestätigen",
    });
    // Desktop: the button sits in Monday's column of the state row only.
    expect(
      within(dayCell("Stand", "Mo., 07.09.")).getByRole("button", {
        name: "Erhalten bestätigen",
      }),
    ).toBeInTheDocument();
    expect(
      within(dayCell("Stand", "Di., 08.09.")).getByText("Bestellt"),
    ).toBeInTheDocument();

    fireEvent.click(buttons[0]!);
    await waitFor(() =>
      expect(mocks.confirmCatererOrder).toHaveBeenCalledWith("42", "7"),
    );
    expect(mocks.onConfirmed).toHaveBeenCalled();
  });

  it("confirms in the state row and shows the green badge there afterwards", async () => {
    const orders = (status: string) =>
      ({
        school,
        deliveryAddress: "",
        rhythm: "weekly",
        detail: "total",
        days: [day("2026-09-07", { status }), day("2026-09-08", { status })],
      }) as never;
    const view = renderOrders(orders("ordered"));

    const stateCell = dayCell("Stand", "Mo., 07.09.");
    expect(
      within(stateCell).getByRole("button", { name: "Erhalten bestätigen" }),
    ).toBeInTheDocument();

    view.rerender(
      <CatererSchoolOrders
        school={school}
        orders={orders("confirmed")}
        onConfirmed={mocks.onConfirmed}
      />,
    );
    expect(within(stateCell).getByText("Bestätigt")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Erhalten bestätigen" }),
    ).not.toBeInTheDocument();
  });

  it("counts the changes after the deadline per class instead of listing them", async () => {
    renderOrders({
      school,
      deliveryAddress: "",
      rhythm: "daily",
      detail: "by_class",
      days: [
        day("2026-09-07", {
          total: 5,
          classes: [
            { schoolClass: "2a", count: 3 },
            { schoolClass: "10a", count: 2 },
          ],
          changes: [
            change(1, "2a", 1),
            change(2, "2a", 1),
            change(3, "2a", -1),
            change(4, "4b", 1),
          ],
        }),
        day("2026-09-08", {
          total: 4,
          classes: [{ schoolClass: "2a", count: 4 }],
        }),
      ],
    } as never);

    // Classes in school order, a class seen only in a change included.
    const labels = [...document.querySelectorAll("tbody tr")].map(
      (row) => row.children[0]!.textContent,
    );
    expect(labels).toEqual([
      "Stand",
      "Klasse 2a",
      "Klasse 4b",
      "Klasse 10a",
      "Essen gesamt",
    ]);
    const monday2a = dayCell("Klasse 2a", "Mo., 07.09.");
    expect(within(monday2a).getByText("3")).toBeInTheDocument();
    expect(within(monday2a).getByText("+2")).toBeInTheDocument();
    expect(within(monday2a).getByText("−1")).toBeInTheDocument();
    const monday4b = dayCell("Klasse 4b", "Mo., 07.09.");
    expect(within(monday4b).getByText("0")).toBeInTheDocument();
    expect(within(monday4b).getByText("+1")).toBeInTheDocument();
    const mondayTotal = dayCell("Essen gesamt", "Mo., 07.09.");
    expect(within(mondayTotal).getByText("5")).toBeInTheDocument();
    expect(within(mondayTotal).getByText("+3")).toBeInTheDocument();
    expect(within(mondayTotal).getByText("−1")).toBeInTheDocument();
    // The ordered sum of the window, per class and in total.
    expect(within(tableRow("Klasse 2a")).getAllByText("7")).not.toHaveLength(0);
    expect(within(tableRow("Essen gesamt")).getByText("9")).toBeInTheDocument();
    expect(screen.queryByText(/\+1 .*2a/)).not.toBeInTheDocument();
  });

  it("shows names only when the school shares them and someone opens them", async () => {
    renderOrders({
      school,
      deliveryAddress: "",
      rhythm: "daily",
      detail: "by_class",
      days: [
        day("2026-09-07", {
          status: "confirmed",
          classes: [{ schoolClass: "4b", count: 1 }],
          children: [{ firstName: "Lea", lastName: "Koch", schoolClass: "4b" }],
          changes: [
            {
              sequence: 1,
              child: { firstName: "Ben", lastName: "Neu", schoolClass: "2a" },
              delta: 1,
              detectedAt: "2026-09-07T08:00:00Z",
            },
          ],
        }),
      ],
    } as never);

    // Die Namen sind zugeklappt, bis jemand sie für den Tag aufklappt.
    const show = await screen.findAllByRole("button", {
      name: "2 Namen anzeigen",
    });
    expect(screen.queryByText("Lea Koch (4b)")).not.toBeInTheDocument();
    expect(show[0]).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(show[0]!);
    expect(screen.getAllByText("Lea Koch (4b)")).not.toHaveLength(0);
    expect(screen.getAllByText("+1 Ben Neu (2a)")).not.toHaveLength(0);
    expect(
      screen.getAllByRole("button", { name: "Namen ausblenden" })[0],
    ).toHaveAttribute("aria-expanded", "true");
    expect(
      within(dayCell("Klasse 4b", "Mo., 07.09.")).getByText("1"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Erhalten bestätigen" }),
    ).not.toBeInTheDocument();
  });

  it("shows an open day with its deadline and nothing to confirm", async () => {
    renderOrders({
      school,
      deliveryAddress: "",
      rhythm: "daily",
      detail: "total",
      days: [
        day("2026-09-10", { status: "open", orderId: null }),
        // Die Frist für Montag liegt am Freitag davor.
        day("2026-09-14", {
          status: "open",
          orderId: null,
          deadline: "2026-09-11T07:00:00Z",
        }),
      ],
    } as never);

    expect(await screen.findAllByText("Offen")).not.toHaveLength(0);
    // Die Frist am selben Tag braucht nur die Uhrzeit.
    expect(screen.getAllByText("bis 09:00")).not.toHaveLength(0);
    expect(screen.getAllByText("bis Fr., 11.09., 09:00")).not.toHaveLength(0);
    expect(
      screen.queryByRole("button", { name: "Erhalten bestätigen" }),
    ).not.toBeInTheDocument();
  });

  it("names a class once when the school already writes „Klasse“", async () => {
    renderOrders({
      school,
      deliveryAddress: "",
      rhythm: "daily",
      detail: "by_class",
      days: [
        day("2026-09-07", {
          classes: [{ schoolClass: "Klasse 1a", count: 2 }],
        }),
      ],
    } as never);

    expect(await screen.findAllByText("Klasse 1a")).not.toHaveLength(0);
    expect(screen.queryByText("Klasse Klasse 1a")).not.toBeInTheDocument();
  });
});
