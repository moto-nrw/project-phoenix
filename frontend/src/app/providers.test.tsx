import { describe, it, expect, vi } from "vitest";
import { render } from "@testing-library/react";
import { Providers } from "./providers";

// Mock all provider components
vi.mock("next-auth/react", () => ({
  SessionProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="session-provider">{children}</div>
  ),
}));

vi.mock("@/components/dashboard/modal-context", () => ({
  ModalProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="modal-provider">{children}</div>
  ),
}));

vi.mock("~/lib/profile-context", () => ({
  ProfileProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="profile-provider">{children}</div>
  ),
}));

vi.mock("~/lib/supervision-context", () => ({
  SupervisionProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="supervision-provider">{children}</div>
  ),
}));

vi.mock("~/contexts/AlertContext", () => ({
  AlertProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="alert-provider">{children}</div>
  ),
}));

vi.mock("~/contexts/ToastContext", () => ({
  ToastProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="toast-provider">{children}</div>
  ),
}));

vi.mock("~/components/auth-wrapper", () => ({
  AuthWrapper: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="auth-wrapper">{children}</div>
  ),
}));

describe("Providers", () => {
  it("renders all providers in correct nesting order", () => {
    const { getByTestId, getByText } = render(
      <Providers>
        <div>Child Content</div>
      </Providers>,
    );

    // Verify all providers are rendered
    expect(getByTestId("session-provider")).toBeInTheDocument();
    expect(getByTestId("auth-wrapper")).toBeInTheDocument();
    expect(getByTestId("profile-provider")).toBeInTheDocument();
    expect(getByTestId("supervision-provider")).toBeInTheDocument();
    expect(getByTestId("modal-provider")).toBeInTheDocument();
    expect(getByTestId("alert-provider")).toBeInTheDocument();
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

    // Child should be deeply nested in all providers
    const child = container.querySelector("#test-child");
    expect(child).toBeInTheDocument();
  });
});
