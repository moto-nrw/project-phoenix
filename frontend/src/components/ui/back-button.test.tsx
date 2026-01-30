import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { BackButton } from "./back-button";

// Mock next/navigation
const mockPush = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
  }),
}));

describe("BackButton", () => {
  it("renders the back button", () => {
    render(<BackButton referrer="/dashboard" />);

    expect(screen.getByText("Zurück")).toBeInTheDocument();
  });

  it("renders the back icon", () => {
    const { container } = render(<BackButton referrer="/dashboard" />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("calls router.push with referrer on click", () => {
    render(<BackButton referrer="/dashboard" />);

    const button = screen.getByRole("button");
    fireEvent.click(button);

    expect(mockPush).toHaveBeenCalledWith("/dashboard");
  });

  it("has md:hidden class for mobile-only visibility", () => {
    render(<BackButton referrer="/groups" />);

    const button = screen.getByRole("button");
    expect(button).toHaveClass("md:hidden");
  });
});
