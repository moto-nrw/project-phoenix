/**
 * Tests for SSEErrorBoundary Component
 * Tests error boundary for SSE components
 */
import React from "react";
import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { SSEErrorBoundary } from "./SSEErrorBoundary";

// Component that throws an error
function ThrowError(): React.ReactNode {
  throw new Error("Test error");
}

// Component that renders normally
function NormalComponent() {
  return <div>Normal content</div>;
}

describe("SSEErrorBoundary", () => {
  // Suppress console.error for these tests
  const originalError = console.error;
  beforeEach(() => {
    console.error = vi.fn() as typeof console.error;
  });

  afterEach(() => {
    console.error = originalError;
  });

  it("renders children when no error", () => {
    render(
      <SSEErrorBoundary>
        <NormalComponent />
      </SSEErrorBoundary>,
    );

    expect(screen.getByText("Normal content")).toBeInTheDocument();
  });

  it("renders default fallback when error occurs", () => {
    render(
      <SSEErrorBoundary>
        <ThrowError />
      </SSEErrorBoundary>,
    );

    expect(
      screen.getByText("Live-Updates sind derzeit nicht verfügbar."),
    ).toBeInTheDocument();
  });

  it("renders custom fallback when provided", () => {
    const customFallback = <div>Custom error message</div>;

    render(
      <SSEErrorBoundary fallback={customFallback}>
        <ThrowError />
      </SSEErrorBoundary>,
    );

    expect(screen.getByText("Custom error message")).toBeInTheDocument();
  });

  it("applies correct styling to default fallback", () => {
    const { container } = render(
      <SSEErrorBoundary>
        <ThrowError />
      </SSEErrorBoundary>,
    );

    // Default fallback is the kit's load-error Alert (#6071).
    const fallback = screen.getByRole("alert");
    expect(fallback).toHaveTextContent(
      "Live-Updates sind derzeit nicht verfügbar.",
    );
    expect(fallback).toHaveClass("border-moto-red/20");
    expect(fallback).toHaveClass("bg-moto-red-soft");
    expect(fallback).toHaveClass("text-moto-red-strong");
    expect(container).toContainElement(fallback);
  });

  it("logs error to console", () => {
    const consoleErrorSpy = vi.spyOn(console, "error");

    render(
      <SSEErrorBoundary>
        <ThrowError />
      </SSEErrorBoundary>,
    );

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "SSE boundary caught an error",
      expect.objectContaining({
        error: "Test error",
      }),
    );
  });

  it("logs the shown notice for Loki without the crash message (#2512)", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});

    render(
      <SSEErrorBoundary>
        <ThrowError />
      </SSEErrorBoundary>,
    );

    expect(warn).toHaveBeenCalledWith("error_displayed", {
      surface: "page",
      error_class: "crash",
      error_name: "Error",
    });
    expect(JSON.stringify(warn.mock.calls)).not.toContain("Test error");
    warn.mockRestore();
  });
});
