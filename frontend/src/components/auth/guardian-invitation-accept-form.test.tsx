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

vi.mock("~/lib/guardian-invitation-api", () => ({
  acceptGuardianInvitation: mocks.acceptGuardianInvitation,
}));

import deMessages from "~/i18n/messages/de.json";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
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

    // The success screen redirects after 1.5 s; drive it with fake timers.
    vi.useFakeTimers({ shouldAdvanceTime: true });
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
    await vi.advanceTimersByTimeAsync(1600);
    expect(window.location.href).toBe("https://parents.example.test/login");
  });

  it("falls back to router push when parent hostname is not configured", async () => {
    delete process.env.NEXT_PUBLIC_PARENTS_HOSTNAME;
    mocks.acceptGuardianInvitation.mockResolvedValueOnce({
      accountId: "5",
      email: "mara@example.test",
    });

    // The success screen redirects after 1.5 s; drive it with fake timers.
    vi.useFakeTimers({ shouldAdvanceTime: true });
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
    await vi.advanceTimersByTimeAsync(1600);
    expect(mocks.push).toHaveBeenCalledWith("/");
  });

  it("shows API and connection errors on the shared error path", async () => {
    mocks.acceptGuardianInvitation.mockRejectedValueOnce(
      new ApiError("gone", 410, { code: "identity.invitation_expired" }),
    );

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
    expect(alert).toHaveTextContent(
      catalogText(
        "identity.invitation_expired",
        deMessages.guardianInvite.errorObject,
      ),
    );
    expect(screen.queryByText(/moto-(Team|Support)/i)).not.toBeInTheDocument();

    mocks.acceptGuardianInvitation
      .mockRejectedValueOnce(
        new ApiError("Failed to fetch", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce({ accountId: "5", email: "mara@example.test" });
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
      await screen.findByText(
        catalogText(
          "general.unavailable",
          deMessages.guardianInvite.errorObject,
        ),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Failed to fetch")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(await screen.findByText("Konto erstellt")).toBeInTheDocument();
  });

  it("checks the passwords before sending and marks the field", async () => {
    render(
      <GuardianInvitationAcceptForm
        token="invite-token"
        invitation={invitation}
      />,
    );
    fillPasswords(acceptedCredential(), "anders");
    fireEvent.click(
      screen.getByRole("button", { name: "Einladung akzeptieren" }),
    );

    expect(
      await screen.findByText(
        deMessages.guardianInvite.formErrors.passwordMismatch,
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Passwort bestätigen")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(mocks.acceptGuardianInvitation).not.toHaveBeenCalled();
  });
});
