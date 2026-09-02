import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { MobileBackButton } from "./mobile-back-button";

// Mock useIsMobile hook
const mockIsMobile = vi.fn();
vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: (): boolean => mockIsMobile() as boolean,
}));

const mockPush = vi.fn();
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush }),
}));

describe("MobileBackButton", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders on mobile", () => {
    mockIsMobile.mockReturnValue(true);

    render(<MobileBackButton />);

    expect(screen.getByText("Zurück")).toBeInTheDocument();
  });

  it("does not render on desktop", () => {
    mockIsMobile.mockReturnValue(false);

    const { container } = render(<MobileBackButton />);

    expect(container.firstChild).toBeNull();
  });

  it("uses default href when not provided", () => {
    mockIsMobile.mockReturnValue(true);

    render(<MobileBackButton />);

    const button = screen.getByRole("button");
    fireEvent.click(button);

    expect(mockPush).toHaveBeenCalledWith("/database");
  });

  it("uses custom href when provided", () => {
    mockIsMobile.mockReturnValue(true);

    render(<MobileBackButton href="/groups" />);

    const button = screen.getByRole("button");
    fireEvent.click(button);

    expect(mockPush).toHaveBeenCalledWith("/groups");
  });

  it("has correct aria-label", () => {
    mockIsMobile.mockReturnValue(true);

    render(<MobileBackButton ariaLabel="Zurück zur Übersicht" />);

    const button = screen.getByLabelText("Zurück zur Übersicht");
    expect(button).toBeInTheDocument();
  });

  it("renders the back icon", () => {
    mockIsMobile.mockReturnValue(true);

    const { container } = render(<MobileBackButton />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });
});
