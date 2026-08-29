import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SchoolSidebar } from "./school-sidebar";

const teamChat = { unreadCount: 0, available: false } as const;

const mockPathname = vi.hoisted(() => ({ value: "/" }));

vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname.value,
}));

vi.mock("~/lib/school-url", () => ({
  schoolPath: (path: string) => path.replace(/^\/school/, "") || "/",
}));

describe("SchoolSidebar", () => {
  it("führt genau die drei erreichbaren Ziele", () => {
    render(<SchoolSidebar teamChat={teamChat} />);

    expect(screen.getByText("Klassenansicht")).toBeInTheDocument();
    // Seit #2527 führt eine Lehrkraft auch ihre eigenen Aufsichten.
    expect(screen.getByText("Meine Aufsichten")).toBeInTheDocument();
    expect(screen.getByText("Hilfe")).toBeInTheDocument();
    expect(document.querySelectorAll("[data-school-nav-item]")).toHaveLength(3);
  });

  it("markiert die Aufsichten auf ihrer Seite des Schul-Hosts (#2527)", () => {
    mockPathname.value = "/aufsichten";
    render(<SchoolSidebar teamChat={teamChat} />);

    const item = document.querySelector(
      '[data-school-nav-item="supervisions"]',
    );
    expect(item).toHaveAttribute("data-active", "true");
    expect(item).toHaveAttribute("href", "/aufsichten");
    expect(
      document.querySelector('[data-school-nav-item="classDay"]'),
    ).toHaveAttribute("data-active", "false");
  });

  it("markiert die Klassenansicht auf der Startseite des Schul-Hosts", () => {
    mockPathname.value = "/";
    render(<SchoolSidebar teamChat={teamChat} />);

    const item = document.querySelector('[data-school-nav-item="classDay"]');
    expect(item).toHaveAttribute("data-active", "true");
    expect(item).toHaveAttribute("aria-current", "page");
    expect(item).toHaveAttribute("href", "/");
  });

  it("öffnet die Hilfe in einem neuen Tab, ohne /school-Präfix", () => {
    render(<SchoolSidebar teamChat={teamChat} />);

    const help = document.querySelector('[data-school-nav-item="help"]');
    expect(help).toHaveAttribute("href", "/help");
    expect(help).toHaveAttribute("target", "_blank");
    expect(help).toHaveAttribute("rel", "noopener noreferrer");
  });
});
