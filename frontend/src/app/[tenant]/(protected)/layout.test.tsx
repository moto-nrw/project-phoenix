import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import ProtectedLayout from "./layout";

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

describe("ProtectedLayout", () => {
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
});
