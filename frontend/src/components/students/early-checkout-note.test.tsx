import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
} from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";

const { tenantState } = vi.hoisted(() => ({
  tenantState: { tolerance: 15 as number | null },
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSafe: () => ({
    tenant: { earlyCheckoutNoteToleranceMinutes: tenantState.tolerance },
  }),
}));

import {
  useEarlyCheckoutCheck,
  useEarlyCheckoutDialog,
} from "./early-checkout-note";

// The test clock stands at Wednesday 2026-09-09 12:00 Berlin.

describe("useEarlyCheckoutCheck (#3324)", () => {
  beforeEach(() => {
    tenantState.tolerance = 15;
  });

  it("flags a checkout more than the tolerance before the pickup time", () => {
    const { result } = renderHook(() => useEarlyCheckoutCheck());
    expect(result.current("15:00")).toBe(true);
    expect(result.current("12:15")).toBe(false);
    expect(result.current("12:16")).toBe(true);
    expect(result.current(undefined)).toBe(false);
  });

  it("never flags when the school switched the question off", () => {
    tenantState.tolerance = null;
    const { result } = renderHook(() => useEarlyCheckoutCheck());
    expect(result.current("15:00")).toBe(false);
  });
});

function DialogHarness({
  onConfirm,
  plannedPickup,
  room = null,
}: Readonly<{
  onConfirm: (studentId: string, note: string) => void;
  plannedPickup?: string;
  room?: string | null;
}>) {
  const { request, dialog } = useEarlyCheckoutDialog(onConfirm);
  return (
    <>
      <button
        type="button"
        onClick={() => {
          const handled = request({
            studentId: "7",
            studentName: "Mia Beispiel",
            plannedPickup,
            room,
          });
          if (!handled) onConfirm("7", "direct");
        }}
      >
        Tippen
      </button>
      {dialog}
    </>
  );
}

describe("useEarlyCheckoutDialog (#3324)", () => {
  beforeEach(() => {
    tenantState.tolerance = 15;
  });

  it("asks for an optional reason and passes it on", () => {
    const onConfirm = vi.fn();
    render(<DialogHarness onConfirm={onConfirm} plannedPickup="15:00:00" />);

    fireEvent.click(screen.getByRole("button", { name: "Tippen" }));

    expect(screen.getByText("Kind früher abmelden")).toBeInTheDocument();
    expect(screen.getByText("15:00 Uhr")).toBeInTheDocument();
    expect(
      screen.getByText("Der Grund steht danach beim Kind."),
    ).toBeInTheDocument();
    fireEvent.change(
      screen.getByLabelText("Grund für das frühe Gehen (freiwillig)"),
      { target: { value: "Arzttermin" } },
    );
    act(() => {
      fireEvent.click(screen.getByRole("button", { name: "Abmelden" }));
    });

    expect(onConfirm).toHaveBeenCalledWith("7", "Arzttermin");
  });

  it("lets staff check out without writing a reason", () => {
    const onConfirm = vi.fn();
    render(<DialogHarness onConfirm={onConfirm} plannedPickup="15:00" />);

    fireEvent.click(screen.getByRole("button", { name: "Tippen" }));
    fireEvent.click(screen.getByRole("button", { name: "Abmelden" }));

    expect(onConfirm).toHaveBeenCalledWith("7", "");
  });

  it("names the room whose visit the checkout ends", () => {
    render(
      <DialogHarness
        onConfirm={vi.fn()}
        plannedPickup="15:00"
        room="Atelier"
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Tippen" }));
    expect(screen.getByText("Atelier")).toBeInTheDocument();
  });

  it("keeps an ordinary checkout a single tap", () => {
    const onConfirm = vi.fn();
    render(<DialogHarness onConfirm={onConfirm} plannedPickup="12:10" />);

    fireEvent.click(screen.getByRole("button", { name: "Tippen" }));

    expect(screen.queryByText("Kind früher abmelden")).not.toBeInTheDocument();
    expect(onConfirm).toHaveBeenCalledWith("7", "direct");
  });
});
