/**
 * Tests for app/(auth)/loading.tsx
 *
 * Tests the loading component displayed during route transitions.
 */

import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import LoadingPage from "../loading";

describe("LoadingPage", () => {
  it("renders the loading component with message", () => {
    render(<LoadingPage />);

    // The Loading component should display the "Laden..." message
    expect(screen.getByText("Laden...")).toBeInTheDocument();
  });

  it("renders without crashing", () => {
    const { container } = render(<LoadingPage />);
    expect(container).toBeTruthy();
  });
});
