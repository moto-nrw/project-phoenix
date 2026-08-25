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
  // Seit dem dritten Ziel (#2527) traegt nur das aktive Feld seine
  // Beschriftung: drei volle Namen passen auf 390 px nicht in die Pille und
  // schoben "Hilfe" aus dem Bild. Erreichbar bleiben alle drei ueber ihr
  // aria-label.
  it("beschriftet das aktive Ziel und benennt die uebrigen fuer Screenreader", () => {
    mockPathname.value = "/";
    render(<SchoolBottomNav />);

    expect(screen.getByText("Klassenansicht")).toBeInTheDocument();
    expect(screen.queryByText("Meine Aufsichten")).not.toBeInTheDocument();
    expect(screen.queryByText("Hilfe")).not.toBeInTheDocument();

    expect(
      screen.getByRole("link", { name: "Meine Aufsichten" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Hilfe" })).toBeInTheDocument();
  });

  it("verschiebt die Beschriftung mit dem aktiven Ziel", () => {
    mockPathname.value = "/aufsichten";
    render(<SchoolBottomNav />);

    expect(screen.getByText("Meine Aufsichten")).toBeInTheDocument();
    expect(screen.queryByText("Klassenansicht")).not.toBeInTheDocument();
  });

  it("verschwindet ab der Sidebar-Breite", () => {
    render(<SchoolBottomNav />);

    expect(screen.getByRole("navigation")).toHaveClass("lg:hidden");
  });
});
