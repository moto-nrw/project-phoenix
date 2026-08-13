import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const messages = {
  "guardianInvite.eyebrow": "Eltern-Portal",
  "guardianInvite.title": "Willkommen",
  "guardianInvite.fallbackSchool": "deiner Schule",
  "guardianInvite.subtitle":
    "Bestätige deine Einladung für {school} und lege dein persönliches Passwort fest.",
  "guardianInvite.errors.expired":
    "Diese Einladung ist abgelaufen oder wurde bereits verwendet.",
  "guardianInvite.contactOgs": "Kontaktieren Sie bitte Ihre OGS.",
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
  AuthShell: ({ children }: { children: React.ReactNode }) => (
    <main>{children}</main>
  ),
}));

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

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(
      "Diese Einladung ist abgelaufen oder wurde bereits verwendet.",
    );
    expect(alert).toHaveTextContent("Kontaktieren Sie bitte Ihre OGS.");
    expect(screen.queryByText(/moto-(Team|Support)/i)).not.toBeInTheDocument();
  });
});
