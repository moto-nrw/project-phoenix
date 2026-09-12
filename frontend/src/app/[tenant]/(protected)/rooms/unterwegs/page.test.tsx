import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import TransitPage from "./page";

const { presenceModeState } = vi.hoisted(() => ({
  presenceModeState: { mode: "detailed" as string },
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
}));

vi.mock("~/lib/tenant-context", () => ({
  usePresenceMode: () => presenceModeState.mode,
  useTenantSafe: () => null,
  useTenantSlugSafe: () => null,
  useTenantRoutingModeSafe: () => "subdomain",
}));

vi.mock("~/components/rooms/transit-students-section", () => ({
  TransitStudentsSection: ({ fromReferrer }: { fromReferrer?: string }) => (
    <div data-testid="transit-section" data-from={fromReferrer} />
  ),
}));

describe("TransitPage", () => {
  it("renders the transit list as its own page with the way back to the rooms", () => {
    presenceModeState.mode = "detailed";
    render(<TransitPage />);

    expect(
      screen.getByRole("heading", { name: "Unterwegs" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Kinder ohne Raumzuweisung")).toBeInTheDocument();
    expect(screen.getByTestId("transit-section")).toHaveAttribute(
      "data-from",
      "/rooms/unterwegs",
    );
  });

  it("is switched off in binary presence mode", () => {
    presenceModeState.mode = "binary";
    render(<TransitPage />);

    expect(screen.queryByTestId("transit-section")).not.toBeInTheDocument();
    expect(
      screen.getByText(/erfasst nur, ob ein Kind da ist/),
    ).toBeInTheDocument();
  });
});
