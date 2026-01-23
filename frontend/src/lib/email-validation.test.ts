/**
 * Tests for ReDoS-safe email validation utilities.
 *
 * Covers all validation paths in isValidEmail() and validateEmail() functions.
 */
import { describe, expect, it } from "vitest";

import {
  isValidEmail,
  MAX_EMAIL_LENGTH,
  validateEmail,
} from "./email-validation";

describe("email-validation", () => {
  describe("MAX_EMAIL_LENGTH constant", () => {
    it("should be 254 per RFC 5321", () => {
      expect(MAX_EMAIL_LENGTH).toBe(254);
    });
  });

  describe("isValidEmail", () => {
    describe("empty and whitespace handling", () => {
      it("should return false for empty string", () => {
        expect(isValidEmail("")).toBe(false);
      });

      it("should return false for whitespace-only string", () => {
        expect(isValidEmail("   ")).toBe(false);
        expect(isValidEmail("\t")).toBe(false);
        expect(isValidEmail("\n")).toBe(false);
      });

      it("should trim whitespace and validate content", () => {
        expect(isValidEmail("  test@example.com  ")).toBe(true);
      });
    });

    describe("length validation (RFC 5321)", () => {
      it("should return false for email exceeding 254 characters", () => {
        // Need 255+ chars: local(243) + @(1) + domain(11) = 255
        const longLocal = "a".repeat(243);
        const longEmail = `${longLocal}@example.com`;
        expect(longEmail.length).toBe(255);
        expect(isValidEmail(longEmail)).toBe(false);
      });

      it("should return true for email at exactly 254 characters", () => {
        // local(63) + @ + domain(190) = 254
        const local = "a".repeat(63);
        const domainPart = "b".repeat(186);
        const email254 = `${local}@${domainPart}.com`;
        expect(email254.length).toBe(254);
        expect(isValidEmail(email254)).toBe(true);
      });

      it("should return true for short valid email", () => {
        expect(isValidEmail("a@b.co")).toBe(true);
      });
    });

    describe("space handling", () => {
      it("should return false for email with space in local part", () => {
        expect(isValidEmail("test user@example.com")).toBe(false);
      });

      it("should return false for email with space in domain", () => {
        expect(isValidEmail("test@exam ple.com")).toBe(false);
      });

      it("should return false for email with space after @", () => {
        expect(isValidEmail("test@ example.com")).toBe(false);
      });
    });

    describe("@ symbol validation", () => {
      it("should return false for email without @", () => {
        expect(isValidEmail("testexample.com")).toBe(false);
      });

      it("should return false for email starting with @", () => {
        expect(isValidEmail("@example.com")).toBe(false);
      });

      it("should return false for email ending with @", () => {
        expect(isValidEmail("test@")).toBe(false);
      });

      it("should return false for email with multiple @ symbols", () => {
        expect(isValidEmail("test@example@domain.com")).toBe(false);
        expect(isValidEmail("test@@example.com")).toBe(false);
      });

      it("should return true for email with exactly one @ in valid position", () => {
        expect(isValidEmail("test@example.com")).toBe(true);
      });
    });

    describe("domain validation", () => {
      it("should return false for domain without dot", () => {
        expect(isValidEmail("test@localhost")).toBe(false);
      });

      it("should return false when last dot is at start of domain (single TLD case)", () => {
        // Uses lastIndexOf, so only catches when the LAST dot is at position 0
        // This means test@.com fails (last dot at 0), but test@.example.com passes
        expect(isValidEmail("test@.com")).toBe(false);
      });

      it("should allow domain with leading dot when there are subsequent dots", () => {
        // The validation uses lastIndexOf(".") which finds the LAST dot
        // So test@.example.com has lastDotIndex=8, not 0, so it passes
        // This is intentionally permissive per the function's docstring
        expect(isValidEmail("test@.example.com")).toBe(true);
      });

      it("should return false for domain ending with dot", () => {
        expect(isValidEmail("test@example.")).toBe(false);
      });

      it("should return true for domain with valid dot placement", () => {
        expect(isValidEmail("test@sub.example.com")).toBe(true);
      });
    });

    describe("TLD validation", () => {
      it("should return false for TLD with only 1 character", () => {
        expect(isValidEmail("test@example.a")).toBe(false);
      });

      it("should return true for TLD with 2 characters", () => {
        expect(isValidEmail("test@example.de")).toBe(true);
      });

      it("should return true for TLD with more than 2 characters", () => {
        expect(isValidEmail("test@example.com")).toBe(true);
        expect(isValidEmail("test@example.museum")).toBe(true);
      });
    });

    describe("valid email formats", () => {
      const validEmails = [
        "simple@example.com",
        "very.common@example.com",
        "disposable.style.email.with+symbol@example.com",
        "other.email-with-hyphen@example.com",
        "fully-qualified-domain@example.com",
        "user.name+tag+sorting@example.com",
        "x@example.com",
        "example-indeed@strange-example.com",
        "example@s.example",
        "user@subdomain.example.com",
        "user123@example123.com",
        "first.last@example.co.uk",
      ];

      it.each(validEmails)(
        'should return true for valid email: "%s"',
        (email) => {
          expect(isValidEmail(email)).toBe(true);
        },
      );
    });

    describe("invalid email formats", () => {
      const invalidEmails = [
        { email: "", reason: "empty string" },
        { email: "   ", reason: "whitespace only" },
        { email: "plainaddress", reason: "missing @ and domain" },
        { email: "@missinglocal.com", reason: "missing local part" },
        { email: "missingdomain@", reason: "missing domain" },
        { email: "missing@dot", reason: "domain without dot" },
        { email: "two@@at.com", reason: "consecutive @ symbols" },
        { email: "has space@example.com", reason: "space in local part" },
        { email: "test@has space.com", reason: "space in domain" },
        {
          email: "test@.com",
          reason: "last dot at domain start (no subdomain)",
        },
        { email: "test@example.", reason: "dot at domain end" },
        { email: "test@example.c", reason: "TLD too short (1 char)" },
      ];

      it.each(invalidEmails)(
        'should return false for "$email" ($reason)',
        ({ email }) => {
          expect(isValidEmail(email)).toBe(false);
        },
      );
    });

    describe("edge cases", () => {
      it("should handle email with numbers in all parts", () => {
        expect(isValidEmail("123@456.78")).toBe(true);
      });

      it("should handle email with hyphens in domain", () => {
        expect(isValidEmail("test@my-domain.com")).toBe(true);
      });

      it("should handle email with dots in local part", () => {
        expect(isValidEmail("test.user.name@example.com")).toBe(true);
      });

      it("should handle email with plus sign in local part", () => {
        expect(isValidEmail("test+tag@example.com")).toBe(true);
      });

      it("should handle email with underscores", () => {
        expect(isValidEmail("test_user@example.com")).toBe(true);
      });

      it("should handle international TLDs", () => {
        expect(isValidEmail("user@example.рф")).toBe(true); // Cyrillic TLD
      });
    });
  });

  describe("validateEmail", () => {
    describe("empty and whitespace handling", () => {
      it("should return error for empty string", () => {
        const result = validateEmail("");
        expect(result.valid).toBe(false);
        expect(result.error).toBe("E-Mail-Adresse ist erforderlich");
      });

      it("should return error for whitespace-only string", () => {
        const result = validateEmail("   ");
        expect(result.valid).toBe(false);
        expect(result.error).toBe("E-Mail-Adresse ist erforderlich");
      });
    });

    describe("length validation", () => {
      it("should return error for email exceeding 254 characters", () => {
        // Need 255+ chars: local(243) + @(1) + domain(11) = 255
        const longLocal = "a".repeat(243);
        const longEmail = `${longLocal}@example.com`;
        expect(longEmail.length).toBe(255);
        const result = validateEmail(longEmail);
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          `E-Mail-Adresse darf maximal ${MAX_EMAIL_LENGTH} Zeichen haben`,
        );
      });
    });

    describe("space validation", () => {
      it("should return error for email with spaces", () => {
        const result = validateEmail("test user@example.com");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "E-Mail-Adresse darf keine Leerzeichen enthalten",
        );
      });
    });

    describe("@ symbol validation", () => {
      it("should return error for missing @", () => {
        const result = validateEmail("testexample.com");
        expect(result.valid).toBe(false);
        expect(result.error).toBe("E-Mail-Adresse muss ein @ enthalten");
      });

      it("should return error for @ at start", () => {
        const result = validateEmail("@example.com");
        expect(result.valid).toBe(false);
        expect(result.error).toBe("E-Mail-Adresse darf nicht mit @ beginnen");
      });

      it("should return error for @ at end", () => {
        const result = validateEmail("test@");
        expect(result.valid).toBe(false);
        expect(result.error).toBe("E-Mail-Adresse darf nicht mit @ enden");
      });

      it("should return error for multiple @ symbols", () => {
        const result = validateEmail("test@example@domain.com");
        expect(result.valid).toBe(false);
        expect(result.error).toBe("E-Mail-Adresse darf nur ein @ enthalten");
      });
    });

    describe("domain validation", () => {
      it("should return error for domain without dot", () => {
        const result = validateEmail("test@localhost");
        expect(result.valid).toBe(false);
        expect(result.error).toBe("Ungültige Domain in der E-Mail-Adresse");
      });

      it("should return error when last dot is at start of domain (single TLD case)", () => {
        // Uses lastIndexOf, so only catches when the LAST dot is at position 0
        const result = validateEmail("test@.com");
        expect(result.valid).toBe(false);
        expect(result.error).toBe("Ungültige Domain in der E-Mail-Adresse");
      });

      it("should allow domain with leading dot when there are subsequent dots", () => {
        // Intentionally permissive: test@.example.com has lastDotIndex=8, not 0
        const result = validateEmail("test@.example.com");
        expect(result.valid).toBe(true);
        expect(result.error).toBeUndefined();
      });

      it("should return error for domain ending with dot", () => {
        const result = validateEmail("test@example.");
        expect(result.valid).toBe(false);
        expect(result.error).toBe("Ungültige Domain in der E-Mail-Adresse");
      });
    });

    describe("TLD validation", () => {
      it("should return error for TLD with only 1 character", () => {
        const result = validateEmail("test@example.a");
        expect(result.valid).toBe(false);
        expect(result.error).toBe("Ungültige Domain-Endung");
      });

      it("should accept TLD with 2+ characters", () => {
        const result = validateEmail("test@example.de");
        expect(result.valid).toBe(true);
        expect(result.error).toBeUndefined();
      });
    });

    describe("valid email", () => {
      it("should return valid:true and no error for valid email", () => {
        const result = validateEmail("test@example.com");
        expect(result.valid).toBe(true);
        expect(result.error).toBeUndefined();
      });

      it("should trim whitespace and validate successfully", () => {
        const result = validateEmail("  test@example.com  ");
        expect(result.valid).toBe(true);
        expect(result.error).toBeUndefined();
      });
    });

    describe("error message consistency", () => {
      it("should return German error messages", () => {
        // Verify all error messages are in German
        expect(validateEmail("").error).toContain("E-Mail-Adresse");
        expect(validateEmail("a".repeat(300) + "@test.com").error).toContain(
          "Zeichen",
        );
        expect(validateEmail("test user@test.com").error).toContain(
          "Leerzeichen",
        );
        expect(validateEmail("noat").error).toContain("@");
        expect(validateEmail("@start.com").error).toContain("beginnen");
        expect(validateEmail("end@").error).toContain("enden");
        expect(validateEmail("two@@test.com").error).toContain("nur ein");
        expect(validateEmail("test@nodot").error).toContain("Domain");
        expect(validateEmail("test@example.a").error).toContain(
          "Domain-Endung",
        );
      });
    });
  });

  describe("isValidEmail and validateEmail consistency", () => {
    const testCases = [
      "test@example.com",
      "",
      "invalid",
      "@start.com",
      "end@",
      "test@localhost",
      "test@.com",
      "test@example.",
      "test@example.a",
      "test user@example.com",
    ];

    it.each(testCases)('should have consistent results for "%s"', (email) => {
      const boolResult = isValidEmail(email);
      const objectResult = validateEmail(email);
      expect(objectResult.valid).toBe(boolResult);
    });
  });
});
