import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import ProtectedLoadingPage from "./loading";

vi.mock("~/components/ui/loading", () => ({
  Loading: ({
    message,
    fullPage,
  }: {
    message?: string;
    fullPage?: boolean;
  }) => (
    <div data-testid="loading" data-full-page={String(fullPage)}>
      {message}
    </div>
  ),
}));

describe("ProtectedLoadingPage", () => {
  it("renders Loading component with correct props", () => {
    render(<ProtectedLoadingPage />);

    const loading = screen.getByTestId("loading");
    expect(loading).toBeInTheDocument();
    expect(loading).toHaveAttribute("data-full-page", "false");
    expect(loading).toHaveTextContent("Laden...");
  });
});
