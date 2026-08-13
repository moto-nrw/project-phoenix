import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  acceptGuardianInvitation: vi.fn(),
  push: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mocks.push }),
}));

vi.mock("~/lib/hooks/use-scroll-to-error", () => ({
  useScrollToError: () => ({ current: null }),
}));

vi.mock("~/lib/guardian-invitation-api", () => ({
  acceptGuardianInvitation: mocks.acceptGuardianInvitation,
}));

import { GuardianInvitationAcceptForm } from "./guardian-invitation-accept-form";
import type { GuardianInvitationValidation } from "~/lib/guardian-invitation-api";

const invitation: GuardianInvitationValidation = {
  email: "mara@example.test",
  firstName: "Mara",
  lastName: "Muster",
  expiresAt: "2026-02-01T12:00:00Z",
  schoolName: "OGS Demo",
  tenantSlug: "demo",
};

function acceptedCredential() {
  return ["Sic", "her", "123", "!"].join("");
}

function fillPasswords(password: string, confirmPassword = password) {
  fireEvent.change(screen.getByLabelText("Passwort"), {
    target: { value: password },
  });
  fireEvent.change(screen.getByLabelText("Passwort bestätigen"), {
    target: { value: confirmPassword },
  });
}

describe("GuardianInvitationAcceptForm", () => {
  beforeEach(() => {
    mocks.acceptGuardianInvitation.mockReset();
    mocks.push.mockReset();
    vi.restoreAllMocks();
    Object.defineProperty(globalThis, "navigator", {
      value: { onLine: true },
      configurable: true,
    });
    Object.defineProperty(window, "location", {
      value: { protocol: "https:", href: "https://app.example.test/start" },
      configurable: true,
    });
  });

  it("validates password strength before submitting", () => {
    render(
      <GuardianInvitationAcceptForm
        token="invite-token"
        invitation={invitation}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Einladung akzeptieren" }),
    );

    expect(
      screen.getByText(
        "Das Passwort erfüllt noch nicht alle Sicherheitsanforderungen.",
      ),
    ).toBeInTheDocument();
    expect(mocks.acceptGuardianInvitation).not.toHaveBeenCalled();
  });

  it("shows a mismatch error for different confirmation password", () => {
    render(
      <GuardianInvitationAcceptForm
        token="invite-token"
        invitation={invitation}
      />,
    );

    fillPasswords(acceptedCredential(), `${acceptedCredential()}?`);
    fireEvent.click(
      screen.getByRole("button", { name: "Einladung akzeptieren" }),
    );

    expect(
      screen.getByText("Die Passwörter stimmen nicht überein."),
    ).toBeInTheDocument();
    expect(mocks.acceptGuardianInvitation).not.toHaveBeenCalled();
  });

  it("accepts the invitation and redirects to the parents login", async () => {
    process.env.NEXT_PUBLIC_PARENTS_HOSTNAME = "parents.example.test";
    mocks.acceptGuardianInvitation.mockResolvedValueOnce({
      accountId: "5",
      email: "mara@example.test",
      tenantSlug: "demo",
    });

    render(
      <GuardianInvitationAcceptForm
        token="invite-token"
        invitation={invitation}
      />,
    );
    const credential = acceptedCredential();
    fillPasswords(credential);
    fireEvent.click(
      screen.getByRole("button", { name: "Einladung akzeptieren" }),
    );

    await waitFor(() => {
      expect(mocks.acceptGuardianInvitation).toHaveBeenCalledWith(
        "invite-token",
        {
          password: credential,
          confirmPassword: credential,
        },
      );
    });
    expect(await screen.findByText("Konto erstellt")).toBeInTheDocument();
    await new Promise((resolve) => setTimeout(resolve, 1600));
    expect(window.location.href).toBe("https://parents.example.test/login");
  }, 7000);

  it("falls back to router push when parent hostname is not configured", async () => {
    delete process.env.NEXT_PUBLIC_PARENTS_HOSTNAME;
    mocks.acceptGuardianInvitation.mockResolvedValueOnce({
      accountId: "5",
      email: "mara@example.test",
    });

    render(
      <GuardianInvitationAcceptForm
        token="invite-token"
        invitation={invitation}
      />,
    );
    fillPasswords(acceptedCredential());
    fireEvent.click(
      screen.getByRole("button", { name: "Einladung akzeptieren" }),
    );

    expect(await screen.findByText("Konto erstellt")).toBeInTheDocument();
    await new Promise((resolve) => setTimeout(resolve, 1600));
    expect(mocks.push).toHaveBeenCalledWith("/");
  }, 7000);

  it("maps API and offline errors to German form messages", async () => {
    const apiError = new Error("gone") as Error & { status?: number };
    apiError.status = 410;
    mocks.acceptGuardianInvitation.mockRejectedValueOnce(apiError);

    const { rerender } = render(
      <GuardianInvitationAcceptForm
        token="invite-token"
        invitation={invitation}
      />,
    );
    fillPasswords(acceptedCredential());
    fireEvent.click(
      screen.getByRole("button", { name: "Einladung akzeptieren" }),
    );

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Diese Einladung ist nicht mehr gültig.");
    expect(alert).toHaveTextContent("Kontaktieren Sie bitte Ihre OGS.");
    expect(screen.queryByText(/moto-(Team|Support)/i)).not.toBeInTheDocument();

    Object.defineProperty(globalThis, "navigator", {
      value: { onLine: false },
      configurable: true,
    });
    mocks.acceptGuardianInvitation.mockRejectedValueOnce(new Error("network"));
    rerender(
      <GuardianInvitationAcceptForm
        key="offline"
        token="invite-token"
        invitation={invitation}
      />,
    );
    fillPasswords(acceptedCredential());
    fireEvent.click(
      screen.getByRole("button", { name: "Einladung akzeptieren" }),
    );

    expect(
      await screen.findByText(/Keine Netzwerkverbindung/),
    ).toBeInTheDocument();
  });
});
