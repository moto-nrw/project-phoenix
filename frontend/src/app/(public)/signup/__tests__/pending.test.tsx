/**
 * Tests for app/(public)/signup/pending/page.tsx
 *
 * Tests the signup pending page including:
 * - Success message display
 * - Next steps list
 * - Status information
 * - Navigation links
 * - Contact information
 */

import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

// Mock next/image
vi.mock("next/image", () => ({
  default: ({
    src,
    alt,
    ...props
  }: {
    src: string;
    alt: string;
    width?: number;
    height?: number;
    priority?: boolean;
  }) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img src={src} alt={alt} {...props} />
  ),
}));

// Import after mocks
import SignupPendingPage from "../pending/page";

describe("SignupPendingPage", () => {
  // =============================================================================
  // Page Structure Tests
  // =============================================================================

  describe("page structure", () => {
    it("renders without crashing", () => {
      const { container } = render(<SignupPendingPage />);

      expect(container).toBeTruthy();
    });

    it("renders the moto logo", () => {
      render(<SignupPendingPage />);

      const logo = screen.getByAltText("moto Logo");
      expect(logo).toBeInTheDocument();
      expect(logo).toHaveAttribute("src", "/images/moto_transparent.png");
    });
  });

  // =============================================================================
  // Success Message Tests
  // =============================================================================

  describe("success message", () => {
    it("displays success title", () => {
      render(<SignupPendingPage />);

      expect(
        screen.getByText("Registrierung erfolgreich!"),
      ).toBeInTheDocument();
    });

    it("displays thank you message", () => {
      render(<SignupPendingPage />);

      expect(
        screen.getByText("Vielen Dank für deine Registrierung bei moto."),
      ).toBeInTheDocument();
    });

    it("displays success check icon", () => {
      render(<SignupPendingPage />);

      // The success icon is an SVG with a check path
      const svgElements = document.querySelectorAll("svg");
      expect(svgElements.length).toBeGreaterThan(0);
    });
  });

  // =============================================================================
  // Status Information Tests
  // =============================================================================

  describe("status information", () => {
    it("displays pending status headline", () => {
      render(<SignupPendingPage />);

      expect(
        screen.getByText("Deine Organisation wird geprüft"),
      ).toBeInTheDocument();
    });

    it("displays pending status description", () => {
      render(<SignupPendingPage />);

      // Use more specific regex that matches the full status description
      expect(
        screen.getByText(/sobald deine Organisation freigeschaltet wurde/),
      ).toBeInTheDocument();
    });

    it("mentions email notification in status", () => {
      render(<SignupPendingPage />);

      // Multiple elements mention email, verify at least one exists
      const emailMentions = screen.getAllByText(/Du erhältst eine E-Mail/);
      expect(emailMentions.length).toBeGreaterThan(0);
    });
  });

  // =============================================================================
  // Next Steps Tests
  // =============================================================================

  describe("next steps", () => {
    it("displays next steps header", () => {
      render(<SignupPendingPage />);

      expect(
        screen.getByText("Was passiert als nächstes?"),
      ).toBeInTheDocument();
    });

    it("displays step 1: team review", () => {
      render(<SignupPendingPage />);

      expect(
        screen.getByText("Unser Team prüft deine Anfrage"),
      ).toBeInTheDocument();
    });

    it("displays step 2: email notification", () => {
      render(<SignupPendingPage />);

      expect(
        screen.getByText("Du erhältst eine E-Mail mit dem Ergebnis"),
      ).toBeInTheDocument();
    });

    it("displays step 3: login after approval", () => {
      render(<SignupPendingPage />);

      expect(
        screen.getByText("Nach der Freischaltung kannst du dich anmelden"),
      ).toBeInTheDocument();
    });

    it("displays step numbers", () => {
      render(<SignupPendingPage />);

      expect(screen.getByText("1")).toBeInTheDocument();
      expect(screen.getByText("2")).toBeInTheDocument();
      expect(screen.getByText("3")).toBeInTheDocument();
    });
  });

  // =============================================================================
  // Email Information Tests
  // =============================================================================

  describe("email information", () => {
    it("displays email sent notice", () => {
      render(<SignupPendingPage />);

      expect(
        screen.getByText(/Wir haben dir eine Bestätigungsmail geschickt/),
      ).toBeInTheDocument();
    });

    it("mentions spam folder check", () => {
      render(<SignupPendingPage />);

      expect(screen.getByText(/Spam-Ordner/)).toBeInTheDocument();
    });
  });

  // =============================================================================
  // Navigation Tests
  // =============================================================================

  describe("navigation", () => {
    it("displays back to login button", () => {
      render(<SignupPendingPage />);

      const loginLink = screen.getByRole("link", { name: "Zur Anmeldung" });
      expect(loginLink).toBeInTheDocument();
      expect(loginLink).toHaveAttribute("href", "/");
    });
  });

  // =============================================================================
  // Contact Information Tests
  // =============================================================================

  describe("contact information", () => {
    it("displays support email", () => {
      render(<SignupPendingPage />);

      const supportLink = screen.getByRole("link", {
        name: "support@moto.nrw",
      });
      expect(supportLink).toBeInTheDocument();
      expect(supportLink).toHaveAttribute("href", "mailto:support@moto.nrw");
    });

    it("displays contact help text", () => {
      render(<SignupPendingPage />);

      expect(
        screen.getByText(/Bei Fragen kannst du dich jederzeit an/),
      ).toBeInTheDocument();
    });
  });

  // =============================================================================
  // Metadata Export Tests
  // =============================================================================

  describe("metadata", () => {
    it("exports metadata with correct title", async () => {
      // Import the page module to access metadata
      const pageModule = await import("../pending/page");
      expect(pageModule.metadata.title).toBe(
        "Registrierung erfolgreich | moto",
      );
    });

    it("exports metadata with correct description", async () => {
      const pageModule = await import("../pending/page");
      expect(pageModule.metadata.description).toBe(
        "Deine Organisation wird geprüft",
      );
    });
  });
});
