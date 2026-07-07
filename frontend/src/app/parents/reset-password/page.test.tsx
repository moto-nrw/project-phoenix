import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  token: "valid-token" as string | null,
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => ({
    get: (key: string) => (key === "token" ? mocks.token : null),
  }),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mocks.push }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSafe: () => null,
}));

vi.mock("~/lib/tenant-api", () => ({
  loginImageSrc: (path: string) => `/resolved${path}`,
}));

// parentPath is environment-driven; pin it so the success/back links are
// deterministic without standing up the full parent-url env contract.
vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => `https://parents.example.com${path}`,
}));

const confirmParentPasswordReset = vi.fn();
vi.mock("~/lib/auth-api", () => ({
  confirmParentPasswordReset: (...args: unknown[]) =>
    confirmParentPasswordReset(...args),
}));

import ParentResetPasswordPage from "./page";

describe("ParentResetPasswordPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.token = "valid-token";
  });

  it("renders the reset form wired with localized parent copy", () => {
    render(<ParentResetPasswordPage />);

    // The German default catalog (mocked globally) drives next-intl, so the
    // localized parent form chrome is what renders.
    expect(screen.getByText("Neues Passwort festlegen")).toBeInTheDocument();
    expect(screen.getByText("Passwort-Anforderungen:")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Passwort ändern" }),
    ).toBeEnabled();
  });

  it("shows the missing-token error when the URL has no token", () => {
    mocks.token = null;
    render(<ParentResetPasswordPage />);

    expect(
      screen.getByRole("button", { name: "Passwort ändern" }),
    ).toBeDisabled();
  });

  it("links back to the parents login via parentPath", () => {
    render(<ParentResetPasswordPage />);

    const backLink = screen.getByRole("link", { name: /Anmeldung/i });
    expect(backLink).toHaveAttribute(
      "href",
      "https://parents.example.com/parents/login",
    );
  });
});
