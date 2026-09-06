import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PortalShell } from "./portal-shell";

describe("PortalShell", () => {
  it("rendert Kopfzeile, beide Navigationen und den Inhalt", () => {
    render(
      <PortalShell
        header={<header data-testid="header" />}
        sidebar={<nav data-testid="sidebar" />}
        bottomNav={<nav data-testid="bottom-nav" />}
      >
        <div data-testid="content" />
      </PortalShell>,
    );

    expect(screen.getByTestId("header")).toBeInTheDocument();
    expect(screen.getByTestId("sidebar")).toBeInTheDocument();
    expect(screen.getByTestId("bottom-nav")).toBeInTheDocument();
    expect(screen.getByTestId("content")).toBeInTheDocument();
  });

  it("klebt die Kopfzeile ohne eigene Angabe auf Ebene 40", () => {
    render(
      <PortalShell
        header={<header data-testid="header" />}
        sidebar={<nav />}
        bottomNav={<nav />}
      >
        <div />
      </PortalShell>,
    );

    expect(screen.getByTestId("header").parentElement).toHaveClass(
      "sticky",
      "top-0",
      "z-40",
    );
  });

  it("uebernimmt die Kopfzeilen-Klassen des Portals", () => {
    render(
      <PortalShell
        header={<header data-testid="header" />}
        headerClassName="sticky top-0 z-50 hidden lg:block"
        sidebar={<nav />}
        bottomNav={<nav />}
      >
        <div />
      </PortalShell>,
    );

    expect(screen.getByTestId("header").parentElement).toHaveClass(
      "hidden",
      "lg:block",
      "z-50",
    );
  });

  it("legt die obere Schicht vor die Kopfzeile, ohne Inhaltsflaeche zu faerben", () => {
    render(
      <PortalShell
        header={<header data-testid="header" />}
        topLayer={<div data-testid="top-layer" />}
        sidebar={<nav />}
        bottomNav={<nav />}
      >
        <div />
      </PortalShell>,
    );

    const topLayer = screen.getByTestId("top-layer");
    const headerWrapper = screen.getByTestId("header").parentElement;
    expect(
      topLayer.compareDocumentPosition(headerWrapper as Node) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();

    const main = screen.getByRole("main");
    expect(main.className).toContain("flex-1");
    expect(main.className).not.toContain("moto-dotted-background");
  });

  it("reicht eine Flex-Spalte bis zur Inhaltshuelle durch, damit eine Seite die Hoehe fuellen kann", () => {
    render(
      <PortalShell
        header={<header />}
        contentClassName="flex flex-1 flex-col"
        sidebar={<nav />}
        bottomNav={<nav />}
      >
        <div data-testid="content" />
      </PortalShell>,
    );

    const main = screen.getByRole("main");
    // Jede Stufe waechst in der naechsten: Wurzel -> Zeile -> main -> Huelle.
    expect(main.className).toContain("flex-col");
    expect(main.parentElement).toHaveClass("flex", "flex-1");
    expect(main.parentElement?.parentElement).toHaveClass(
      "flex",
      "min-h-screen",
      "flex-col",
    );
    expect(screen.getByTestId("content").parentElement).toHaveClass(
      "flex",
      "flex-1",
      "flex-col",
    );
  });

  it("laesst die Inhaltshuelle ohne Angabe ein gewoehnlicher Block", () => {
    render(
      <PortalShell header={<header />} sidebar={<nav />} bottomNav={<nav />}>
        <div data-testid="content" />
      </PortalShell>,
    );

    expect(screen.getByTestId("content").parentElement).not.toHaveClass("flex");
  });
});
