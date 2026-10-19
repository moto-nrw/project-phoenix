import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  fetchCatererOrders: vi.fn(),
  confirmCatererOrder: vi.fn(),
}));

vi.mock("~/lib/caterer-api", () => ({
  fetchCatererOrders: mocks.fetchCatererOrders,
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

function renderOrders() {
  return render(
    <CatererSchoolOrders
      school={school}
      mondayISO="2026-09-07"
      fridayISO="2026-09-11"
    />,
    { wrapper: ToastProvider },
  );
}

describe("CatererSchoolOrders", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.confirmCatererOrder.mockResolvedValue(undefined);
  });

  it("shows numbers only, and one confirmation per order (ADR 0074)", async () => {
    // A weekly order spans two days with one order ID.
    mocks.fetchCatererOrders.mockResolvedValue({
      school,
      deliveryAddress: "Schulstraße 1",
      rhythm: "weekly",
      detail: "total",
      days: [day("2026-09-07"), day("2026-09-08")],
    });
    renderOrders();

    expect(
      await screen.findByText("Lieferadresse: Schulstraße 1"),
    ).toBeInTheDocument();
    expect(mocks.fetchCatererOrders).toHaveBeenCalledWith(
      "42",
      "2026-09-07",
      "2026-09-11",
    );
    expect(screen.queryByText("Je Klasse")).not.toBeInTheDocument();
    expect(screen.queryByText("Kinder")).not.toBeInTheDocument();
    const buttons = screen.getAllByRole("button", {
      name: "Erhalten bestätigen",
    });
    // The table may render a desktop and a stacked mobile copy of each row:
    // every copy of the button belongs to Monday, the order's first day.
    for (const button of buttons) {
      expect(button.closest("tr, li")?.textContent).toContain("Montag");
    }

    fireEvent.click(buttons[0]!);
    await waitFor(() =>
      expect(mocks.confirmCatererOrder).toHaveBeenCalledWith("42", "7"),
    );
    expect(mocks.fetchCatererOrders).toHaveBeenCalledTimes(2);
  });

  it("shows classes and names only when the school shares them", async () => {
    mocks.fetchCatererOrders.mockResolvedValue({
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
    });
    renderOrders();

    expect(await screen.findAllByText("Lea Koch (4b)")).not.toHaveLength(0);
    expect(screen.getAllByText("4b: 1")).not.toHaveLength(0);
    expect(screen.getAllByText("+1 Ben Neu (2a)")).not.toHaveLength(0);
    expect(
      screen.queryByRole("button", { name: "Erhalten bestätigen" }),
    ).not.toBeInTheDocument();
  });
});
