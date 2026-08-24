import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SchoolBottomNav } from "./school-bottom-nav";

const mockPathname = vi.hoisted(() => ({ value: "/" }));

vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname.value,
}));

vi.mock("~/lib/school-url", () => ({
  schoolPath: (path: string) => path.replace(/^\/school/, "") || "/",
}));

describe("SchoolBottomNav", () => {
  it("beschriftet alle Ziele dauerhaft, nicht nur das aktive", () => {
    mockPathname.value = "/";
    render(<SchoolBottomNav />);

    expect(screen.getByText("Klassenansicht")).toBeInTheDocument();
    expect(screen.getByText("Meine Aufsichten")).toBeInTheDocument();
    expect(screen.getByText("Hilfe")).toBeInTheDocument();
  });

  it("verschwindet ab der Sidebar-Breite", () => {
    render(<SchoolBottomNav />);

    expect(screen.getByRole("navigation")).toHaveClass("lg:hidden");
  });
});
