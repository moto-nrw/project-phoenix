import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SchoolSidebar } from "./school-sidebar";

const mockPathname = vi.hoisted(() => ({ value: "/" }));

vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname.value,
}));

vi.mock("~/lib/school-url", () => ({
  schoolPath: (path: string) => path.replace(/^\/school/, "") || "/",
}));

describe("SchoolSidebar", () => {
  it("führt genau die zwei erreichbaren Ziele", () => {
    render(<SchoolSidebar />);

    expect(screen.getByText("Klassenansicht")).toBeInTheDocument();
    expect(screen.getByText("Hilfe")).toBeInTheDocument();
    expect(document.querySelectorAll("[data-school-nav-item]")).toHaveLength(2);
  });

  it("markiert die Klassenansicht auf der Startseite des Schul-Hosts", () => {
    mockPathname.value = "/";
    render(<SchoolSidebar />);

    const item = document.querySelector('[data-school-nav-item="classDay"]');
    expect(item).toHaveAttribute("data-active", "true");
    expect(item).toHaveAttribute("aria-current", "page");
    expect(item).toHaveAttribute("href", "/");
  });

  it("öffnet die Hilfe in einem neuen Tab, ohne /school-Präfix", () => {
    render(<SchoolSidebar />);

    const help = document.querySelector('[data-school-nav-item="help"]');
    expect(help).toHaveAttribute("href", "/help");
    expect(help).toHaveAttribute("target", "_blank");
    expect(help).toHaveAttribute("rel", "noopener noreferrer");
  });
});
