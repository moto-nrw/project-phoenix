/**
 * Tests for slug-validation.ts
 *
 * Covers all validation paths:
 * - validateSlug: empty, length, characters, hyphens, reserved slugs
 * - normalizeSlug: lowercase, trim
 * - generateSlugFromName: umlauts, special chars, length limits
 */
import { describe, it, expect } from "vitest";
import {
  validateSlug,
  normalizeSlug,
  generateSlugFromName,
  RESERVED_SLUGS,
} from "./slug-validation";

describe("slug-validation", () => {
  describe("RESERVED_SLUGS", () => {
    it("should export the reserved slugs array", () => {
      expect(RESERVED_SLUGS).toBeDefined();
      expect(Array.isArray(RESERVED_SLUGS)).toBe(true);
      expect(RESERVED_SLUGS.length).toBeGreaterThan(0);
    });

    it("should contain system routes", () => {
      expect(RESERVED_SLUGS).toContain("api");
      expect(RESERVED_SLUGS).toContain("auth");
      expect(RESERVED_SLUGS).toContain("admin");
      expect(RESERVED_SLUGS).toContain("app");
      expect(RESERVED_SLUGS).toContain("www");
    });

    it("should contain infrastructure slugs", () => {
      expect(RESERVED_SLUGS).toContain("staging");
      expect(RESERVED_SLUGS).toContain("demo");
      expect(RESERVED_SLUGS).toContain("test");
      expect(RESERVED_SLUGS).toContain("dev");
      expect(RESERVED_SLUGS).toContain("prod");
      expect(RESERVED_SLUGS).toContain("production");
    });

    it("should contain service slugs", () => {
      expect(RESERVED_SLUGS).toContain("mail");
      expect(RESERVED_SLUGS).toContain("smtp");
      expect(RESERVED_SLUGS).toContain("cdn");
      expect(RESERVED_SLUGS).toContain("static");
      expect(RESERVED_SLUGS).toContain("assets");
    });

    it("should contain common pattern slugs", () => {
      expect(RESERVED_SLUGS).toContain("login");
      expect(RESERVED_SLUGS).toContain("signup");
      expect(RESERVED_SLUGS).toContain("dashboard");
      expect(RESERVED_SLUGS).toContain("settings");
      expect(RESERVED_SLUGS).toContain("support");
    });
  });

  describe("validateSlug", () => {
    describe("empty and null checks", () => {
      it("should reject empty string", () => {
        const result = validateSlug("");
        expect(result.valid).toBe(false);
        expect(result.error).toBe("Subdomain ist erforderlich");
      });

      it("should reject whitespace-only string", () => {
        // After lowercase normalization, "   " is still "   " which has invalid chars
        const result = validateSlug("   ");
        expect(result.valid).toBe(false);
      });
    });

    describe("length validation", () => {
      it("should reject slug with 1 character", () => {
        const result = validateSlug("a");
        expect(result.valid).toBe(false);
        expect(result.error).toBe("Subdomain muss mindestens 3 Zeichen haben");
      });

      it("should reject slug with 2 characters", () => {
        const result = validateSlug("ab");
        expect(result.valid).toBe(false);
        expect(result.error).toBe("Subdomain muss mindestens 3 Zeichen haben");
      });

      it("should accept slug with exactly 3 characters", () => {
        const result = validateSlug("abc");
        expect(result.valid).toBe(true);
        expect(result.error).toBeUndefined();
      });

      it("should accept slug with exactly 30 characters", () => {
        const result = validateSlug("a".repeat(30));
        expect(result.valid).toBe(true);
        expect(result.error).toBeUndefined();
      });

      it("should reject slug with 31 characters", () => {
        const result = validateSlug("a".repeat(31));
        expect(result.valid).toBe(false);
        expect(result.error).toBe("Subdomain darf maximal 30 Zeichen haben");
      });

      it("should reject slug with 50 characters", () => {
        const result = validateSlug("a".repeat(50));
        expect(result.valid).toBe(false);
        expect(result.error).toBe("Subdomain darf maximal 30 Zeichen haben");
      });
    });

    describe("character validation", () => {
      it("should accept lowercase alphanumeric characters", () => {
        const result = validateSlug("abc123xyz");
        expect(result.valid).toBe(true);
      });

      it("should accept slugs with hyphens", () => {
        const result = validateSlug("my-org-name");
        expect(result.valid).toBe(true);
      });

      it("should accept slugs with numbers", () => {
        const result = validateSlug("org2024");
        expect(result.valid).toBe(true);
      });

      it("should convert uppercase to lowercase and accept", () => {
        const result = validateSlug("MyOrgName");
        expect(result.valid).toBe(true);
      });

      it("should reject spaces in slug", () => {
        const result = validateSlug("my org");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Subdomain darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten",
        );
      });

      it("should reject underscores in slug", () => {
        const result = validateSlug("my_org");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Subdomain darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten",
        );
      });

      it("should reject special characters in slug", () => {
        const result = validateSlug("my@org");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Subdomain darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten",
        );
      });

      it("should reject dots in slug", () => {
        const result = validateSlug("my.org");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Subdomain darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten",
        );
      });

      it("should reject German umlauts in slug", () => {
        const result = validateSlug("müller");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Subdomain darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten",
        );
      });

      it("should reject emoji in slug", () => {
        const result = validateSlug("my🎉org");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Subdomain darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten",
        );
      });
    });

    describe("hyphen position validation", () => {
      it("should reject slug starting with hyphen", () => {
        const result = validateSlug("-myorg");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Subdomain darf nicht mit einem Bindestrich beginnen",
        );
      });

      it("should reject slug ending with hyphen", () => {
        const result = validateSlug("myorg-");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Subdomain darf nicht mit einem Bindestrich enden",
        );
      });

      it("should reject slug with only hyphens", () => {
        const result = validateSlug("---");
        expect(result.valid).toBe(false);
        // Starts with hyphen is checked first
        expect(result.error).toBe(
          "Subdomain darf nicht mit einem Bindestrich beginnen",
        );
      });

      it("should reject slug with consecutive hyphens", () => {
        const result = validateSlug("my--org");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Subdomain darf keine aufeinanderfolgenden Bindestriche enthalten",
        );
      });

      it("should reject slug with multiple consecutive hyphens", () => {
        const result = validateSlug("my---org");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Subdomain darf keine aufeinanderfolgenden Bindestriche enthalten",
        );
      });

      it("should accept hyphen in the middle", () => {
        const result = validateSlug("my-org");
        expect(result.valid).toBe(true);
      });

      it("should accept multiple hyphens not consecutive", () => {
        const result = validateSlug("my-great-org");
        expect(result.valid).toBe(true);
      });
    });

    describe("reserved slug validation", () => {
      it("should reject reserved slug 'api'", () => {
        const result = validateSlug("api");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Diese Subdomain ist reserviert und kann nicht verwendet werden",
        );
      });

      it("should reject reserved slug 'auth'", () => {
        const result = validateSlug("auth");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Diese Subdomain ist reserviert und kann nicht verwendet werden",
        );
      });

      it("should reject reserved slug 'admin'", () => {
        const result = validateSlug("admin");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Diese Subdomain ist reserviert und kann nicht verwendet werden",
        );
      });

      it("should reject reserved slug regardless of case", () => {
        const result = validateSlug("API");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Diese Subdomain ist reserviert und kann nicht verwendet werden",
        );
      });

      it("should reject reserved slug 'dashboard'", () => {
        const result = validateSlug("dashboard");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Diese Subdomain ist reserviert und kann nicht verwendet werden",
        );
      });

      it("should reject reserved slug 'login'", () => {
        const result = validateSlug("login");
        expect(result.valid).toBe(false);
        expect(result.error).toBe(
          "Diese Subdomain ist reserviert und kann nicht verwendet werden",
        );
      });

      it("should accept slug that contains reserved word as part", () => {
        const result = validateSlug("my-api-org");
        expect(result.valid).toBe(true);
      });

      it("should accept slug that starts with reserved word", () => {
        const result = validateSlug("api-service");
        expect(result.valid).toBe(true);
      });
    });

    describe("valid slugs", () => {
      it("should accept simple alphanumeric slug", () => {
        const result = validateSlug("myorg");
        expect(result.valid).toBe(true);
        expect(result.error).toBeUndefined();
      });

      it("should accept slug with numbers", () => {
        const result = validateSlug("org2024");
        expect(result.valid).toBe(true);
      });

      it("should accept slug with hyphens", () => {
        const result = validateSlug("my-cool-org");
        expect(result.valid).toBe(true);
      });

      it("should accept numeric-only slug", () => {
        const result = validateSlug("123");
        expect(result.valid).toBe(true);
      });

      it("should accept mixed slug", () => {
        const result = validateSlug("org-2024-test-v1");
        expect(result.valid).toBe(true);
      });
    });
  });

  describe("normalizeSlug", () => {
    it("should convert uppercase to lowercase", () => {
      expect(normalizeSlug("MyOrg")).toBe("myorg");
    });

    it("should convert all uppercase to lowercase", () => {
      expect(normalizeSlug("MYORG")).toBe("myorg");
    });

    it("should trim leading whitespace", () => {
      expect(normalizeSlug("  myorg")).toBe("myorg");
    });

    it("should trim trailing whitespace", () => {
      expect(normalizeSlug("myorg  ")).toBe("myorg");
    });

    it("should trim both leading and trailing whitespace", () => {
      expect(normalizeSlug("  myorg  ")).toBe("myorg");
    });

    it("should handle both uppercase and whitespace", () => {
      expect(normalizeSlug("  MyORG  ")).toBe("myorg");
    });

    it("should preserve hyphens", () => {
      expect(normalizeSlug("My-Org-Name")).toBe("my-org-name");
    });

    it("should handle empty string", () => {
      expect(normalizeSlug("")).toBe("");
    });

    it("should handle whitespace-only string", () => {
      expect(normalizeSlug("   ")).toBe("");
    });

    it("should preserve numbers", () => {
      expect(normalizeSlug("Org2024")).toBe("org2024");
    });
  });

  describe("generateSlugFromName", () => {
    describe("basic transformations", () => {
      it("should convert to lowercase", () => {
        expect(generateSlugFromName("MyOrg")).toBe("myorg");
      });

      it("should replace spaces with hyphens", () => {
        expect(generateSlugFromName("My Org")).toBe("my-org");
      });

      it("should replace multiple spaces with single hyphen", () => {
        expect(generateSlugFromName("My   Great   Org")).toBe("my-great-org");
      });

      it("should trim leading whitespace", () => {
        expect(generateSlugFromName("  My Org")).toBe("my-org");
      });

      it("should trim trailing whitespace", () => {
        expect(generateSlugFromName("My Org  ")).toBe("my-org");
      });
    });

    describe("German umlaut conversion", () => {
      it("should convert lowercase ä to ae", () => {
        expect(generateSlugFromName("bäcker")).toBe("baecker");
      });

      it("should convert uppercase Ä to ae", () => {
        expect(generateSlugFromName("Bäcker")).toBe("baecker");
      });

      it("should convert lowercase ö to oe", () => {
        expect(generateSlugFromName("köln")).toBe("koeln");
      });

      it("should convert uppercase Ö to oe", () => {
        expect(generateSlugFromName("Österreich")).toBe("oesterreich");
      });

      it("should convert lowercase ü to ue", () => {
        expect(generateSlugFromName("münchen")).toBe("muenchen");
      });

      it("should convert uppercase Ü to ue", () => {
        expect(generateSlugFromName("Über")).toBe("ueber");
      });

      it("should convert ß to ss", () => {
        expect(generateSlugFromName("straße")).toBe("strasse");
      });

      it("should handle multiple umlauts", () => {
        expect(generateSlugFromName("Grüße aus München")).toBe(
          "gruesse-aus-muenchen",
        );
      });

      it("should handle name with all umlauts", () => {
        expect(generateSlugFromName("Äöüß")).toBe("aeoeuess");
      });
    });

    describe("special character handling", () => {
      it("should remove special characters", () => {
        expect(generateSlugFromName("My@Org!")).toBe("my-org");
      });

      it("should replace dots with hyphens", () => {
        expect(generateSlugFromName("my.org.name")).toBe("my-org-name");
      });

      it("should replace underscores with hyphens", () => {
        expect(generateSlugFromName("my_org_name")).toBe("my-org-name");
      });

      it("should handle ampersand", () => {
        expect(generateSlugFromName("Tom & Jerry")).toBe("tom-jerry");
      });

      it("should handle parentheses", () => {
        expect(generateSlugFromName("My Org (Old)")).toBe("my-org-old");
      });

      it("should handle quotes", () => {
        expect(generateSlugFromName('My "Great" Org')).toBe("my-great-org");
      });

      it("should remove emoji", () => {
        expect(generateSlugFromName("My🎉Org")).toBe("my-org");
      });
    });

    describe("hyphen normalization", () => {
      it("should remove leading hyphens", () => {
        expect(generateSlugFromName("-My Org")).toBe("my-org");
      });

      it("should remove trailing hyphens", () => {
        expect(generateSlugFromName("My Org-")).toBe("my-org");
      });

      it("should collapse multiple hyphens", () => {
        expect(generateSlugFromName("My--Org")).toBe("my-org");
      });

      it("should handle name that becomes only hyphens after conversion", () => {
        expect(generateSlugFromName("---")).toBe("");
      });

      it("should handle complex case with multiple issues", () => {
        expect(generateSlugFromName("  --My@@Org!!--  ")).toBe("my-org");
      });
    });

    describe("length limiting", () => {
      it("should truncate to 30 characters", () => {
        const longName = "My Very Long Organization Name That Exceeds Limit";
        const result = generateSlugFromName(longName);
        expect(result.length).toBeLessThanOrEqual(30);
      });

      it("should keep exactly 30 characters when name is longer", () => {
        const longName = "a".repeat(50);
        const result = generateSlugFromName(longName);
        expect(result.length).toBe(30);
      });

      it("should not truncate names under 30 characters", () => {
        const shortName = "My Short Org";
        const result = generateSlugFromName(shortName);
        expect(result).toBe("my-short-org");
        expect(result.length).toBeLessThan(30);
      });

      it("should handle truncation of hyphenated name", () => {
        // Create a name that after conversion will be around 30 chars
        const name = "My Organization Name Is Quite Long Here";
        const result = generateSlugFromName(name);
        expect(result.length).toBeLessThanOrEqual(30);
      });
    });

    describe("edge cases", () => {
      it("should handle empty string", () => {
        expect(generateSlugFromName("")).toBe("");
      });

      it("should handle whitespace-only string", () => {
        expect(generateSlugFromName("   ")).toBe("");
      });

      it("should handle special characters only", () => {
        expect(generateSlugFromName("@#$%^&*")).toBe("");
      });

      it("should handle numbers only", () => {
        expect(generateSlugFromName("12345")).toBe("12345");
      });

      it("should preserve numbers in name", () => {
        expect(generateSlugFromName("Org 2024")).toBe("org-2024");
      });

      it("should handle realistic German organization name", () => {
        expect(generateSlugFromName("Grundschule Müller-Straße")).toBe(
          "grundschule-mueller-strasse",
        );
      });

      it("should handle realistic German school name", () => {
        expect(generateSlugFromName("OGS Düsseldorf Süd")).toBe(
          "ogs-duesseldorf-sued",
        );
      });
    });
  });

  describe("integration: generateSlugFromName -> validateSlug", () => {
    it("should generate valid slugs from typical organization names", () => {
      const names = [
        "My Organization",
        "Test School 2024",
        "Grundschule Müller",
        "OGS Düsseldorf",
        "Primary School",
      ];

      for (const name of names) {
        const slug = generateSlugFromName(name);
        const validation = validateSlug(slug);
        expect(
          validation.valid,
          `Expected "${slug}" (from "${name}") to be valid, got error: ${validation.error}`,
        ).toBe(true);
      }
    });

    it("should generate valid slug from name with all umlauts", () => {
      const slug = generateSlugFromName("Äöüß Test");
      const validation = validateSlug(slug);
      expect(validation.valid).toBe(true);
    });

    it("should generate valid slug from name with special characters", () => {
      const slug = generateSlugFromName("Tom & Jerry's School (Main)");
      const validation = validateSlug(slug);
      expect(validation.valid).toBe(true);
    });
  });
});
