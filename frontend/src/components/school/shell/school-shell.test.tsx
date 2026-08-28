import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SchoolShell } from "./school-shell";

const mockUseSchoolTeamChatUnread = vi.hoisted(() =>
  vi.fn(() => ({ unreadCount: 0, available: false })),
);

vi.mock("~/components/dashboard/header", () => ({
  Header: () => <header data-testid="global-header">Header</header>,
}));

vi.mock("./school-sidebar", () => ({
  SchoolSidebar: () => <nav data-testid="school-sidebar" />,
}));

vi.mock("./school-bottom-nav", () => ({
  SchoolBottomNav: () => <nav data-testid="school-bottom-nav" />,
}));

vi.mock("~/lib/hooks/use-school-team-chat-unread", () => ({
  useSchoolTeamChatUnread: mockUseSchoolTeamChatUnread,
}));

beforeEach(() => {
  mockUseSchoolTeamChatUnread.mockClear();
});

describe("SchoolShell", () => {
  it("hält die Kopfzeile auf jeder Breite sichtbar — sie trägt das Abmelden", () => {
    render(
      <SchoolShell>
        <div>Inhalt</div>
      </SchoolShell>,
    );

    const headerWrapper = screen.getByTestId("global-header").parentElement;
    expect(headerWrapper).toHaveClass("sticky", "top-0", "z-40");
    expect(headerWrapper).not.toHaveClass("hidden");
  });

  it("rendert beide Navigationen und den Inhalt", () => {
    render(
      <SchoolShell>
        <div>Inhalt</div>
      </SchoolShell>,
    );

    expect(screen.getByTestId("school-sidebar")).toBeInTheDocument();
    expect(screen.getByTestId("school-bottom-nav")).toBeInTheDocument();
    expect(screen.getByText("Inhalt")).toBeInTheDocument();
    expect(mockUseSchoolTeamChatUnread).toHaveBeenCalledTimes(1);
  });
});
