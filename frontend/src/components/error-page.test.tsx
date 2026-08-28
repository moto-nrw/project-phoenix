import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { MapPinOff } from "lucide-react";
import {
  ErrorPage,
  ErrorPage404Visual,
  ErrorPageIconVisual,
} from "./error-page";
import { ErrorPageBackButton } from "./error-page-back-button";

describe("ErrorPage", () => {
  it("renders title as heading, description, and actions", () => {
    render(
      <ErrorPage
        visual={<ErrorPage404Visual />}
        title="Seite nicht gefunden"
        description="Diese Seite gibt es nicht."
        actions={<button type="button">Zur Startseite</button>}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Seite nicht gefunden" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Diese Seite gibt es nicht.")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Zur Startseite" }),
    ).toBeInTheDocument();
  });

  it("renders without an actions row", () => {
    render(
      <ErrorPage
        visual={<ErrorPageIconVisual icon={MapPinOff} />}
        title="Schule nicht gefunden"
        description="Unter dieser Adresse gibt es keine Schule."
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Schule nicht gefunden" }),
    ).toBeInTheDocument();
  });
});

describe("ErrorPageBackButton", () => {
  it("navigates back in browser history on click", () => {
    const backSpy = vi
      .spyOn(window.history, "back")
      .mockImplementation(() => undefined);

    render(<ErrorPageBackButton label="Zurück" />);
    screen.getByRole("button", { name: "Zurück" }).click();

    expect(backSpy).toHaveBeenCalledTimes(1);
    backSpy.mockRestore();
  });
});
