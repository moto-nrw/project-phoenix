import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import TraegerResetPasswordPage from "./page";

let mockToken: string | null = null;

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => ({
    get: (key: string) => (key === "token" ? mockToken : null),
  }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSafe: () => null,
  useTenantSlugSafe: () => null,
  useTenantRoutingModeSafe: vi.fn(() => "path"),
}));

vi.mock("~/lib/auth-api", () => ({
  confirmTraegerPasswordReset: vi.fn(),
}));

vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path,
}));

describe("TraegerResetPasswordPage", () => {
  beforeEach(() => {
    mockToken = null;
  });

  it("offers a new link instead of a form that cannot be sent", () => {
    render(<TraegerResetPasswordPage />);

    expect(
      screen.getByText(
        "Der Link ist unvollständig. Bitte fordern Sie einen neuen Link an.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Neuen Link anfordern" }),
    ).toHaveAttribute("href", "/traeger/login?passwort-vergessen=1");
    expect(screen.queryByLabelText("Neues Passwort")).not.toBeInTheDocument();
  });

  it("shows the form when the link carries its token", () => {
    mockToken = "reset-token";
    render(<TraegerResetPasswordPage />);

    expect(screen.getByLabelText("Neues Passwort")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Neuen Link anfordern" }),
    ).not.toBeInTheDocument();
  });
});
