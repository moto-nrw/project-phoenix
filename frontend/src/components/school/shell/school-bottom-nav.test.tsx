import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SchoolBottomNav } from "./school-bottom-nav";

const teamChat = { unreadCount: 0, available: false } as const;

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
  // schoben "Hilfe" aus dem Bild. Erreichbar bleiben alle ueber ihr
  // aria-label.
  it("beschriftet das aktive Ziel und benennt die uebrigen fuer Screenreader", () => {
    mockPathname.value = "/";
    render(<SchoolBottomNav teamChat={teamChat} />);

    expect(screen.getByText("Klassenansicht")).toBeInTheDocument();
    expect(screen.queryByText("Meine Aufsichten")).not.toBeInTheDocument();
    expect(screen.queryByText("Tagesinformationen")).not.toBeInTheDocument();
    expect(screen.queryByText("Hilfe")).not.toBeInTheDocument();

    expect(
      screen.getByRole("link", { name: "Meine Aufsichten" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Tagesinformationen" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Hilfe" })).toBeInTheDocument();
  });

  it("verschiebt die Beschriftung mit dem aktiven Ziel", () => {
    mockPathname.value = "/aufsichten";
    render(<SchoolBottomNav teamChat={teamChat} />);

    expect(screen.getByText("Meine Aufsichten")).toBeInTheDocument();
    expect(screen.queryByText("Klassenansicht")).not.toBeInTheDocument();
  });

  // Ab dem vierten Ziel passt die laengste Beschriftung erst ab 420 px in
  // die Pille. Mit den Tagesinformationen (#2208) sind es immer mindestens
  // vier Ziele, mit Team-Chat fuenf — auf schmaleren Geraeten blendet CSS die
  // Beschriftung aus, statt ein Ziel aus der Leiste zu schieben.
  it("blendet die Beschriftung bei vier Zielen unterhalb von 420 px aus", () => {
    mockPathname.value = "/aufsichten";
    render(<SchoolBottomNav teamChat={teamChat} />);

    expect(screen.getByText("Meine Aufsichten")).toHaveClass(
      "hidden",
      "min-[420px]:inline",
    );
  });

  it("blendet die Beschriftung auch bei fuenf Zielen mit Team-Chat aus", () => {
    mockPathname.value = "/aufsichten";
    render(<SchoolBottomNav teamChat={{ unreadCount: 0, available: true }} />);

    expect(document.querySelectorAll("[data-school-nav-item]")).toHaveLength(5);
    expect(screen.getByText("Meine Aufsichten")).toHaveClass(
      "hidden",
      "min-[420px]:inline",
    );
  });

  it("zeigt offene Tagesinformationen als Zahl am Icon (#2208)", () => {
    mockPathname.value = "/";
    render(
      <SchoolBottomNav teamChat={teamChat} notices={{ pendingCount: 3 }} />,
    );

    expect(
      screen.getByLabelText("3 offene Tagesinformationen"),
    ).toBeInTheDocument();
  });

  it("verschwindet ab der Sidebar-Breite", () => {
    render(<SchoolBottomNav teamChat={teamChat} />);

    expect(screen.getByRole("navigation")).toHaveClass("lg:hidden");
  });
});
