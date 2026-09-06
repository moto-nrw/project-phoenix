import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { MobileBackButton } from "./mobile-back-button";

const mockPush = vi.fn();
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush }),
}));

describe("MobileBackButton", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders below the desktop header breakpoint", () => {
    render(<MobileBackButton />);

    expect(screen.getByText("Zurück")).toBeInTheDocument();
  });

  it("is hidden when the desktop header is visible", () => {
    render(<MobileBackButton />);

    expect(screen.getByRole("button")).toHaveClass("lg:hidden");
  });

  it("uses default href when not provided", () => {
    render(<MobileBackButton />);

    const button = screen.getByRole("button");
    fireEvent.click(button);

    expect(mockPush).toHaveBeenCalledWith("/database");
  });

  it("uses custom href when provided", () => {
    render(<MobileBackButton href="/groups" />);

    const button = screen.getByRole("button");
    fireEvent.click(button);

    expect(mockPush).toHaveBeenCalledWith("/groups");
  });

  it("has correct aria-label", () => {
    render(<MobileBackButton ariaLabel="Zurück zur Übersicht" />);

    const button = screen.getByLabelText("Zurück zur Übersicht");
    expect(button).toBeInTheDocument();
  });

  it("renders the back icon", () => {
    const { container } = render(<MobileBackButton />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });
});
