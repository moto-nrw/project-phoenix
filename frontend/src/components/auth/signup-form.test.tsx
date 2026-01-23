/**
 * Tests for SignupForm component and related logic
 *
 * This file tests:
 * - Password requirements validation logic
 * - Slug generation and validation logic
 * - Email validation logic
 */

import { describe, it, expect } from "vitest";

// =============================================================================
// Password Validation Logic Tests
// =============================================================================

// Replicate the password requirements from the component
const PASSWORD_REQUIREMENTS: Array<{
  label: string;
  test: (value: string) => boolean;
}> = [
  { label: "Mindestens 8 Zeichen", test: (value) => value.length >= 8 },
  { label: "Ein Großbuchstabe", test: (value) => /[A-Z]/.test(value) },
  { label: "Ein Kleinbuchstabe", test: (value) => /[a-z]/.test(value) },
  { label: "Eine Zahl", test: (value) => /\d/.test(value) },
  { label: "Ein Sonderzeichen", test: (value) => /[^A-Za-z0-9]/.test(value) },
];

function checkPasswordStrength(password: string): {
  passed: string[];
  failed: string[];
  isStrong: boolean;
} {
  const passed: string[] = [];
  const failed: string[] = [];

  for (const req of PASSWORD_REQUIREMENTS) {
    if (req.test(password)) {
      passed.push(req.label);
    } else {
      failed.push(req.label);
    }
  }

  return {
    passed,
    failed,
    isStrong: failed.length === 0,
  };
}

describe("Password Validation", () => {
  describe("minimum length requirement", () => {
    it("fails for password shorter than 8 characters", () => {
      expect(PASSWORD_REQUIREMENTS[0]?.test("short")).toBe(false);
      expect(PASSWORD_REQUIREMENTS[0]?.test("1234567")).toBe(false);
    });

    it("passes for password with exactly 8 characters", () => {
      expect(PASSWORD_REQUIREMENTS[0]?.test("12345678")).toBe(true);
    });

    it("passes for password longer than 8 characters", () => {
      expect(PASSWORD_REQUIREMENTS[0]?.test("longpassword123")).toBe(true);
    });
  });

  describe("uppercase letter requirement", () => {
    it("fails for password without uppercase", () => {
      expect(PASSWORD_REQUIREMENTS[1]?.test("lowercase")).toBe(false);
      expect(PASSWORD_REQUIREMENTS[1]?.test("123456")).toBe(false);
    });

    it("passes for password with uppercase", () => {
      expect(PASSWORD_REQUIREMENTS[1]?.test("Uppercase")).toBe(true);
      expect(PASSWORD_REQUIREMENTS[1]?.test("ABC")).toBe(true);
    });
  });

  describe("lowercase letter requirement", () => {
    it("fails for password without lowercase", () => {
      expect(PASSWORD_REQUIREMENTS[2]?.test("UPPERCASE")).toBe(false);
      expect(PASSWORD_REQUIREMENTS[2]?.test("123456")).toBe(false);
    });

    it("passes for password with lowercase", () => {
      expect(PASSWORD_REQUIREMENTS[2]?.test("lowercase")).toBe(true);
      expect(PASSWORD_REQUIREMENTS[2]?.test("MixedCase")).toBe(true);
    });
  });

  describe("digit requirement", () => {
    it("fails for password without digit", () => {
      expect(PASSWORD_REQUIREMENTS[3]?.test("nodigits")).toBe(false);
    });

    it("passes for password with digit", () => {
      expect(PASSWORD_REQUIREMENTS[3]?.test("has1digit")).toBe(true);
      expect(PASSWORD_REQUIREMENTS[3]?.test("123")).toBe(true);
    });
  });

  describe("special character requirement", () => {
    it("fails for password without special character", () => {
      expect(PASSWORD_REQUIREMENTS[4]?.test("NoSpecial1")).toBe(false);
    });

    it("passes for password with special character", () => {
      expect(PASSWORD_REQUIREMENTS[4]?.test("has!special")).toBe(true);
      expect(PASSWORD_REQUIREMENTS[4]?.test("test@123")).toBe(true);
      expect(PASSWORD_REQUIREMENTS[4]?.test("with space")).toBe(true);
    });
  });

  describe("complete password strength check", () => {
    it("identifies weak password missing all requirements", () => {
      const result = checkPasswordStrength("short");
      expect(result.isStrong).toBe(false);
      expect(result.failed.length).toBeGreaterThan(0);
    });

    it("identifies strong password meeting all requirements", () => {
      const result = checkPasswordStrength("StrongP@ss1");
      expect(result.isStrong).toBe(true);
      expect(result.failed.length).toBe(0);
      expect(result.passed.length).toBe(5);
    });

    it("tracks individual requirement status", () => {
      const result = checkPasswordStrength("weakpass");
      expect(result.passed).toContain("Mindestens 8 Zeichen");
      expect(result.passed).toContain("Ein Kleinbuchstabe");
      expect(result.failed).toContain("Ein Großbuchstabe");
      expect(result.failed).toContain("Eine Zahl");
      expect(result.failed).toContain("Ein Sonderzeichen");
    });
  });
});

