import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";

const { passthrough, sessionData } = vi.hoisted(() => ({
  passthrough: ({ children }: { children?: ReactNode }) => children,
  sessionData: {
    current: { user: { id: "41", scope: "school" } } as Record<string, unknown>,
  },
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  usePathname: () => "/school",
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: sessionData.current,
    status: "authenticated",
  }),
}));
vi.mock("~/lib/shell-auth-context", () => ({
  SchoolShellProvider: passthrough,
}));
vi.mock("~/lib/breadcrumb-context", () => ({
  BreadcrumbProvider: passthrough,
}));
vi.mock("~/components/school/shell/school-shell", () => ({
  SchoolShell: passthrough,
}));
vi.mock("~/components/school/school-realtime-bridge", () => ({
  SchoolRealtimeBridge: () => null,
}));
vi.mock("~/components/dashboard/shell-intl-provider", () => ({
  ShellIntlProvider: passthrough,
}));
vi.mock("~/components/tenant/pwa-install-hint", () => ({
  PwaInstallHint: () => null,
}));
vi.mock("~/components/notifications/portal-notification-setup", () => ({
  PortalNotificationSetup: () => null,
}));

import { SchoolAuthGuard } from "./auth-guard";

describe("SchoolAuthGuard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionData.current = { user: { id: "41", scope: "school" } };
  });

  // #6187: a renewal refused with module.not_booked keeps the session and
  // marks it, also after the access token expired and the page load could
  // not ask the backend.
  it("explains the unused portal when the session's renewal found it not booked", () => {
    sessionData.current = {
      user: { id: "41", scope: "school" },
      notBooked: true,
    };
    render(
      <SchoolAuthGuard>
        <div>Klassenseite</div>
      </SchoolAuthGuard>,
    );

    expect(
      screen.getByText(/Ihre Schule nutzt moto schule zurzeit nicht/),
    ).toBeInTheDocument();
    expect(screen.queryByText("Klassenseite")).not.toBeInTheDocument();
  });

  // #6187: the page load's session check names the unbooked module, so the
  // portal explains it before any request is refused.
  it("explains the unused portal instead of the page", () => {
    render(
      <SchoolAuthGuard notBooked>
        <div>Klassenseite</div>
      </SchoolAuthGuard>,
    );

    expect(
      screen.getByText(/Ihre Schule nutzt moto schule zurzeit nicht/),
    ).toBeInTheDocument();
    expect(screen.queryByText("Klassenseite")).not.toBeInTheDocument();
  });

  it("shows the page of a booked portal", () => {
    render(
      <SchoolAuthGuard>
        <div>Klassenseite</div>
      </SchoolAuthGuard>,
    );

    expect(screen.getByText("Klassenseite")).toBeInTheDocument();
  });
});
