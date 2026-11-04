import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { _resetTraegerHostCache } from "~/lib/traeger-url";
import { catalogText } from "~/test/error-catalog-text";

const mocks = vi.hoisted(() => ({
  fetchTraegerInvitation: vi.fn(),
  acceptTraegerInvitation: vi.fn(),
  traegerLogin: vi.fn(),
  signIn: vi.fn(),
}));

vi.mock("~/lib/traeger-api", () => ({
  fetchTraegerInvitation: mocks.fetchTraegerInvitation,
  acceptTraegerInvitation: mocks.acceptTraegerInvitation,
  traegerLogin: mocks.traegerLogin,
}));
vi.mock("next-auth/react", () => ({ signIn: mocks.signIn }));
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams("token=abc"),
}));

import TraegerInvitePage from "./page";

const PREVIEW = {
  organizationName: "Träger Bissendorf",
  email: "neu@traeger.example",
  roles: ["billing"],
  requiresExistingPassword: false,
  expiresAt: "2026-09-11T10:00:00Z",
};

function renderPage() {
  return render(
    <ToastProvider>
      <TraegerInvitePage />
    </ToastProvider>,
  );
}

describe("TraegerInvitePage", () => {
  let assign: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    for (const fn of Object.values(mocks)) fn.mockReset();
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", window.location.host);
    _resetTraegerHostCache();
    assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllEnvs();
    _resetTraegerHostCache();
  });

  it("creates the account with a new password and signs the person in", async () => {
    mocks.fetchTraegerInvitation.mockResolvedValue(PREVIEW);
    mocks.acceptTraegerInvitation.mockResolvedValue("neu@traeger.example");
    mocks.traegerLogin.mockResolvedValue({
      status: "authenticated",
    });
    mocks.signIn.mockResolvedValue({ error: undefined });
    renderPage();

    expect(await screen.findByText("Träger Bissendorf")).toBeInTheDocument();
    expect(screen.getByText("Abrechnung")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Neues Passwort"), {
      target: { value: "Sicher1234%" },
    });
    fireEvent.change(screen.getByLabelText("Passwort wiederholen"), {
      target: { value: "Anders1234%" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Einladung annehmen" }));
    expect(
      await screen.findByText("Die Passwörter stimmen nicht überein."),
    ).toBeInTheDocument();
    expect(mocks.acceptTraegerInvitation).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Passwort wiederholen"), {
      target: { value: "Sicher1234%" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Einladung annehmen" }));

    await waitFor(() => expect(assign).toHaveBeenCalledWith("/"));
    expect(mocks.acceptTraegerInvitation).toHaveBeenCalledWith("abc", {
      password: "Sicher1234%",
      confirmPassword: "Sicher1234%",
    });
    expect(mocks.signIn).not.toHaveBeenCalled();
    expect(mocks.traegerLogin).toHaveBeenCalledWith({
      email: "neu@traeger.example",
      password: "Sicher1234%",
    });
  });

  it("asks an existing account for its password only", async () => {
    mocks.fetchTraegerInvitation.mockResolvedValue({
      ...PREVIEW,
      requiresExistingPassword: true,
    });
    mocks.acceptTraegerInvitation.mockRejectedValue(
      new ApiError("Unauthorized", 401, {
        code: "identity.invalid_credentials",
      }),
    );
    renderPage();

    fireEvent.change(
      await screen.findByLabelText("Passwort Ihres moto-Kontos"),
      {
        target: { value: "Falsch1234%" },
      },
    );
    // Anker: das Feld für das bestehende Konto ist da, eine Wiederholung nicht.
    expect(screen.queryAllByLabelText("Passwort wiederholen")).toEqual([]);
    fireEvent.click(screen.getByRole("button", { name: "Einladung annehmen" }));

    expect(
      await screen.findByText(
        catalogText("identity.invalid_credentials", "die Einladung"),
      ),
    ).toBeInTheDocument();
    expect(mocks.acceptTraegerInvitation).toHaveBeenCalledWith("abc", {
      password: "Falsch1234%",
      confirmPassword: undefined,
    });
  });

  it("explains an expired link and who sends a new one", async () => {
    mocks.fetchTraegerInvitation.mockRejectedValue(
      new ApiError("Gone", 410, { code: "identity.invitation_expired" }),
    );
    renderPage();

    expect(
      await screen.findByText(
        catalogText("identity.invitation_expired", "die Einladung"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Bitten Sie die Person, die Sie eingeladen hat, um eine neue Einladung.",
      ),
    ).toBeInTheDocument();
  });

  // Quorum #4937: the link is spent once accepted; a failed sign-in must not
  // offer to accept it again.
  it("sends the person to the login when the sign-in after acceptance fails", async () => {
    mocks.fetchTraegerInvitation.mockResolvedValue(PREVIEW);
    mocks.acceptTraegerInvitation.mockResolvedValue("neu@traeger.example");
    mocks.traegerLogin.mockRejectedValue(new Error("network"));
    renderPage();

    fireEvent.change(await screen.findByLabelText("Neues Passwort"), {
      target: { value: "Sicher1234%" },
    });
    fireEvent.change(screen.getByLabelText("Passwort wiederholen"), {
      target: { value: "Sicher1234%" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Einladung annehmen" }));

    await waitFor(() => expect(assign).toHaveBeenCalledWith("/login"));
    expect(mocks.acceptTraegerInvitation).toHaveBeenCalledTimes(1);
  });
});
