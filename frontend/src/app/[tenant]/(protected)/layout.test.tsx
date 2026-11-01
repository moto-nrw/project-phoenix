import type { ReactNode } from "react";
import { SessionProvider } from "next-auth/react";
import { afterEach, describe, it, expect, vi } from "vitest";
import {
  act,
  render as renderWithContext,
  screen,
} from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

vi.unmock("next-intl");

import ProtectedLayout from "./layout";

// The layout now includes authenticated navigation data intent. Keep the real
// session provider; these layout-only cases have no signed-in person.
function render(ui: ReactNode) {
  return renderWithContext(
    <SessionProvider session={null}>{ui}</SessionProvider>,
  );
}

const IPHONE_UA =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1";
const SAMSUNG_INTERNET_UA =
  "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/28.0 Chrome/130.0.0.0 Mobile Safari/537.36";

vi.mock("~/lib/breadcrumb-context", () => ({
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="breadcrumb-provider">{children}</div>
  ),
}));

const shellAuth = vi.hoisted(() => ({
  current: undefined as
    { status: "authenticated"; user: { id: string } } | undefined,
}));

vi.mock("~/lib/shell-auth-context", () => ({
  TeacherShellProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="teacher-shell-provider">{children}</div>
  ),
  // Der geführte Einstieg im Layout (#2831) liest die Sitzung hierüber.
  useShellAuthSafe: () => shellAuth.current,
}));

vi.mock("~/components/dashboard/app-shell", () => ({
  AppShell: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="app-shell">{children}</div>
  ),
}));

vi.mock("~/components/platform/announcement-modal", () => ({
  AnnouncementModal: () => <div data-testid="announcement-modal" />,
}));

// Die Einrichtungsassistenten lesen mit angemeldeter Sitzung NextAuth und die
// API; hier geht es nur um Installationshinweis und Benachrichtigungen.
vi.mock("~/components/school-setup/school-setup-wizard", () => ({
  SchoolSetupWizard: () => null,
}));

vi.mock("~/components/staff-onboarding/staff-onboarding-wizard", () => ({
  StaffOnboardingWizard: () => null,
}));

// Die geführte Einrichtung prüft vor dem ersten Schritt die Push-
// Konfiguration der Schule beim Server.
vi.mock("~/lib/push-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/push-api")>()),
  verifyPushConfiguration: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("~/lib/hooks/use-settings-cache-bridge", () => ({
  useSettingsCacheBridge: vi.fn(),
}));

describe("ProtectedLayout", () => {
  afterEach(() => {
    shellAuth.current = undefined;
    vi.unstubAllGlobals();
    localStorage.clear();
    sessionStorage.clear();
  });

  it("renders children inside BreadcrumbProvider and AppShell", () => {
    render(
      <ProtectedLayout>
        <div data-testid="child">Content</div>
      </ProtectedLayout>,
    );

    expect(screen.getByTestId("teacher-shell-provider")).toBeInTheDocument();
    expect(screen.getByTestId("breadcrumb-provider")).toBeInTheDocument();
    expect(screen.getByTestId("app-shell")).toBeInTheDocument();
    expect(screen.getByTestId("child")).toBeInTheDocument();
  });

  it("nests AppShell inside BreadcrumbProvider", () => {
    render(
      <ProtectedLayout>
        <span>Nested</span>
      </ProtectedLayout>,
    );

    const provider = screen.getByTestId("breadcrumb-provider");
    const shell = screen.getByTestId("app-shell");
    expect(provider).toContainElement(shell);
  });

  it("uses only the guided notification setup on a mobile device", async () => {
    vi.stubGlobal("navigator", {
      userAgent: IPHONE_UA,
      platform: "iPhone",
      maxTouchPoints: 1,
    });
    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => ({
        matches: false,
        media: "(display-mode: standalone)",
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      })),
    );
    localStorage.setItem("moto-pwa-install-hint-visits", "1");
    shellAuth.current = { status: "authenticated", user: { id: "5" } };

    render(
      <ProtectedLayout>
        <div>Geschützter Inhalt</div>
      </ProtectedLayout>,
    );

    // Wait for the real deferred dialog import, including its dependencies,
    // before the DOM assertion's timeout starts. Cold transforms can exceed it.
    await act(async () => {
      await import("~/components/notifications/notification-setup-dialog");
    });

    // Anker: die geführte Einrichtung hat den Installationsschritt für das
    // iPhone übernommen.
    expect(
      await screen.findByRole("heading", {
        name: "moto zum Home-Bildschirm hinzufügen",
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "moto als App nutzen" }),
    ).not.toBeInTheDocument();
  });

  it("keeps the Chrome guidance for Samsung Internet", async () => {
    vi.stubGlobal("navigator", {
      userAgent: SAMSUNG_INTERNET_UA,
      platform: "Linux armv81",
      maxTouchPoints: 1,
    });
    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => ({
        matches: false,
        media: "(display-mode: standalone)",
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      })),
    );
    localStorage.setItem("moto-pwa-install-hint-visits", "1");

    render(
      <ProtectedLayout>
        <div>Geschützter Inhalt</div>
      </ProtectedLayout>,
    );

    expect(
      await screen.findByRole("heading", { name: "moto als App nutzen" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/nicht über Samsung Internet/)).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "In Chrome öffnen" }),
    ).toBeInTheDocument();
  });
});
