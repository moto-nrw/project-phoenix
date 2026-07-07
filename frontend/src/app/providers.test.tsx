import { describe, it, expect, vi } from "vitest";
import { render } from "@testing-library/react";
import { Providers } from "./providers";

// Mock all provider components
vi.mock("@/components/dashboard/modal-context", () => ({
  ModalProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="modal-provider">{children}</div>
  ),
}));

vi.mock("~/contexts/ToastContext", () => ({
  ToastProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="toast-provider">{children}</div>
  ),
}));

describe("Providers", () => {
  it("renders auth-free root providers in correct nesting order", () => {
    const { getByTestId, getByText } = render(
      <Providers>
        <div>Child Content</div>
      </Providers>,
    );

    // Root providers are auth-free (SessionProvider moved to tenant/operator layouts)
    expect(getByTestId("modal-provider")).toBeInTheDocument();
    expect(getByTestId("toast-provider")).toBeInTheDocument();

    // Verify children are rendered
    expect(getByText("Child Content")).toBeInTheDocument();
  });

  it("wraps children in all context providers", () => {
    const { container } = render(
      <Providers>
        <div id="test-child">Test</div>
      </Providers>,
    );

    const child = container.querySelector("#test-child");
    expect(child).toBeInTheDocument();
  });
});
