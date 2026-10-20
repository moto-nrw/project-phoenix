import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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

describe("CatererSchoolOrders", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.confirmCatererOrder.mockResolvedValue(undefined);
    mocks.onConfirmed.mockResolvedValue(undefined);
  });

  it("shows numbers only, and one confirmation per order (ADR 0075)", async () => {
    // A weekly order spans two days with one order ID.
    renderOrders({
      school,
      deliveryAddress: "Schulstraße 1",
      rhythm: "weekly",
      detail: "total",
      days: [day("2026-09-07"), day("2026-09-08")],
    } as never);

    expect(screen.queryByText("Je Klasse")).not.toBeInTheDocument();
    expect(screen.queryByText("Kinder")).not.toBeInTheDocument();
    const buttons = await screen.findAllByRole("button", {
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
    expect(mocks.onConfirmed).toHaveBeenCalled();
  });

  it("shows classes and names only when the school shares them", async () => {
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

    expect(await screen.findAllByText("Lea Koch (4b)")).not.toHaveLength(0);
    expect(screen.getAllByText("4b: 1")).not.toHaveLength(0);
    expect(screen.getAllByText("+1 Ben Neu (2a)")).not.toHaveLength(0);
    expect(
      screen.queryByRole("button", { name: "Erhalten bestätigen" }),
    ).not.toBeInTheDocument();
  });
});
