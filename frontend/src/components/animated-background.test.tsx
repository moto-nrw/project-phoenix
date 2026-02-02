/**
 * Tests for AnimatedBackground Component
 * Tests the rendering of animated canvas background
 */
import { render } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { AnimatedBackground } from "./animated-background";

// Mock canvas context
const mockClearRect = vi.fn();
const mockBeginPath = vi.fn();
const mockArc = vi.fn();
const mockFill = vi.fn();
const mockCreateRadialGradient = vi.fn(() => ({
  addColorStop: vi.fn(),
}));

const mockCtx = {
  clearRect: mockClearRect,
  beginPath: mockBeginPath,
  arc: mockArc,
  fill: mockFill,
  createRadialGradient: mockCreateRadialGradient,
  fillStyle: "",
  globalAlpha: 0,
  filter: "",
};

describe("AnimatedBackground", () => {
  let rafId: number;

  beforeEach(() => {
    vi.clearAllMocks();
    rafId = 1;

    // Mock HTMLCanvasElement.prototype.getContext
    HTMLCanvasElement.prototype.getContext = vi.fn().mockReturnValue(mockCtx);

    // Mock requestAnimationFrame - call callback once to test animation, then stop
    let called = false;
    vi.spyOn(window, "requestAnimationFrame").mockImplementation((cb) => {
      const id = rafId++;
      if (!called) {
        called = true;
        cb(0);
      }
      return id;
    });

    // Mock cancelAnimationFrame
    vi.spyOn(window, "cancelAnimationFrame").mockImplementation(() => {
      // no-op for tests
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders canvas element", () => {
    const { container } = render(<AnimatedBackground />);
    const canvas = container.querySelector("canvas");
    expect(canvas).toBeInTheDocument();
  });

  it("applies correct CSS classes", () => {
    const { container } = render(<AnimatedBackground />);
    const canvas = container.querySelector("canvas");
    expect(canvas).toHaveClass("fixed", "inset-0", "h-full", "w-full");
  });

  it("sets correct z-index style", () => {
    const { container } = render(<AnimatedBackground />);
    const canvas = container.querySelector("canvas");
    expect(canvas).toHaveStyle({ zIndex: "-10" });
  });

  it("initializes canvas context", () => {
    render(<AnimatedBackground />);
    // eslint-disable-next-line @typescript-eslint/unbound-method
    expect(HTMLCanvasElement.prototype.getContext).toHaveBeenCalledWith("2d");
  });

  it("sets canvas dimensions to window size", () => {
    const { container } = render(<AnimatedBackground />);
    const canvas = container.querySelector("canvas")!;
    expect(canvas.width).toBe(window.innerWidth);
    expect(canvas.height).toBe(window.innerHeight);
  });

  it("starts animation on mount", () => {
    render(<AnimatedBackground />);
    expect(window.requestAnimationFrame).toHaveBeenCalled();
  });

  it("cancels animation on unmount", () => {
    const { unmount } = render(<AnimatedBackground />);
    unmount();
    expect(window.cancelAnimationFrame).toHaveBeenCalled();
  });

  it("handles resize events", () => {
    const { container } = render(<AnimatedBackground />);

    // Trigger resize
    window.dispatchEvent(new Event("resize"));

    const canvas = container.querySelector("canvas")!;
    expect(canvas.width).toBe(window.innerWidth);
    expect(canvas.height).toBe(window.innerHeight);
  });

  it("cleans up resize listener on unmount", () => {
    const removeEventListenerSpy = vi.spyOn(window, "removeEventListener");
    const { unmount } = render(<AnimatedBackground />);
    unmount();
    expect(removeEventListenerSpy).toHaveBeenCalledWith(
      "resize",
      expect.any(Function),
    );
  });

  it("handles missing canvas context gracefully", () => {
    HTMLCanvasElement.prototype.getContext = vi.fn().mockReturnValue(null);
    expect(() => {
      render(<AnimatedBackground />);
    }).not.toThrow();
  });

  it("performs animation drawing operations", () => {
    render(<AnimatedBackground />);

    // The animate function was called, which should call drawing methods
    expect(mockClearRect).toHaveBeenCalled();
    expect(mockCreateRadialGradient).toHaveBeenCalled();
    expect(mockBeginPath).toHaveBeenCalled();
    expect(mockArc).toHaveBeenCalled();
    expect(mockFill).toHaveBeenCalled();
  });
});