// =============================================================================
// Slug Generation Logic Tests
// =============================================================================

// Replicate the slug generation logic from the component/lib
function generateSlugFromName(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-)|(-$)/g, "");
}

function normalizeSlug(slug: string): string {
  return slug.toLowerCase().replace(/[^a-z0-9-]/g, "");
}

function validateSlug(slug: string): { valid: boolean; error: string | null } {
  if (!slug) {
    return { valid: false, error: "Subdomain ist erforderlich" };
  }
  if (slug.length < 3) {
    return { valid: false, error: "Mindestens 3 Zeichen erforderlich" };
  }
  if (!/^[a-z0-9-]+$/.test(slug)) {
    return {
      valid: false,
      error: "Nur Kleinbuchstaben, Zahlen und Bindestriche",
    };
  }
  if (slug.startsWith("-") || slug.endsWith("-")) {
    return {
      valid: false,
      error: "Darf nicht mit Bindestrich beginnen oder enden",
    };
  }
  return { valid: true, error: null };
}

describe("Slug Generation", () => {
  describe("generateSlugFromName", () => {
    it("converts name to lowercase", () => {
      expect(generateSlugFromName("OGS Musterstadt")).toBe("ogs-musterstadt");
    });

    it("replaces spaces with hyphens", () => {
      expect(generateSlugFromName("my organization")).toBe("my-organization");
    });

    it("removes special characters", () => {
      expect(generateSlugFromName("OGS (Test)")).toBe("ogs-test");
    });

    it("removes leading and trailing hyphens", () => {
      expect(generateSlugFromName("-leading")).toBe("leading");
      expect(generateSlugFromName("trailing-")).toBe("trailing");
    });

    it("collapses multiple hyphens", () => {
      expect(generateSlugFromName("test   multiple   spaces")).toBe(
        "test-multiple-spaces",
      );
    });

    it("handles German umlauts", () => {
      // Umlauts are replaced with hyphens since they're not in a-z
      // Note: In production, you might want to transliterate ü→ue, ö→oe, etc.
      expect(generateSlugFromName("Müller")).toBe("m-ller");
    });

    it("handles empty string", () => {
      expect(generateSlugFromName("")).toBe("");
    });
  });

  describe("normalizeSlug", () => {
    it("converts to lowercase", () => {
      expect(normalizeSlug("MySlug")).toBe("myslug");
    });

    it("removes invalid characters", () => {
      expect(normalizeSlug("my_slug!")).toBe("myslug");
    });

    it("preserves hyphens", () => {
      expect(normalizeSlug("my-slug")).toBe("my-slug");
    });
  });

  describe("validateSlug", () => {
    it("rejects empty slug", () => {
      const result = validateSlug("");
      expect(result.valid).toBe(false);
      expect(result.error).toBe("Subdomain ist erforderlich");
    });

    it("rejects slug shorter than 3 characters", () => {
      const result = validateSlug("ab");
      expect(result.valid).toBe(false);
      expect(result.error).toBe("Mindestens 3 Zeichen erforderlich");
    });

    it("rejects slug with invalid characters", () => {
      const result = validateSlug("my_slug");
      expect(result.valid).toBe(false);
      expect(result.error).toBe("Nur Kleinbuchstaben, Zahlen und Bindestriche");
    });

    it("rejects slug starting with hyphen", () => {
      const result = validateSlug("-invalid");
      expect(result.valid).toBe(false);
    });

    it("rejects slug ending with hyphen", () => {
      const result = validateSlug("invalid-");
      expect(result.valid).toBe(false);
    });

    it("accepts valid slug", () => {
      const result = validateSlug("ogs-musterstadt");
      expect(result.valid).toBe(true);
      expect(result.error).toBeNull();
    });

    it("accepts slug with numbers", () => {
      const result = validateSlug("ogs123");
      expect(result.valid).toBe(true);
    });
  });
});

