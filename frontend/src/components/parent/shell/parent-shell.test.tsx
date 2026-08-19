import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ParentShell } from "./parent-shell";

vi.mock("swr", () => ({
  default: () => ({ data: [] }),
}));

vi.mock("~/components/dashboard/header", () => ({
  Header: () => <header data-testid="global-header">Header</header>,
}));

vi.mock("./parent-sidebar", () => ({
  ParentSidebar: () => <nav data-testid="parent-sidebar" />,
}));

vi.mock("./parent-bottom-nav", () => ({
  ParentBottomNav: () => <nav data-testid="parent-bottom-nav" />,
}));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({ status: "authenticated" }),
}));

vi.mock("~/lib/hooks/use-parent-messages-unread", () => ({
  useParentMessagesUnread: () => ({ unreadCount: 0 }),
}));

vi.mock("~/lib/hooks/use-parent-news-unread", () => ({
  useParentNewsUnread: () => ({ unreadCount: 0 }),
}));

vi.mock("~/lib/hooks/use-parent-news-enabled", () => ({
  useParentNewsEnabled: () => false,
}));

vi.mock("~/lib/hooks/use-parent-meal-plan-enabled", () => ({
  useParentMealPlanEnabled: () => false,
}));

describe("ParentShell", () => {
  it("hides only the parent header on mobile and preserves the top safe area", () => {
    render(
      <ParentShell>
        <div>Inhalt</div>
      </ParentShell>,
    );

    expect(screen.getByTestId("global-header").parentElement).toHaveClass(
      "hidden",
      "lg:block",
      "sticky",
      "top-0",
      "z-50",
    );
    expect(document.querySelector("[data-parent-safe-area-top]")).toHaveClass(
      "h-[env(safe-area-inset-top)]",
      "lg:hidden",
    );
    expect(screen.getByTestId("parent-bottom-nav")).toBeInTheDocument();
  });
});
