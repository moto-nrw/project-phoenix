import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import deMessages from "~/i18n/messages/de.generated";
import { catalogText } from "~/test/error-catalog-text";

const messages = {
  "guardianInvite.eyebrow": "Eltern-Portal",
  "guardianInvite.title": "Willkommen",
  "guardianInvite.fallbackSchool": "deiner Schule",
  "guardianInvite.subtitle":
    "Bestätige deine Einladung für {school} und lege dein persönliches Passwort fest.",
  "guardianInvite.errorHelpBefore": "Du kannst dich über",
  "guardianInvite.startPage": "die Startseite",
  "guardianInvite.errorHelpAfter": "anmelden.",
} as const;

vi.mock("next-intl/server", () => ({
  getTranslations: (namespace: keyof typeof messages | string) =>
    Promise.resolve((key: string, values?: Record<string, unknown>) => {
      const message = messages[`${namespace}.${key}` as keyof typeof messages];
      if (!message) return `${namespace}.${key}`;
      let result: string = message;
      for (const [name, value] of Object.entries(values ?? {})) {
        result = result.replaceAll(`{${name}}`, String(value));
      }
      return result;
    }),
}));

vi.mock("~/components/auth/auth-shell", () => ({
  AuthShell: ({
    children,
    subtitle,
  }: {
    children: React.ReactNode;
    subtitle?: string;
  }) => (
    <main>
      {subtitle ? <p data-testid="auth-subtitle">{subtitle}</p> : null}
      {children}
    </main>
  ),
}));

// The form needs the app router; these tests look at the page around it.
vi.mock(
  "~/components/auth/guardian-invitation-accept-form",
  async (importOriginal) => ({
    ...(await importOriginal<
      typeof import("~/components/auth/guardian-invitation-accept-form")
    >()),
    GuardianInvitationAcceptForm: () => null,
  }),
);

vi.mock("~/components/parent/language-switcher", () => ({
  LanguageSwitcher: () => null,
}));

vi.mock("~/components/auth/parent-auth-shell-copy", () => ({
  buildParentAuthShellCopy: () => undefined,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://server:8080",
}));

vi.mock("~/lib/tenant-api", () => ({
  loginImageSrc: (path: string) => path,
}));

import AcceptGuardianInvitePage from "./page";

describe("AcceptGuardianInvitePage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("directs parents to their OGS when initial validation returns 410", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(null, { status: 410 }),
    );

    render(
      await AcceptGuardianInvitePage({
        params: Promise.resolve({ token: "expired-token" }),
      }),
    );

    // An uncoded 410 still means the invitation; its catalog text sends
    // parents to their school (#2518).
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(
      catalogText(
        "identity.invitation_expired",
        deMessages.guardianInvite.errorObject,
      ),
    );
    expect(screen.queryByText(/moto-(Team|Support)/i)).not.toBeInTheDocument();
  });

  // A dead link asks for nothing: the subtitle about confirming the
  // invitation and choosing a password would contradict the error (#6294).
  it.each([
    ["the address changed", "identity.guardian_invitation_address_changed"],
    ["the link expired", "identity.invitation_expired"],
  ])("drops the password subtitle when %s", async (_case, code) => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ status: 410, code }), {
        status: 410,
        headers: { "Content-Type": "application/problem+json" },
      }),
    );

    render(
      await AcceptGuardianInvitePage({
        params: Promise.resolve({ token: "dead-token" }),
      }),
    );

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.queryByTestId("auth-subtitle")).not.toBeInTheDocument();
    expect(screen.queryByText(/Passwort/)).not.toBeInTheDocument();
  });

  it("keeps the password subtitle for a valid invitation", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          data: {
            email: "eltern@example.test",
            expires_at: "2026-11-11T12:00:00Z",
            school_name: "OGS Am Lindenpark",
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    render(
      await AcceptGuardianInvitePage({
        params: Promise.resolve({ token: "valid-token" }),
      }),
    );

    expect(screen.getByTestId("auth-subtitle")).toHaveTextContent(
      "Bestätige deine Einladung für OGS Am Lindenpark und lege dein persönliches Passwort fest.",
    );
  });

  it("shows the code the backend sends, not its sentence", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      Response.json(
        {
          code: "identity.invitation_not_found",
          message: "invitation row 42 missing",
        },
        { status: 404 },
      ),
    );

    render(
      await AcceptGuardianInvitePage({
        params: Promise.resolve({ token: "unknown-token" }),
      }),
    );

    expect(
      await screen.findByText(
        catalogText(
          "identity.invitation_not_found",
          deMessages.guardianInvite.errorObject,
        ),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/row 42/)).not.toBeInTheDocument();
  });

  it("offers a retry when the invitation service is unreachable", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValueOnce(
      new TypeError("fetch failed"),
    );

    render(
      await AcceptGuardianInvitePage({
        params: Promise.resolve({ token: "some-token" }),
      }),
    );

    expect(
      await screen.findByText(
        catalogText(
          "general.unavailable",
          deMessages.guardianInvite.errorObject,
        ),
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Wiederholen" }),
    ).toBeInTheDocument();
  });
});
