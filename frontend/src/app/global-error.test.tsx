import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("@sentry/nextjs", () => ({
  captureException: vi.fn(),
}));

import * as Sentry from "@sentry/nextjs";
import GlobalError from "./global-error";

describe("GlobalError", () => {
  const mockReset = vi.fn();
  const testError = new Error("Something went wrong");

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("reports the error to Sentry on mount", () => {
    render(<GlobalError error={testError} reset={mockReset} />);

    expect(Sentry.captureException).toHaveBeenCalledWith(testError);
  });

  it("renders the German error message", () => {
    render(<GlobalError error={testError} reset={mockReset} />);

    expect(
      screen.getByText("Ein unerwarteter Fehler ist aufgetreten"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Bitte versuchen Sie es erneut/),
    ).toBeInTheDocument();
  });

  it("renders the retry button", () => {
    render(<GlobalError error={testError} reset={mockReset} />);

    expect(
      screen.getByRole("button", { name: "Erneut versuchen" }),
    ).toBeInTheDocument();
  });

  it("calls reset when the retry button is clicked", () => {
    render(<GlobalError error={testError} reset={mockReset} />);

    fireEvent.click(screen.getByRole("button", { name: "Erneut versuchen" }));

    expect(mockReset).toHaveBeenCalledOnce();
  });

  it("reports a new error when the error prop changes", () => {
    const { rerender } = render(
      <GlobalError error={testError} reset={mockReset} />,
    );

    expect(Sentry.captureException).toHaveBeenCalledTimes(1);

    const newError = new Error("Another error");
    rerender(<GlobalError error={newError} reset={mockReset} />);

    expect(Sentry.captureException).toHaveBeenCalledTimes(2);
    expect(Sentry.captureException).toHaveBeenLastCalledWith(newError);
  });
});
