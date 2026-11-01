import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { catalogText } from "~/test/error-catalog-text";

const logout = vi.fn(() => Promise.resolve());

const mfa = vi.hoisted(() => ({
  status: vi.fn(),
  start: vi.fn(),
  confirm: vi.fn(),
  disable: vi.fn(),
  signIn: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  signIn: (...args: unknown[]) => mfa.signIn(...args),
  useSession: () => ({
    data: { user: { email: "kueche@example.com" } },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({ logout }),
}));

// The device card has tests of its own.
vi.mock("~/components/settings/trusted-devices-section", () => ({
  TrustedDevicesSection: () => <p>Gemerkte Geräte (Karte)</p>,
}));

vi.mock("~/lib/caterer-mfa-api", () => ({
  fetchCatererMFAStatus: () => mfa.status(),
  startCatererMFA: () => mfa.start(),
  confirmCatererMFA: (...args: unknown[]) => mfa.confirm(...args),
  disableCatererMFA: (...args: unknown[]) => mfa.disable(...args),
}));

import CatererAccountPage from "./page";

const originalFetch = global.fetch;

function reply(status: number, body?: unknown): typeof global.fetch {
  return vi.fn().mockImplementation(() =>
    Promise.resolve(
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
    ),
  );
}

function openAndSubmit() {
  fireEvent.click(screen.getByRole("button", { name: "Passwort ändern" }));
  fireEvent.change(screen.getByLabelText("Aktuelles Passwort"), {
    target: { value: "Old-passw0rd!" },
  });
  fireEvent.change(screen.getByLabelText("Neues Passwort"), {
    target: { value: "New-passw0rd!" },
  });
  fireEvent.change(screen.getByLabelText("Neues Passwort bestätigen"), {
    target: { value: "New-passw0rd!" },
  });
  const submit = screen
    .getAllByRole("button", { name: "Passwort ändern" })
    .find((button) => button.getAttribute("type") === "submit");
  if (!submit) throw new Error("submit button missing");
  fireEvent.click(submit);
}

describe("Caterer-Konto (#4496)", () => {
  beforeEach(() => {
    logout.mockClear();
    mfa.status.mockResolvedValue({ enabled: false, maskedEmail: "" });
    vi.spyOn(console, "error").mockImplementation(() => undefined);
  });

  afterEach(() => {
    global.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it("shows the sign-in address and the password change", () => {
    render(<CatererAccountPage />, { wrapper: ToastProvider });

    expect(screen.getByText("kueche@example.com")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Nach dem Ändern melden Sie sich auf allen Geräten neu an.",
      ),
    ).toBeInTheDocument();
  });

  it("sends the change to the caterer route and signs out afterwards", async () => {
    global.fetch = reply(204);
    render(<CatererAccountPage />, { wrapper: ToastProvider });

    openAndSubmit();

    await waitFor(() =>
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/caterer/account/password",
        expect.objectContaining({ method: "POST" }),
      ),
    );
    expect(
      await screen.findByText("Passwort erfolgreich geändert!"),
    ).toBeInTheDocument();
    await waitFor(() => expect(logout).toHaveBeenCalledTimes(1), {
      timeout: 3000,
    });
  });

  it("keeps a wrong current password in the dialog", async () => {
    global.fetch = reply(401, {
      status: 401,
      detail: "current password is incorrect",
      code: "identity.current_password_wrong",
      errors: [{ field: "current_password", reason: "invalid credentials" }],
    });
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    render(<CatererAccountPage />, { wrapper: ToastProvider });

    openAndSubmit();

    expect(
      await screen.findByText(
        catalogText(
          "identity.current_password_wrong",
          "das Ändern des Passworts",
        ),
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Aktuelles Passwort")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(assign).not.toHaveBeenCalled();
    expect(logout).not.toHaveBeenCalled();
  });
});

describe("Caterer-Konto: Zwei-Faktor-Anmeldung (#4497)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(console, "error").mockImplementation(() => undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("switches it on with the mailed code and keeps the renewed session", async () => {
    mfa.status.mockResolvedValue({ enabled: false, maskedEmail: "" });
    mfa.start.mockResolvedValue({
      challengeToken: "challenge",
      maskedEmail: "k***@example.com",
    });
    mfa.confirm.mockResolvedValue({ sessionRenewed: true });
    render(<CatererAccountPage />, { wrapper: ToastProvider });

    expect(await screen.findByText("Ausgeschaltet")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Einschalten" }));
    expect(
      await screen.findByText(/Wir haben einen Code an k\*\*\*@example.com/),
    ).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Code"), {
      target: { value: "123456" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Einschalten" }));

    expect(await screen.findByText("Eingeschaltet")).toBeInTheDocument();
    expect(
      (
        await screen.findAllByText(
          "Die Zwei-Faktor-Anmeldung ist eingeschaltet.",
        )
      ).length,
    ).toBeGreaterThan(0);
    expect(mfa.confirm).toHaveBeenCalledWith("challenge", "123456");
    // The BFF installs the new session; the browser holds no token (#5005).
    expect(mfa.signIn).not.toHaveBeenCalled();
  });

  it("offers a fresh sign-in when the BFF could not install the new session", async () => {
    mfa.status.mockResolvedValue({ enabled: false, maskedEmail: "" });
    mfa.start.mockResolvedValue({
      challengeToken: "challenge",
      maskedEmail: "k***@example.com",
    });
    mfa.confirm.mockResolvedValue({ sessionRenewed: false });
    render(<CatererAccountPage />, { wrapper: ToastProvider });

    fireEvent.click(await screen.findByRole("button", { name: "Einschalten" }));
    fireEvent.change(await screen.findByLabelText("Code"), {
      target: { value: "123456" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Einschalten" }));

    // Anker: Der fehlgeschlagene Sitzungswechsel zeigt die neue Anmeldung an.
    expect(
      await screen.findByText(
        "Die Zwei-Faktor-Anmeldung ist eingeschaltet. Die Anmeldung konnte nicht erneuert werden. Bitte melden Sie sich neu an.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Code")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Die Zwei-Faktor-Anmeldung ist eingeschaltet."),
    ).not.toBeInTheDocument();
    expect(mfa.confirm).toHaveBeenCalledTimes(1);
    expect(logout).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Neu anmelden" }));
    await waitFor(() => expect(logout).toHaveBeenCalledTimes(1));
  });

  it("keeps a wrong code in the card", async () => {
    const { ApiError } = await import("~/lib/api-error");
    mfa.status.mockResolvedValue({ enabled: false, maskedEmail: "" });
    mfa.start.mockResolvedValue({
      challengeToken: "challenge",
      maskedEmail: "k***@example.com",
    });
    mfa.confirm.mockRejectedValue(
      new ApiError("invalid", 401, { code: "identity.mfa_code_invalid" }),
    );
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    render(<CatererAccountPage />, { wrapper: ToastProvider });

    fireEvent.click(await screen.findByRole("button", { name: "Einschalten" }));
    fireEvent.change(await screen.findByLabelText("Code"), {
      target: { value: "000000" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Einschalten" }));

    expect(
      await screen.findByText(
        catalogText(
          "identity.mfa_code_invalid",
          "das Einschalten der Zwei-Faktor-Anmeldung",
        ),
      ),
    ).toBeInTheDocument();
    expect(assign).not.toHaveBeenCalled();
    expect(mfa.signIn).not.toHaveBeenCalled();
  });

  it("switches it off after the current password", async () => {
    mfa.status.mockResolvedValue({ enabled: true, maskedEmail: "" });
    mfa.disable.mockResolvedValue(undefined);
    render(<CatererAccountPage />, { wrapper: ToastProvider });

    expect(await screen.findByText("Eingeschaltet")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Ausschalten" }));
    fireEvent.change(screen.getByLabelText("Aktuelles Passwort"), {
      target: { value: "Passw0rd!" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Ausschalten" }));

    expect(await screen.findByText("Ausgeschaltet")).toBeInTheDocument();
    expect(mfa.disable).toHaveBeenCalledWith("Passw0rd!");
  });

  it("shows the remembered devices only while the factor is on", async () => {
    mfa.status.mockResolvedValue({ enabled: true, maskedEmail: "" });
    mfa.disable.mockResolvedValue(undefined);
    render(<CatererAccountPage />, { wrapper: ToastProvider });

    expect(await screen.findByText("Eingeschaltet")).toBeInTheDocument();
    expect(screen.getByText("Gemerkte Geräte (Karte)")).toBeInTheDocument();

    // Switching off deletes the remembered devices in the backend.
    fireEvent.click(screen.getByRole("button", { name: "Ausschalten" }));
    fireEvent.change(screen.getByLabelText("Aktuelles Passwort"), {
      target: { value: "Passw0rd!" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Ausschalten" }));

    expect(await screen.findByText("Ausgeschaltet")).toBeInTheDocument();
    expect(
      screen.queryByText("Gemerkte Geräte (Karte)"),
    ).not.toBeInTheDocument();
  });
});
