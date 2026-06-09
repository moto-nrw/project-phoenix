import { fireEvent, render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { describe, expect, it, vi } from "vitest";

import StartPage from "./page";

describe("StartPage", () => {
  it("shows the intake entry point", () => {
    render(<StartPage />);

    expect(
      screen.getByRole("heading", {
        name: "Erzählt uns kurz, wie eure OGS arbeitet.",
      }),
    ).toBeInTheDocument();
    expect(screen.getAllByText("OGS oder Träger")).toHaveLength(2);

    fireEvent.change(screen.getByLabelText("Name und Rolle"), {
      target: { value: "Anna Müller, OGS-Leitung" },
    });
    fireEvent.change(screen.getByLabelText("Kontakt"), {
      target: { value: "anna@example.test" },
    });
    fireEvent.change(screen.getByLabelText("Wobei soll moto zuerst helfen?"), {
      target: { value: "Anwesenheit und Abholung" },
    });

    const hrefSpy = vi.spyOn(window.location, "href", "set");

    fireEvent.click(
      screen.getByRole("button", { name: "Kontaktaufnahme starten" }),
    );

    expect(hrefSpy).toHaveBeenCalledWith(
      expect.stringContaining("mailto:kontakt@moto.nrw"),
    );
    expect(hrefSpy).toHaveBeenCalledWith(
      expect.stringContaining("Anna%20M%C3%BCller"),
    );

    hrefSpy.mockRestore();
  });
});
