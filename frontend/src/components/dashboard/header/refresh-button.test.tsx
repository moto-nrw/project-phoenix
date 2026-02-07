/**
 * Tests for RefreshButton Component
 * Tests data refresh trigger, loading state, and animation feedback
 */
import { render, screen, fireEvent, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { RefreshButton } from "./refresh-button";

// Mock SWR config
const mockMutate = vi.fn().mockResolvedValue(undefined);
vi.mock("swr", () => ({
  useSWRConfig: () => ({ mutate: mockMutate }),
}));

describe("RefreshButton", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockMutate.mockClear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders with correct aria-label", () => {
    render(<RefreshButton />);

    const button = screen.getByRole("button", {
      name: "Daten aktualisieren",
    });
    expect(button).toBeInTheDocument();
  });

  it("renders RotateCw icon", () => {
    const { container } = render(<RefreshButton />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("does not show spin animation initially", () => {
    const { container } = render(<RefreshButton />);

    const svg = container.querySelector("svg");
    expect(svg).not.toHaveClass("animate-spin");
  });

  it("calls SWR mutate on click", async () => {
    render(<RefreshButton />);

    await act(async () => {
      fireEvent.click(
        screen.getByRole("button", { name: "Daten aktualisieren" }),
      );
    });

    expect(mockMutate).toHaveBeenCalledWith(expect.any(Function), undefined, {
      revalidate: true,
    });
  });

  it("shows spin animation during refresh", async () => {
    render(<RefreshButton />);

    await act(async () => {
      fireEvent.click(
        screen.getByRole("button", { name: "Daten aktualisieren" }),
      );
    });

    const button = screen.getByRole("button", { name: "Daten aktualisieren" });
    const svg = button.querySelector("svg");
    expect(svg).toHaveClass("animate-spin");
  });

  it("disables button during refresh", async () => {
    render(<RefreshButton />);

    await act(async () => {
      fireEvent.click(
        screen.getByRole("button", { name: "Daten aktualisieren" }),
      );
    });

    expect(
      screen.getByRole("button", { name: "Daten aktualisieren" }),
    ).toBeDisabled();
  });

  it("re-enables button after timeout", async () => {
    render(<RefreshButton />);

    await act(async () => {
      fireEvent.click(
        screen.getByRole("button", { name: "Daten aktualisieren" }),
      );
    });

    expect(
      screen.getByRole("button", { name: "Daten aktualisieren" }),
    ).toBeDisabled();

    await act(async () => {
      vi.advanceTimersByTime(600);
    });

    expect(
      screen.getByRole("button", { name: "Daten aktualisieren" }),
    ).toBeEnabled();
  });

  it("prevents double-click during refresh", async () => {
    render(<RefreshButton />);

    const button = screen.getByRole("button", {
      name: "Daten aktualisieren",
    });

    await act(async () => {
      fireEvent.click(button);
    });

    await act(async () => {
      fireEvent.click(button);
    });

    expect(mockMutate).toHaveBeenCalledTimes(1);
  });
});
