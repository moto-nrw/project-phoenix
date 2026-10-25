import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const { mockLogin, mockSignIn, mockSearchParamsGet } = vi.hoisted(() => ({
  mockLogin: vi.fn(),
  mockSignIn: vi.fn(),
  mockSearchParamsGet: vi.fn((_key: string): string | null => null),
}));

vi.mock("next-auth/react", () => ({
  signIn: (...args: unknown[]) => mockSignIn(...args),
  signOut: vi.fn(),
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));

const stableSearchParams = { get: mockSearchParamsGet };
vi.mock("next/navigation", () => ({
  useSearchParams: () => stableSearchParams,
}));

vi.mock("~/lib/traeger-api", () => ({
  traegerLogin: (...args: unknown[]) => mockLogin(...args),
}));

vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path,
}));

import TraegerLoginPage from "./page";

function submitCredentials() {
  fireEvent.change(screen.getByTestId("input-email"), {
    target: { value: "buero@traeger.example" },
  });
  fireEvent.change(screen.getByTestId("input-password"), {
    target: { value: "secret" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Anmelden" }));
}

describe("TraegerLoginPage", () => {
  beforeEach(() => {
    mockLogin.mockReset();
    mockSignIn.mockReset();
    mockSearchParamsGet.mockReturnValue(null);
  });

  it("seeds the Träger session with the minted pair", async () => {
    mockLogin.mockResolvedValue({
      status: "authenticated",
      accessToken: "access",
      refreshToken: "refresh",
    });
    mockSignIn.mockResolvedValue({ error: undefined });
    render(<TraegerLoginPage />);

    expect(
      screen.getByRole("heading", { name: "Willkommen im Träger-Portal" }),
    ).toBeInTheDocument();
    submitCredentials();

    await waitFor(() =>
      expect(mockSignIn).toHaveBeenCalledWith("traeger-credentials", {
        redirect: false,
        internalRefresh: "true",
        token: "access",
        refreshToken: "refresh",
      }),
    );
    expect(mockLogin).toHaveBeenCalledWith({
      email: "buero@traeger.example",
      password: "secret",
      organizationId: undefined,
    });
  });

  it("asks which Träger to open and resubmits with the choice", async () => {
    mockLogin
      .mockResolvedValueOnce({
        status: "organization_selection_required",
        organizations: [
          { id: "7", name: "Träger Nord" },
          { id: "9", name: "Träger Süd" },
        ],
      })
      .mockResolvedValueOnce({
        status: "authenticated",
        accessToken: "access",
        refreshToken: "refresh",
      });
    mockSignIn.mockResolvedValue({ error: undefined });
    render(<TraegerLoginPage />);

    submitCredentials();

    const south = await screen.findByLabelText("Träger Süd");
    expect(screen.getByLabelText("Träger Nord")).toBeChecked();
    fireEvent.click(south);
    fireEvent.click(screen.getByRole("button", { name: "Träger öffnen" }));

    await waitFor(() =>
      expect(mockLogin).toHaveBeenLastCalledWith({
        email: "buero@traeger.example",
        password: "secret",
        organizationId: "9",
      }),
    );
    await waitFor(() => expect(mockSignIn).toHaveBeenCalledTimes(1));
  });

  it("explains a missing Träger-Portal access with the catalog text", async () => {
    mockLogin.mockRejectedValue(
      new ApiError("account has no Träger membership", 403, {
        code: "traeger.no_membership",
      }),
    );
    render(<TraegerLoginPage />);

    submitCredentials();

    expect(
      await screen.findByText(
        catalogText("traeger.no_membership", "die Anmeldung"),
      ),
    ).toBeInTheDocument();
    expect(mockSignIn).not.toHaveBeenCalled();
  });

  it("keeps a refused password in the form instead of redirecting", async () => {
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    mockLogin.mockRejectedValue(
      new ApiError("invalid username or password", 401, {
        code: "identity.invalid_credentials",
      }),
    );
    render(<TraegerLoginPage />);

    submitCredentials();

    expect(
      await screen.findByText(
        catalogText("identity.invalid_credentials", "die Anmeldung"),
      ),
    ).toBeInTheDocument();
    expect(assign).not.toHaveBeenCalled();
    expect(mockSignIn).not.toHaveBeenCalled();
    assign.mockRestore();
  });
});
