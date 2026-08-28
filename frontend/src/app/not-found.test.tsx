import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import NotFound from "./not-found";

describe("root not-found page", () => {
  it("renders the moto 404 with a working exit", () => {
    render(<NotFound />);

    expect(screen.getByRole("main")).toHaveClass(
      "moto-dotted-background",
      "moto-dotted-background--fullscreen",
    );
    expect(
      screen.getByRole("heading", { name: "Seite nicht gefunden" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Zur Startseite" }),
    ).toHaveAttribute("href", "/");
  });
});
