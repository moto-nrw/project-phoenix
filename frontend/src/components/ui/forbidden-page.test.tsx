import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { ForbiddenPage } from "./forbidden-page";

describe("ForbiddenPage", () => {
  it("renders with default title and message", () => {
    render(<ForbiddenPage />);

    expect(screen.getByText("Kein Zugriff")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Du verfügst nicht über die notwendigen Berechtigungen, um diese Seite aufzurufen.",
      ),
    ).toBeInTheDocument();
  });

  it("renders with custom title", () => {
    render(<ForbiddenPage title="Keine Berechtigung" />);

    expect(screen.getByText("Keine Berechtigung")).toBeInTheDocument();
  });

  it("renders with custom message", () => {
    render(
      <ForbiddenPage message="Du darfst die Datenverwaltung nicht aufrufen." />,
    );

    expect(
      screen.getByText("Du darfst die Datenverwaltung nicht aufrufen."),
    ).toBeInTheDocument();
  });

  it("renders warning icon", () => {
    const { container } = render(<ForbiddenPage />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveClass("text-moto-red");
  });
});
