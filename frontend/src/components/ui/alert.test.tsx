import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
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
});
