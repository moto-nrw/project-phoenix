import { afterEach, describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

vi.unmock("next-intl");

import ProtectedLayout from "./layout";

const IPHONE_UA =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1";

vi.mock("~/lib/breadcrumb-context", () => ({
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="breadcrumb-provider">{children}</div>
  ),
}));

vi.mock("~/lib/shell-auth-context", () => ({
  TeacherShellProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="teacher-shell-provider">{children}</div>
  ),
}));

vi.mock("~/components/dashboard/app-shell", () => ({
  AppShell: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="app-shell">{children}</div>
  ),
}));

vi.mock("~/components/platform/announcement-modal", () => ({
  AnnouncementModal: () => <div data-testid="announcement-modal" />,
}));

vi.mock("~/lib/hooks/use-settings-cache-bridge", () => ({
  useSettingsCacheBridge: vi.fn(),
}));

describe("ProtectedLayout", () => {
  afterEach(() => {
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

  it("renders the install hint with the real translation provider", async () => {
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

    render(
      <ProtectedLayout>
        <div>Geschützter Inhalt</div>
      </ProtectedLayout>,
    );

    expect(
      await screen.findByRole("heading", { name: "moto als App nutzen" }),
    ).toBeInTheDocument();
  });
});
