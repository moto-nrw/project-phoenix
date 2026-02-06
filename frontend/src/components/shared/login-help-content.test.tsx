import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { LoginHelpContent } from "./login-help-content";

describe("LoginHelpContent", () => {
  it("renders account type information", () => {
    render(
      <LoginHelpContent
        accountType="Betreuer-Account"
        emailLabel="Ihre Dienst-E-Mail"
        passwordLabel="Ihr persönliches Passwort"
      />,
    );

    expect(screen.getByText(/Betreuer-Account/)).toBeInTheDocument();
  });

  it("renders email label", () => {
    render(
      <LoginHelpContent
        accountType="Admin-Account"
        emailLabel="admin@example.com"
        passwordLabel="Passwort"
      />,
    );

    expect(screen.getByText(/admin@example\.com/)).toBeInTheDocument();
  });

  it("renders password label", () => {
    render(
      <LoginHelpContent
        accountType="Betreuer-Account"
        emailLabel="E-Mail"
        passwordLabel="Ihr Initialpasswort"
      />,
    );

    expect(screen.getByText(/Ihr Initialpasswort/)).toBeInTheDocument();
  });

  it("displays all troubleshooting steps", () => {
    render(
      <LoginHelpContent
        accountType="Test Account"
        emailLabel="test@example.com"
        passwordLabel="password123"
      />,
    );

    expect(screen.getByText(/Internetverbindung/)).toBeInTheDocument();
    expect(screen.getByText(/Caps Lock/)).toBeInTheDocument();
    expect(screen.getByText(/Support/)).toBeInTheDocument();
  });

  it("displays heading for login problems", () => {
    render(
      <LoginHelpContent
        accountType="Account"
        emailLabel="email"
        passwordLabel="password"
      />,
    );

    expect(screen.getByText(/Probleme beim Anmelden?/)).toBeInTheDocument();
  });

  it("renders login instruction text", () => {
    render(
      <LoginHelpContent
        accountType="Betreuer-Account"
        emailLabel="E-Mail"
        passwordLabel="Passwort"
      />,
    );

    expect(screen.getByText(/Melden Sie sich mit Ihrem/)).toBeInTheDocument();
  });

  it("displays E-Mail field label", () => {
    render(
      <LoginHelpContent
        accountType="Account"
        emailLabel="user@company.com"
        passwordLabel="pwd"
      />,
    );

    const emailLabel = screen.getByText((content, element) => {
      return (element?.textContent ?? "") === "• E-Mail: user@company.com";
    });

    expect(emailLabel).toBeInTheDocument();
  });

  it("displays Password field label", () => {
    render(
      <LoginHelpContent
        accountType="Account"
        emailLabel="email"
        passwordLabel="Your secure password"
      />,
    );

    const passwordLabel = screen.getByText((content, element) => {
      return (
        (element?.textContent ?? "") === "• Passwort: Your secure password"
      );
    });

    expect(passwordLabel).toBeInTheDocument();
  });

  it("renders all troubleshooting list items", () => {
    render(
      <LoginHelpContent
        accountType="Account"
        emailLabel="email"
        passwordLabel="password"
      />,
    );

    const listItems = screen.getAllByRole("listitem");
    // 2 items for email/password + 3 troubleshooting items = 5 total
    expect(listItems).toHaveLength(5);
  });

  it("renders with long account type name", () => {
    render(
      <LoginHelpContent
        accountType="Administrator-Account mit erweiterten Berechtigungen"
        emailLabel="admin@school.edu"
        passwordLabel="SecurePassword123!"
      />,
    );

    expect(
      screen.getByText(/Administrator-Account mit erweiterten Berechtigungen/),
    ).toBeInTheDocument();
  });

  it("renders with special characters in labels", () => {
    render(
      <LoginHelpContent
        accountType="Test"
        emailLabel="user+test@example.com"
        passwordLabel="Pass@123#$"
      />,
    );

    expect(screen.getByText(/user\+test@example\.com/)).toBeInTheDocument();
    expect(screen.getByText(/Pass@123#\$/)).toBeInTheDocument();
  });

  it("maintains proper semantic structure with strong tags", () => {
    const { container } = render(
      <LoginHelpContent
        accountType="Account"
        emailLabel="email"
        passwordLabel="password"
      />,
    );

    const strongElements = container.querySelectorAll("strong");
    expect(strongElements.length).toBeGreaterThan(0);
  });

  it("renders lists with proper spacing classes", () => {
    const { container } = render(
      <LoginHelpContent
        accountType="Account"
        emailLabel="email"
        passwordLabel="password"
      />,
    );

    const lists = container.querySelectorAll("ul");
    expect(lists[0]).toHaveClass("mt-3", "space-y-2");
    expect(lists[1]).toHaveClass("mt-2", "space-y-1", "text-sm");
  });

  it("displays paragraph with proper spacing", () => {
    const { container } = render(
      <LoginHelpContent
        accountType="Account"
        emailLabel="email"
        passwordLabel="password"
      />,
    );

    const paragraph = container.querySelector("p.mt-4");
    expect(paragraph).toBeInTheDocument();
    expect(paragraph).toHaveTextContent("Probleme beim Anmelden?");
  });
});
