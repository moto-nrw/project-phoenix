import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ModernContactActions } from "./ModernContactActions";

describe("ModernContactActions", () => {
  const originalLocation = globalThis.location;

  beforeEach(() => {
    // Mock location.href
    Object.defineProperty(globalThis, "location", {
      value: {
        href: "",
      },
      writable: true,
    });
  });

  afterEach(() => {
    globalThis.location = originalLocation;
  });

  it("renders nothing when no email or phone", () => {
    const { container } = render(<ModernContactActions />);

    expect(container.firstChild).toBeNull();
  });

  it("renders contact section header", () => {
    render(<ModernContactActions email="test@example.com" />);

    expect(screen.getByText("Kontakt aufnehmen")).toBeInTheDocument();
  });

  it("renders email button when email provided", () => {
    render(<ModernContactActions email="test@example.com" />);

    expect(screen.getByText("E-Mail")).toBeInTheDocument();
  });

  it("renders phone button when phone provided", () => {
    render(<ModernContactActions phone="0123456789" />);

    expect(screen.getByText("Anrufen")).toBeInTheDocument();
  });

  it("renders both buttons when email and phone provided", () => {
    render(
      <ModernContactActions email="test@example.com" phone="0123456789" />,
    );

    expect(screen.getByText("E-Mail")).toBeInTheDocument();
    expect(screen.getByText("Anrufen")).toBeInTheDocument();
  });

  it("opens mailto link with student name as subject when email clicked", () => {
    render(
      <ModernContactActions
        email="test@example.com"
        studentName="Max Mustermann"
      />,
    );

    fireEvent.click(screen.getByText("E-Mail"));

    expect(globalThis.location.href).toBe(
      "mailto:test@example.com?subject=Betreff%3A%20Max%20Mustermann",
    );
  });

  it("opens mailto link with default subject when no student name", () => {
    render(<ModernContactActions email="test@example.com" />);

    fireEvent.click(screen.getByText("E-Mail"));

    expect(globalThis.location.href).toBe(
      "mailto:test@example.com?subject=Kontaktanfrage",
    );
  });

  it("opens tel link when phone clicked", () => {
    render(<ModernContactActions phone="0123 456 789" />);

    fireEvent.click(screen.getByText("Anrufen"));

    expect(globalThis.location.href).toBe("tel:0123456789");
  });

  it("removes spaces from phone number for tel link", () => {
    render(<ModernContactActions phone="0123 456 789" />);

    fireEvent.click(screen.getByText("Anrufen"));

    expect(globalThis.location.href).toBe("tel:0123456789");
  });

  it("hides email button when email not provided", () => {
    render(<ModernContactActions phone="0123456789" />);

    expect(screen.queryByText("E-Mail")).not.toBeInTheDocument();
  });

  it("hides phone button when phone not provided", () => {
    render(<ModernContactActions email="test@example.com" />);

    expect(screen.queryByText("Anrufen")).not.toBeInTheDocument();
  });

  it("has SVG icons in buttons", () => {
    const { container } = render(
      <ModernContactActions email="test@example.com" phone="0123456789" />,
    );

    const svgs = container.querySelectorAll("svg");
    expect(svgs.length).toBe(2);
  });
});
