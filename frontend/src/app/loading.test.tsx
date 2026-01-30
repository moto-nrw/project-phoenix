import { describe, it, expect, vi } from "vitest";
import { render } from "@testing-library/react";
import RootLoadingPage from "./loading";

// Mock Loading component
vi.mock("~/components/ui/loading", () => ({
  Loading: ({ message }: { message: string }) => (
    <div data-testid="loading">
      <span>{message}</span>
    </div>
  ),
}));

describe("RootLoadingPage", () => {
  it("renders Loading component", () => {
    const { getByTestId } = render(<RootLoadingPage />);

    expect(getByTestId("loading")).toBeInTheDocument();
  });

  it("displays German loading message", () => {
    const { getByText } = render(<RootLoadingPage />);

    expect(getByText("Laden...")).toBeInTheDocument();
  });
});
