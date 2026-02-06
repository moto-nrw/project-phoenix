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

    const fallback = container.querySelector(".border-red-200");
    expect(fallback).toBeInTheDocument();
    expect(fallback).toHaveClass("bg-red-50");
    expect(fallback).toHaveClass("text-red-700");
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
});
