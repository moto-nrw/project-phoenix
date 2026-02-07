import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { AppShell } from "./app-shell";

vi.mock("next/navigation", () => ({
  usePathname: () => "/dashboard",
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
    expect(main.className).toContain("pb-24");
  });
});
