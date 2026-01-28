import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, beforeEach, afterEach } from "vitest";
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

  // Tests for multiple phone numbers (dropdown)
  describe("multiple phone numbers", () => {
    const multiplePhones = [
      { number: "+49 170 1234567", label: "Mobil", isPrimary: true },
      { number: "+49 221 9876543", label: "Telefon", isPrimary: false },
      { number: "+49 170 5555555", label: "Dienstlich", isPrimary: false },
    ];

    it("renders phone button with single phoneNumber entry", () => {
      render(
        <ModernContactActions
          phoneNumbers={[{ number: "0123456789", label: "Mobil" }]}
        />,
      );

      expect(screen.getByText("Anrufen")).toBeInTheDocument();
    });

    it("renders dropdown button when multiple phone numbers provided", () => {
      render(<ModernContactActions phoneNumbers={multiplePhones} />);

      const button = screen.getByText("Anrufen");
      expect(button).toBeInTheDocument();
      // Should have chevron icon for dropdown
      expect(
        button.parentElement?.querySelector("svg.lucide-chevron-down"),
      ).toBeInTheDocument();
    });

    it("opens dropdown when clicked with multiple phones", () => {
      render(<ModernContactActions phoneNumbers={multiplePhones} />);

      fireEvent.click(screen.getByText("Anrufen"));

      // Should show all phone numbers
      expect(screen.getByText("Mobil")).toBeInTheDocument();
      expect(screen.getByText("Telefon")).toBeInTheDocument();
      expect(screen.getByText("Dienstlich")).toBeInTheDocument();
    });

    it("shows phone numbers in dropdown", () => {
      render(<ModernContactActions phoneNumbers={multiplePhones} />);

      fireEvent.click(screen.getByText("Anrufen"));

      expect(screen.getByText("+49 170 1234567")).toBeInTheDocument();
      expect(screen.getByText("+49 221 9876543")).toBeInTheDocument();
      expect(screen.getByText("+49 170 5555555")).toBeInTheDocument();
    });

    it("shows primary badge for primary phone", () => {
      render(<ModernContactActions phoneNumbers={multiplePhones} />);

      fireEvent.click(screen.getByText("Anrufen"));

      expect(screen.getByText("Primär")).toBeInTheDocument();
    });

    it("opens tel link when dropdown item clicked", () => {
      render(<ModernContactActions phoneNumbers={multiplePhones} />);

      fireEvent.click(screen.getByText("Anrufen"));
      fireEvent.click(screen.getByText("+49 221 9876543"));

      expect(globalThis.location.href).toBe("tel:+492219876543");
    });

    it("closes dropdown after selecting a phone", () => {
      render(<ModernContactActions phoneNumbers={multiplePhones} />);

      fireEvent.click(screen.getByText("Anrufen"));
      expect(screen.getByText("+49 170 1234567")).toBeInTheDocument();

      fireEvent.click(screen.getByText("+49 221 9876543"));

      // Dropdown should be closed - check that the phone numbers are not visible
      expect(screen.queryByText("+49 170 1234567")).not.toBeInTheDocument();
    });

    it("does not show dropdown for single phone number", () => {
      render(
        <ModernContactActions
          phoneNumbers={[{ number: "0123456789", label: "Mobil" }]}
        />,
      );

      const button = screen.getByText("Anrufen");
      // Should NOT have chevron icon
      expect(
        button.parentElement?.querySelector("svg.lucide-chevron-down"),
      ).toBeNull();
    });

    it("calls phone directly for single phoneNumber entry", () => {
      render(
        <ModernContactActions
          phoneNumbers={[{ number: "0123 456 789", label: "Mobil" }]}
        />,
      );

      fireEvent.click(screen.getByText("Anrufen"));

      expect(globalThis.location.href).toBe("tel:0123456789");
    });
  });
});
