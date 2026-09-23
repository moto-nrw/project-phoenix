import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { AppShell } from "./app-shell";

vi.mock("next/navigation", () => ({
  usePathname: () => "/dashboard",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("./header", () => ({
  Header: () => <header data-testid="header">Header</header>,
}));

vi.mock("./sidebar", () => ({
  Sidebar: ({ className }: { className?: string }) => (
    <nav data-testid="sidebar" className={className}>
      Sidebar
    </nav>
  ),
}));

vi.mock("~/components/demo/demo-banner", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/components/demo/demo-banner")>()),
  DemoBanner: () => <div data-testid="demo-banner">Demo</div>,
}));

const shellAuth = vi.hoisted(() => ({
  useShellAuthSafe: vi.fn<() => { mode: string; isPreview?: boolean } | null>(
    () => null,
  ),
}));
vi.mock("~/lib/shell-auth-context", () => shellAuth);

vi.mock("./mobile-bottom-nav", () => ({
  MobileBottomNav: () => <nav data-testid="mobile-nav">Mobile Nav</nav>,
}));

describe("AppShell", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders children content", () => {
    render(
      <AppShell>
        <div data-testid="child-content">Page Content</div>
      </AppShell>,
    );

    expect(screen.getByTestId("child-content")).toBeInTheDocument();
    expect(screen.getByText("Page Content")).toBeInTheDocument();
  });

  it("renders header, sidebar, and mobile nav", () => {
    render(
      <AppShell>
        <div>Content</div>
      </AppShell>,
    );

    expect(screen.getByTestId("header")).toBeInTheDocument();
    expect(screen.getByTestId("sidebar")).toBeInTheDocument();
    expect(screen.getByTestId("mobile-nav")).toBeInTheDocument();
  });

  it("hides sidebar on mobile via CSS class", () => {
    render(
      <AppShell>
        <div>Content</div>
      </AppShell>,
    );

    const sidebar = screen.getByTestId("sidebar");
    expect(sidebar.className).toContain("hidden");
    expect(sidebar.className).toContain("lg:block");
  });

  it("renders main content area with correct classes", () => {
    render(
      <AppShell>
        <div data-testid="page">Page</div>
      </AppShell>,
    );

    const main = screen.getByRole("main");
    expect(main).toBeInTheDocument();
    expect(main.className).toContain("flex-1");
    expect(main.className).toContain(
      "pb-[calc(7rem+env(safe-area-inset-bottom))]",
    );
    expect(main.className).toContain(
      "md:pb-[calc(7rem+env(safe-area-inset-bottom))]",
    );
    expect(main.className).not.toContain("moto-dotted-background");
  });

  // The demo banner (#3467) lies fixed above everything, like the staff
  // preview strip: the shell moves down by its height, so the banner covers
  // neither the header nor the page, and the bottom bar stays free.
  it("moves the OGS app below the demo banner in the demo build only", () => {
    shellAuth.useShellAuthSafe.mockReturnValue({ mode: "teacher" });
    const { container, unmount } = render(
      <AppShell>
        <div>Content</div>
      </AppShell>,
    );
    expect(screen.queryByTestId("demo-banner")).toBeNull();
    expect(container.firstElementChild?.className ?? "").not.toContain("pt-12");
    unmount();

    vi.stubEnv("NEXT_PUBLIC_APP_ENV", "demo");
    try {
      const demo = render(
        <AppShell>
          <div>Content</div>
        </AppShell>,
      );
      expect(screen.getByTestId("demo-banner")).toBeInTheDocument();
      expect(demo.container.firstElementChild?.className).toContain("pt-12");
      expect(screen.getByTestId("header").parentElement?.className).toContain(
        "top-12",
      );
    } finally {
      vi.unstubAllEnvs();
      shellAuth.useShellAuthSafe.mockReturnValue(null);
    }
  });

  // The operator portal shares the shell but is no part of the OGS app; a
  // role switch there would have no demo school to switch in.
  it("keeps the operator portal free of the demo banner", () => {
    shellAuth.useShellAuthSafe.mockReturnValue({ mode: "operator" });
    vi.stubEnv("NEXT_PUBLIC_APP_ENV", "demo");
    try {
      const { container } = render(
        <AppShell>
          <div>Content</div>
        </AppShell>,
      );
      expect(screen.queryByTestId("demo-banner")).toBeNull();
      expect(container.firstElementChild?.className ?? "").not.toContain(
        "pt-12",
      );
    } finally {
      vi.unstubAllEnvs();
      shellAuth.useShellAuthSafe.mockReturnValue(null);
    }
  });
});
