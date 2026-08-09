/**
 * Tests for RefreshButton Component
 * Tests data refresh trigger, spin animation, success checkmark, and state transitions
 */
import { render, screen, fireEvent, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { RefreshButton } from "./refresh-button";

// Mock SWR config
const mockMutate = vi.fn().mockResolvedValue(undefined);
vi.mock("swr", () => ({
  useSWRConfig: () => ({ mutate: mockMutate }),
}));

/** Helper: find the RotateCw icon (no text-moto-green class) */
function findRotateIcon(container: HTMLElement) {
  return Array.from(container.querySelectorAll("svg")).find(
    (svg) => !svg.classList.contains("text-moto-green"),
  );
}

/** Helper: find the Check icon (has text-moto-green class) */
function findCheckIcon(container: HTMLElement) {
  return Array.from(container.querySelectorAll("svg")).find((svg) =>
    svg.classList.contains("text-moto-green"),
  );
}

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

    expect(findRotateIcon(container)).toBeDefined();
  });

  it("does not show spin animation initially", () => {
    const { container } = render(<RefreshButton />);

    expect(findRotateIcon(container)).not.toHaveClass("animate-spin");
  });

  it("hides check icon initially", () => {
    const { container } = render(<RefreshButton />);

    expect(findCheckIcon(container)).toHaveClass("opacity-0");
  });

  it("calls SWR mutate on click", async () => {
    render(<RefreshButton />);

    await act(async () => {
      fireEvent.click(
        screen.getByRole("button", { name: "Daten aktualisieren" }),
      );
    });

    expect(mockMutate).toHaveBeenCalledWith(expect.any(Function));
  });

  it("shows spin animation during refresh", async () => {
    const { container } = render(<RefreshButton />);

    await act(async () => {
      fireEvent.click(
        screen.getByRole("button", { name: "Daten aktualisieren" }),
      );
    });

    expect(findRotateIcon(container)).toHaveClass("animate-spin");
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

  it("shows green checkmark after spin completes", async () => {
    const { container } = render(<RefreshButton />);

    await act(async () => {
      fireEvent.click(
        screen.getByRole("button", { name: "Daten aktualisieren" }),
      );
    });

    // Complete the rotation
    const rotateIcon = findRotateIcon(container)!;
    await act(async () => {
      fireEvent.animationIteration(rotateIcon);
    });

    // Check icon should be visible
    expect(findCheckIcon(container)).toHaveClass("opacity-100");
    // Rotate icon should be hidden
    expect(findRotateIcon(container)).toHaveClass("opacity-0");
  });

  it("returns to idle state after success timeout", async () => {
    const { container } = render(<RefreshButton />);

    await act(async () => {
      fireEvent.click(
        screen.getByRole("button", { name: "Daten aktualisieren" }),
      );
    });

    const rotateIcon = findRotateIcon(container)!;
    await act(async () => {
      fireEvent.animationIteration(rotateIcon);
    });

    // Wait for success → idle transition
    await act(async () => {
      vi.advanceTimersByTime(800);
    });

    // Button should be enabled again
    expect(
      screen.getByRole("button", { name: "Daten aktualisieren" }),
    ).toBeEnabled();
    // Check icon should be hidden again
    expect(findCheckIcon(container)).toHaveClass("opacity-0");
    // Rotate icon should be visible (no opacity-0)
    expect(findRotateIcon(container)).not.toHaveClass("opacity-0");
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
