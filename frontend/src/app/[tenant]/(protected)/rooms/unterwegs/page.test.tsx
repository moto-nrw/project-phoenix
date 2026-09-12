import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import TransitPage from "./page";

const { presenceModeState, searchParams } = vi.hoisted(() => ({
  presenceModeState: { mode: "detailed" as string },
  searchParams: new URLSearchParams(),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => searchParams,
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
  TransitStudentsSection: ({
    fromReferrer,
    onTotalCountChange,
  }: {
    fromReferrer?: string;
    onTotalCountChange?: (count: number | null) => void;
  }) => (
    <div
      data-testid="transit-section"
      data-from={fromReferrer}
      data-count-handler={onTotalCountChange ? "yes" : "no"}
    />
  ),
}));

vi.mock("~/components/ui/mobile-back-button", () => ({
  MobileBackButton: ({ href }: { href?: string }) => (
    <div data-testid="back-button" data-href={href} />
  ),
}));

describe("TransitPage", () => {
  beforeEach(() => {
    presenceModeState.mode = "detailed";
    searchParams.delete("from");
  });

  it("renders the transit list as its own page with the way back to the rooms", () => {
    presenceModeState.mode = "detailed";
    render(<TransitPage />);

    expect(
      screen.getByRole("heading", { name: "Unterwegs" }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("transit-section")).toHaveAttribute(
      "data-from",
      `/rooms/unterwegs?from=${encodeURIComponent("/rooms")}`,
    );
    expect(screen.getByTestId("transit-section")).toHaveAttribute(
      "data-count-handler",
      "yes",
    );
  });

  it("returns to the filtered room collection", () => {
    searchParams.set("from", "/rooms?building=Nord&status=occupied");
    render(<TransitPage />);

    expect(screen.getByTestId("back-button")).toHaveAttribute(
      "data-href",
      "/rooms?building=Nord&status=occupied",
    );
    expect(screen.getByTestId("transit-section")).toHaveAttribute(
      "data-from",
      `/rooms/unterwegs?from=${encodeURIComponent("/rooms?building=Nord&status=occupied")}`,
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
