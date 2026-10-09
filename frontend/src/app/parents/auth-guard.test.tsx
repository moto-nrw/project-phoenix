import { describe, it, expect, vi, beforeEach } from "vitest";
import { render } from "@testing-library/react";

const { mockRedirect, mockUsePathname, mockUseSession, mockUseSWR } =
  vi.hoisted(() => ({
    mockRedirect: vi.fn(),
    mockUsePathname: vi.fn(),
    mockUseSession: vi.fn(),
    mockUseSWR: vi.fn(),
  }));

vi.mock("next/navigation", () => ({
  redirect: mockRedirect,
  usePathname: mockUsePathname,
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

vi.mock("swr", () => ({ default: mockUseSWR }));

vi.mock("~/lib/parent-api", () => ({
  fetchParentProfile: vi.fn(),
  parentProfileCacheKey: (accountID: string) => ["parent-profile", accountID],
}));

vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => path,
}));

vi.mock("~/lib/shell-auth-context", () => ({
  ParentShellProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

vi.mock("~/components/parent/shell/parent-shell", () => ({
  ParentShell: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock("~/components/parent/parent-page", () => ({
  ParentPageSkeleton: () => null,
}));

vi.mock("~/components/parent/parent-realtime-bridge", () => ({
  ParentRealtimeBridge: () => null,
}));

vi.mock("~/components/parent/parent-notification-onboarding", () => ({
  ParentNotificationOnboarding: () => null,
}));

import { ParentAuthGuard } from "./auth-guard";

describe("ParentAuthGuard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUsePathname.mockReturnValue("/parents");
    mockUseSession.mockReturnValue({
      data: { user: { id: "parent-1", scope: "parent" } },
      status: "authenticated",
    });
    mockUseSWR.mockReturnValue({ data: undefined });
  });

  it("separates cached profiles by authenticated parent account", () => {
    const { rerender } = render(
      <ParentAuthGuard>
        <div>Elternbereich</div>
      </ParentAuthGuard>,
    );

    expect(mockUseSWR.mock.calls.at(-1)?.[0]).toEqual([
      "parent-profile",
      "parent-1",
    ]);

    mockUseSession.mockReturnValue({
      data: { user: { id: "parent-2", scope: "parent" } },
      status: "authenticated",
    });
    rerender(
      <ParentAuthGuard>
        <div>Elternbereich</div>
      </ParentAuthGuard>,
    );

    expect(mockUseSWR.mock.calls.at(-1)?.[0]).toEqual([
      "parent-profile",
      "parent-2",
    ]);
  });
});
