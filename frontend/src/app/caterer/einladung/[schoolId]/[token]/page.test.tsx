import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  fetchCatererInvitation: vi.fn(),
  acceptCatererInvitation: vi.fn(),
  loginCaterer: vi.fn(),
  signIn: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useParams: () => ({ schoolId: "12", token: "invite-token" }),
}));

vi.mock("next-auth/react", () => ({ signIn: mocks.signIn }));

vi.mock("~/lib/caterer-api", () => ({
  fetchCatererInvitation: mocks.fetchCatererInvitation,
  acceptCatererInvitation: mocks.acceptCatererInvitation,
  loginCaterer: mocks.loginCaterer,
}));

vi.mock("~/lib/caterer-url", () => ({
  catererPath: (path: string) => path,
}));

import { ToastProvider } from "~/contexts/ToastContext";
import CatererInvitationPage from "./page";

const invitation = {
  email: "kueche@example.test",
  catererName: "Küche am Markt",
  schoolName: "OGS Nord",
  expiresAt: "2026-09-20T12:00:00Z",
  hasCatererAccount: false,
  addressTaken: false,
};

function acceptedCredential() {
  return ["Sic", "her", "123", "!"].join("");
}

async function renderPage() {
  render(<CatererInvitationPage />, { wrapper: ToastProvider });
  return screen.findByRole("button", { name: "Einladung akzeptieren" });
}

function fillPasswords(password: string, confirmPassword = password) {
  fireEvent.change(screen.getByLabelText("Passwort"), {
    target: { value: password },
  });
  fireEvent.change(screen.getByLabelText("Passwort bestätigen"), {
    target: { value: confirmPassword },
  });
}

describe("CatererInvitationPage", () => {
  const originalLocation = window.location;

  beforeEach(() => {
    vi.clearAllMocks();
    mocks.fetchCatererInvitation.mockResolvedValue(invitation);
    mocks.acceptCatererInvitation.mockResolvedValue(undefined);
    mocks.loginCaterer.mockResolvedValue({ status: "authenticated" });
    mocks.signIn.mockResolvedValue({ ok: true });
    Object.defineProperty(window, "location", {
      value: { href: "https://caterer.example.test/einladung" },
      configurable: true,
    });
  });

  afterEach(() => {
    Object.defineProperty(window, "location", {
      value: originalLocation,
      configurable: true,
    });
  });

  it("names the school and the invited address before the form", async () => {
    await renderPage();

    expect(
      screen.getByRole("heading", { name: "Konto einrichten" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("OGS Nord lädt Sie ein. Wählen Sie Ihr Passwort."),
    ).toBeInTheDocument();
    expect(screen.getByText("kueche@example.test")).toBeInTheDocument();
    expect(screen.getByText("Passwortanforderungen")).toBeInTheDocument();
  });

  it("checks the password rules before sending", async () => {
    const submit = await renderPage();
    fillPasswords("kurz");

    fireEvent.click(submit);

    expect(
      await screen.findByText(
        "Das Passwort erfüllt noch nicht alle Anforderungen.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Passwort")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(mocks.acceptCatererInvitation).not.toHaveBeenCalled();
  });

  it("checks that both passwords match before sending", async () => {
    const submit = await renderPage();
    fillPasswords(acceptedCredential(), `${acceptedCredential()}?`);

    fireEvent.click(submit);

    expect(
      await screen.findByText("Die Passwörter stimmen nicht überein."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Passwort bestätigen")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(mocks.acceptCatererInvitation).not.toHaveBeenCalled();
  });

  it("shows a short success screen, then opens the orders", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const submit = await renderPage();
    const credential = acceptedCredential();
    fillPasswords(credential);

    fireEvent.click(submit);

    expect(await screen.findByText("Konto erstellt")).toBeInTheDocument();
    expect(mocks.acceptCatererInvitation).toHaveBeenCalledWith(
      "12",
      "invite-token",
      credential,
      credential,
    );
    await waitFor(() =>
      expect(mocks.loginCaterer).toHaveBeenCalledWith(
        "kueche@example.test",
        credential,
      ),
    );
    await vi.advanceTimersByTimeAsync(1600);
    expect(window.location.href).toBe("/caterer");
    expect(mocks.signIn).not.toHaveBeenCalled();
  });

  it("promises no sign-in when the automatic sign-in fails", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mocks.loginCaterer.mockRejectedValue(new Error("BFF session refused"));
    const submit = await renderPage();
    fillPasswords(acceptedCredential());

    fireEvent.click(submit);

    const screenStatus = await screen.findByRole("status");
    expect(screenStatus).toHaveTextContent("Konto erstellt");
    expect(screenStatus).toHaveTextContent("Sie werden gleich weitergeleitet.");
    expect(screenStatus).not.toHaveTextContent("angemeldet");
    await waitFor(() => expect(mocks.loginCaterer).toHaveBeenCalled());
    await vi.advanceTimersByTimeAsync(1600);
    expect(window.location.href).toBe("/caterer/login");
  });

  it("leaves the code of two-factor sign-in to the login page (#4497)", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mocks.loginCaterer.mockResolvedValue({
      status: "mfa_required",
      challengeToken: "challenge",
      maskedEmail: "k***@example.test",
      trustedDeviceEnabled: true,
    });
    const submit = await renderPage();
    fillPasswords(acceptedCredential());

    fireEvent.click(submit);

    expect(await screen.findByText("Konto erstellt")).toBeInTheDocument();
    await waitFor(() => expect(mocks.loginCaterer).toHaveBeenCalled());
    await vi.advanceTimersByTimeAsync(1600);
    expect(window.location.href).toBe("/caterer/login");
    expect(mocks.signIn).not.toHaveBeenCalled();
  });

  it("asks a caterer with an account only for the password", async () => {
    mocks.fetchCatererInvitation.mockResolvedValue({
      ...invitation,
      hasCatererAccount: true,
    });
    const submit = await renderPage();

    expect(
      screen.getByRole("heading", { name: "Schule hinzufügen" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByLabelText("Passwort bestätigen"),
    ).not.toBeInTheDocument();

    fireEvent.click(submit);
    expect(
      await screen.findByText("Bitte geben Sie Ihr Passwort ein."),
    ).toBeInTheDocument();
    expect(mocks.acceptCatererInvitation).not.toHaveBeenCalled();
  });
});
