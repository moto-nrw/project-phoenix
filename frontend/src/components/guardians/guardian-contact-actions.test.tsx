import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { GuardianContactActions } from "./guardian-contact-actions";

describe("GuardianContactActions", () => {
  const originalLocation = globalThis.location;

  beforeEach(() => {
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
    const { container } = render(<GuardianContactActions />);

    expect(container.firstChild).toBeNull();
  });

  it("renders contact section header", () => {
    render(<GuardianContactActions email="test@example.com" />);

    expect(screen.getByText("Kontakt aufnehmen")).toBeInTheDocument();
  });

  it("renders email button when email provided", () => {
    render(<GuardianContactActions email="test@example.com" />);

    expect(screen.getByText("E-Mail")).toBeInTheDocument();
  });

  it("renders phone button when phone provided", () => {
    render(<GuardianContactActions phone="0123456789" />);

    expect(screen.getByText("Anrufen")).toBeInTheDocument();
  });

  it("renders both buttons when email and phone provided", () => {
    render(
      <GuardianContactActions email="test@example.com" phone="0123456789" />,
    );

    expect(screen.getByText("E-Mail")).toBeInTheDocument();
    expect(screen.getByText("Anrufen")).toBeInTheDocument();
  });

  it("opens mailto link with contact name as subject when email clicked", () => {
    render(
      <GuardianContactActions
        email="test@example.com"
        contactName="Anna Müller"
      />,
    );

    fireEvent.click(screen.getByText("E-Mail"));

    expect(globalThis.location.href).toBe(
      "mailto:test@example.com?subject=Betreff%3A%20Anna%20M%C3%BCller",
    );
  });

  it("opens mailto link with default subject when no contact name", () => {
    render(<GuardianContactActions email="test@example.com" />);

    fireEvent.click(screen.getByText("E-Mail"));

    expect(globalThis.location.href).toBe(
      "mailto:test@example.com?subject=Kontaktanfrage",
    );
  });

  it("opens tel link when phone clicked", () => {
    render(<GuardianContactActions phone="0123 456 789" />);

    fireEvent.click(screen.getByText("Anrufen"));

    expect(globalThis.location.href).toBe("tel:0123456789");
  });

  it("removes spaces from phone number for tel link", () => {
    render(<GuardianContactActions phone="0123 456 789" />);

    fireEvent.click(screen.getByText("Anrufen"));

    expect(globalThis.location.href).toBe("tel:0123456789");
  });

  it("hides email button when email not provided", () => {
    render(<GuardianContactActions phone="0123456789" />);

    // Anker: der Nachbarknopf „Anrufen“ steht da.
    expect(screen.getByText("Anrufen")).toBeInTheDocument();
    expect(screen.queryByText("E-Mail")).not.toBeInTheDocument();
  });

  it("hides phone button when phone not provided", () => {
    render(<GuardianContactActions email="test@example.com" />);

    // Anker: der Nachbarknopf „E-Mail“ steht da.
    expect(screen.getByText("E-Mail")).toBeInTheDocument();
    expect(screen.queryByText("Anrufen")).not.toBeInTheDocument();
  });

  it("has lucide icons in buttons", () => {
    const { container } = render(
      <GuardianContactActions email="test@example.com" phone="0123456789" />,
    );

    const svgs = container.querySelectorAll("svg");
    expect(svgs.length).toBe(2);
  });

  describe("multiple phone numbers", () => {
    const multiplePhones = [
      { number: "+49 170 1234567", label: "Mobil", isPrimary: true },
      { number: "+49 221 9876543", label: "Telefon", isPrimary: false },
      { number: "+49 170 5555555", label: "Dienstlich", isPrimary: false },
    ];

    it("renders phone button with single phoneNumber entry", () => {
      render(
        <GuardianContactActions
          phoneNumbers={[{ number: "0123456789", label: "Mobil" }]}
        />,
      );

      expect(screen.getByText("Anrufen")).toBeInTheDocument();
    });

    it("renders dropdown button when multiple phone numbers provided", () => {
      render(<GuardianContactActions phoneNumbers={multiplePhones} />);

      const button = screen.getByText("Anrufen");
      expect(button).toBeInTheDocument();
      expect(
        button.parentElement?.querySelector("svg.lucide-chevron-down"),
      ).toBeInTheDocument();
    });

    it("opens dropdown when clicked with multiple phones", () => {
      render(<GuardianContactActions phoneNumbers={multiplePhones} />);

      fireEvent.click(screen.getByText("Anrufen"));

      expect(screen.getByText("Mobil · Primär")).toBeInTheDocument();
      expect(screen.getByText("Telefon")).toBeInTheDocument();
      expect(screen.getByText("Dienstlich")).toBeInTheDocument();
    });

    it("shows phone numbers in dropdown", () => {
      render(<GuardianContactActions phoneNumbers={multiplePhones} />);

      fireEvent.click(screen.getByText("Anrufen"));

      expect(screen.getByText("+49 170 1234567")).toBeInTheDocument();
      expect(screen.getByText("+49 221 9876543")).toBeInTheDocument();
      expect(screen.getByText("+49 170 5555555")).toBeInTheDocument();
    });

    it("shows primary badge for primary phone", () => {
      render(<GuardianContactActions phoneNumbers={multiplePhones} />);

      fireEvent.click(screen.getByText("Anrufen"));

      expect(screen.getByText("Mobil · Primär")).toBeInTheDocument();
      expect(screen.queryByText("Telefon · Primär")).not.toBeInTheDocument();
    });

    it("opens tel link when dropdown item clicked", () => {
      render(<GuardianContactActions phoneNumbers={multiplePhones} />);

      fireEvent.click(screen.getByText("Anrufen"));
      fireEvent.click(screen.getByText("+49 221 9876543"));

      expect(globalThis.location.href).toBe("tel:+492219876543");
    });

    it("closes dropdown after selecting a phone", () => {
      render(<GuardianContactActions phoneNumbers={multiplePhones} />);

      fireEvent.click(screen.getByText("Anrufen"));
      expect(screen.getByText("+49 170 1234567")).toBeInTheDocument();

      fireEvent.click(screen.getByText("+49 221 9876543"));

      expect(screen.queryByText("+49 170 1234567")).not.toBeInTheDocument();
    });

    it("does not show dropdown for single phone number", () => {
      render(
        <GuardianContactActions
          phoneNumbers={[{ number: "0123456789", label: "Mobil" }]}
        />,
      );

      const button = screen.getByText("Anrufen");
      expect(
        button.parentElement?.querySelector("svg.lucide-chevron-down"),
      ).toBeNull();
    });

    it("calls phone directly for single phoneNumber entry", () => {
      render(
        <GuardianContactActions
          phoneNumbers={[{ number: "0123 456 789", label: "Mobil" }]}
        />,
      );

      fireEvent.click(screen.getByText("Anrufen"));

      expect(globalThis.location.href).toBe("tel:0123456789");
    });
  });

  it("works with the keyboard", () => {
    render(
      <GuardianContactActions
        phoneNumbers={[
          { number: "0170 111", label: "Mobil", isPrimary: true },
          { number: "0211 222", label: "Arbeit" },
        ]}
      />,
    );
    const trigger = screen.getByRole("button", { name: "Anrufen" });
    // Enter und Leertaste lösen einen Klick ohne Zeiger aus (detail 0).
    fireEvent.click(trigger, { detail: 0 });

    const entries = screen.getAllByRole("menuitem");
    expect(entries[0]).toHaveFocus();
    fireEvent.keyDown(screen.getByRole("menu"), { key: "End" });
    expect(entries.at(-1)).toHaveFocus();
    fireEvent.keyDown(screen.getByRole("menu"), { key: "Home" });
    expect(entries[0]).toHaveFocus();
    fireEvent.keyDown(document, { key: "Escape" });

    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });
});
