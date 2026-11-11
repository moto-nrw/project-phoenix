import { fireEvent, render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { getPillTone } from "~/lib/kit-palette";
import { Alert } from "./alert";

describe("Alert", () => {
  it("renders error alert with message", () => {
    render(<Alert type="error" message="Something went wrong" />);

    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
  });

  it("renders success alert with message", () => {
    render(<Alert type="success" message="Operation successful" />);

    expect(screen.getByText("Operation successful")).toBeInTheDocument();
  });

  it("renders warning alert with message", () => {
    render(<Alert type="warning" message="Please be careful" />);

    expect(screen.getByText("Please be careful")).toBeInTheDocument();
  });

  it("renders info alert with message", () => {
    render(<Alert type="info" message="Here is some information" />);

    expect(screen.getByText("Here is some information")).toBeInTheDocument();
  });

  it("returns null when message is empty", () => {
    const { container } = render(<Alert type="error" message="" />);

    expect(container.firstChild).toBeNull();
  });

  it("renders a numeric message of zero", () => {
    render(<Alert type="info" message={0} />);

    expect(screen.getByRole("status")).toHaveTextContent("0");
  });

  it("renders an optional action next to the message", () => {
    render(
      <Alert
        type="warning"
        message="Warning message"
        action={
          <button type="button" onClick={() => undefined}>
            Weiter
          </button>
        }
      />,
    );

    const action = screen.getByRole("button", { name: "Weiter" });
    expect(action).toBeInTheDocument();
    // Die Meldung bleibt direktes Kind der getönten Fläche, die Aktion ist ihr
    // Geschwister — sonst brechen die Stil-Assertions unten.
    const alert = screen.getByText("Warning message").parentElement;
    expect(alert?.className).toContain("bg-moto-orange-soft");
    expect(alert?.className).toContain("flex-wrap");
  });

  it("wraps complex actions in a block container", () => {
    render(
      <Alert
        type="success"
        message="Link wurde erstellt."
        action={<div data-testid="complex-action">Aktion</div>}
      />,
    );

    expect(screen.getByTestId("complex-action").parentElement?.tagName).toBe(
      "DIV",
    );
  });

  it("stays a single row without an action", () => {
    render(<Alert type="info" message="Info message" />);

    const alert = screen.getByText("Info message").parentElement;
    expect(alert?.className).not.toContain("flex-wrap");
  });

  it("applies error styles", () => {
    render(<Alert type="error" message="Error message" />);

    const alert = screen.getByText("Error message").parentElement;
    expect(alert?.className).toContain("bg-moto-red-soft");
    expect(alert?.className).toContain("text-moto-red-strong");
  });

  it("applies success styles", () => {
    render(<Alert type="success" message="Success message" />);

    const alert = screen.getByText("Success message").parentElement;
    expect(alert?.className).toContain("text-moto-green-strong");
  });

  it("applies warning styles", () => {
    render(<Alert type="warning" message="Warning message" />);

    const alert = screen.getByText("Warning message").parentElement;
    expect(alert?.className).toContain("bg-moto-orange-soft");
    expect(alert?.className).toContain("text-moto-orange-strong");
  });

  it("applies info styles", () => {
    render(<Alert type="info" message="Info message" />);

    const alert = screen.getByText("Info message").parentElement;
    expect(alert?.className).toContain("bg-moto-blue-soft");
    expect(alert?.className).toContain("text-moto-blue-strong");
  });

  it("renders with icon for error type", () => {
    const { container } = render(<Alert type="error" message="Error" />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("renders with icon for success type", () => {
    const { container } = render(<Alert type="success" message="Success" />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("renders with icon for warning type", () => {
    const { container } = render(<Alert type="warning" message="Warning" />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("renders with icon for info type", () => {
    const { container } = render(<Alert type="info" message="Info" />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  describe("Hinweis-Optionen (#5591)", () => {
    it("closes through a labelled close button", () => {
      const onDismiss = vi.fn();
      render(
        <Alert
          type="warning"
          message="Schulhof hat keine Aufsicht"
          onDismiss={onDismiss}
          dismissLabel="Hinweis zum Schulhof schließen"
        />,
      );

      fireEvent.click(
        screen.getByRole("button", { name: "Hinweis zum Schulhof schließen" }),
      );

      expect(onDismiss).toHaveBeenCalledTimes(1);
    });

    it("shows no close button without onDismiss", () => {
      render(<Alert type="info" message="Nur ein Hinweis" />);

      expect(screen.getByRole("status")).toHaveTextContent("Nur ein Hinweis");
      expect(screen.queryByRole("button")).not.toBeInTheDocument();
    });

    it("renders a custom icon instead of the type icon", () => {
      render(
        <Alert
          type="info"
          message="Von den Eltern gesetzt"
          icon={<svg data-testid="concept-icon" />}
        />,
      );

      const alert = screen.getByRole("status");
      expect(screen.getByTestId("concept-icon")).toBeInTheDocument();
      expect(alert.querySelectorAll("svg")).toHaveLength(1);
    });

    it("renders rich message content such as a list", () => {
      render(
        <Alert
          type="warning"
          announce="off"
          message={
            <ul>
              <li>Erster Hinweis</li>
              <li>Zweiter Hinweis</li>
            </ul>
          }
        />,
      );

      expect(screen.getAllByRole("listitem")).toHaveLength(2);
    });

    it("tints surface and text from a location colour", () => {
      render(
        <Alert
          type="info"
          message="Kommt heute nicht"
          color={LOCATION_COLORS.NOT_ARRIVAL}
        />,
      );

      const tone = getPillTone(LOCATION_COLORS.NOT_ARRIVAL);
      const alert = screen.getByRole("status");
      expect(alert).toHaveStyle({
        backgroundColor: tone.backgroundColor,
        color: tone.textColor,
      });
      expect(alert.className).not.toContain("bg-moto-blue-soft");
    });

    it("keeps the message readable in the compact size", () => {
      render(
        <Alert type="warning" size="compact" message="Kurzer Feldhinweis" />,
      );

      expect(screen.getByRole("alert")).toHaveTextContent("Kurzer Feldhinweis");
    });
  });
});
