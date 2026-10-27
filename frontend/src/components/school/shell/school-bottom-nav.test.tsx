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

  // Auch ohne Team-Chat sind es sechs Ziele: breite Abstände würden auf
  // 360 px überlaufen. Beide Besetzungen verwenden die kompakte Darstellung.
  it("verwendet bei sechs Zielen ohne Team-Chat die kompakte Darstellung", () => {
    mockPathname.value = "/aufsichten";
    render(<SchoolBottomNav teamChat={teamChat} />);

    expect(document.querySelectorAll("[data-school-nav-item]")).toHaveLength(6);
    expect(screen.getByText("Meine Aufsichten")).toHaveClass(
      "hidden",
      "min-[480px]:inline",
    );
    expect(screen.getByRole("list")).toHaveClass("gap-0");
    expect(screen.getByRole("link", { name: "Elternbriefe" })).toHaveClass(
      "px-2.5",
    );
  });

  // Mit den Elternbriefen (#4800) sind es mit Team-Chat sieben Ziele: die
  // Icons rücken zusammen, die Beschriftung kommt erst ab 480 px dazu.
  it("blendet die Beschriftung bei sieben Zielen mit Team-Chat unterhalb von 480 px aus", () => {
    mockPathname.value = "/aufsichten";
    render(<SchoolBottomNav teamChat={{ unreadCount: 0, available: true }} />);

    expect(document.querySelectorAll("[data-school-nav-item]")).toHaveLength(7);
    expect(screen.getByText("Meine Aufsichten")).toHaveClass(
      "hidden",
      "min-[480px]:inline",
    );
    expect(screen.getByRole("link", { name: "Elternbriefe" })).toHaveClass(
      "px-2.5",
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