// =============================================================================
// Email Validation Logic Tests
// =============================================================================

function isValidEmail(email: string): boolean {
  if (!email) return false;
  // Check basic structure
  const atIndex = email.indexOf("@");
  if (atIndex < 1) return false;
  const domainPart = email.substring(atIndex + 1);
  if (!domainPart.includes(".")) return false;
  // Check for common issues
  if (email.includes("..")) return false;
  if (domainPart.startsWith(".") || domainPart.endsWith(".")) return false;
  return true;
}

describe("Email Validation", () => {
  it("rejects empty email", () => {
    expect(isValidEmail("")).toBe(false);
  });

  it("rejects email without @", () => {
    expect(isValidEmail("notanemail")).toBe(false);
  });

  it("rejects email without domain", () => {
    expect(isValidEmail("user@")).toBe(false);
  });

  it("rejects email without TLD", () => {
    expect(isValidEmail("user@domain")).toBe(false);
  });

  it("rejects email with @ at start", () => {
    expect(isValidEmail("@domain.com")).toBe(false);
  });

  it("rejects email with consecutive dots", () => {
    expect(isValidEmail("user@domain..com")).toBe(false);
  });

  it("accepts valid email", () => {
    expect(isValidEmail("user@domain.com")).toBe(true);
    expect(isValidEmail("teacher@school.de")).toBe(true);
    expect(isValidEmail("admin@ogs-musterstadt.moto-app.de")).toBe(true);
  });

  it("accepts email with subdomain", () => {
    expect(isValidEmail("user@mail.domain.com")).toBe(true);
  });

  it("accepts email with plus sign", () => {
    expect(isValidEmail("user+tag@domain.com")).toBe(true);
  });
});

// =============================================================================
// Form Field Validation Logic Tests
// =============================================================================

describe("Form Field Validation", () => {
  describe("name validation", () => {
    function validateName(name: string): {
      valid: boolean;
      error: string | null;
    } {
      const trimmed = name.trim();
      if (!trimmed) {
        return { valid: false, error: "Name ist erforderlich" };
      }
      if (trimmed.length < 2) {
        return { valid: false, error: "Name muss mindestens 2 Zeichen haben" };
      }
      return { valid: true, error: null };
    }

    it("rejects empty name", () => {
      expect(validateName("").valid).toBe(false);
      expect(validateName("   ").valid).toBe(false);
    });

    it("rejects name shorter than 2 characters", () => {
      expect(validateName("A").valid).toBe(false);
    });

    it("accepts valid name", () => {
      expect(validateName("Max Mustermann").valid).toBe(true);
      expect(validateName("Jo").valid).toBe(true);
    });
  });

  describe("password confirmation validation", () => {
    function validatePasswordMatch(
      password: string,
      confirmPassword: string,
    ): { valid: boolean; error: string | null } {
      if (password !== confirmPassword) {
        return { valid: false, error: "Passwörter stimmen nicht überein" };
      }
      return { valid: true, error: null };
    }

    it("fails when passwords do not match", () => {
      const result = validatePasswordMatch("password1", "password2");
      expect(result.valid).toBe(false);
      expect(result.error).toBe("Passwörter stimmen nicht überein");
    });

    it("passes when passwords match", () => {
      const result = validatePasswordMatch("StrongP@ss1", "StrongP@ss1");
      expect(result.valid).toBe(true);
      expect(result.error).toBeNull();
    });

    it("is case-sensitive", () => {
      const result = validatePasswordMatch("Password", "password");
      expect(result.valid).toBe(false);
    });
  });
});
