import { describe, it, expect } from "vitest";
import {
  mapOrganization,
  mapSchool,
  mapInvitation,
  generateSlug,
  validateSlug,
  validateSubdomain,
  isSlugConflict,
  isSubdomainConflict,
  isEmailConflict,
  SLUG_REGEX,
  DELIVERY_STATUS_LABELS,
  DELIVERY_STATUS_STYLES,
} from "./provisioning-helpers";
import type {
  BackendOrganization,
  BackendSchool,
  BackendInvitation,
} from "./provisioning-helpers";

describe("provisioning-helpers", () => {
  describe("mapOrganization", () => {
    const backend: BackendOrganization = {
      id: 42,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-02T00:00:00Z",
      name: "Test Org",
      slug: "test-org",
      active: true,
      settings: '{"key":"value"}',
    };

    it("maps all fields with id as string", () => {
      const result = mapOrganization(backend);
      expect(result).toEqual({
        id: "42",
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-02T00:00:00Z",
        name: "Test Org",
        slug: "test-org",
        active: true,
      });
    });

    it("handles inactive organization", () => {
      const result = mapOrganization({ ...backend, active: false });
      expect(result.active).toBe(false);
    });
  });

  describe("mapSchool", () => {
    const backendSchool: BackendSchool = {
      id: 7,
      created_at: "2026-02-01T00:00:00Z",
      updated_at: "2026-02-02T00:00:00Z",
      organization_id: 42,
      name: "GGS Musterberg",
      slug: "ggs-musterberg",
      subdomain: "ggs-musterberg",
      active: true,
      address: "Musterstraße 1",
      city: "Köln",
      zip: "50667",
      phone: "0221 12345",
      email: "info@schule.de",
    };

    it("maps all fields including optional ones", () => {
      const result = mapSchool(backendSchool);
      expect(result).toEqual({
        id: "7",
        createdAt: "2026-02-01T00:00:00Z",
        updatedAt: "2026-02-02T00:00:00Z",
        organizationId: "42",
        name: "GGS Musterberg",
        slug: "ggs-musterberg",
        subdomain: "ggs-musterberg",
        active: true,
        address: "Musterstraße 1",
        city: "Köln",
        zip: "50667",
        phone: "0221 12345",
        email: "info@schule.de",
        organization: undefined,
      });
    });

    it("maps nested organization when present", () => {
      const withOrg: BackendSchool = {
        ...backendSchool,
        organization: {
          id: 42,
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-02T00:00:00Z",
          name: "Parent Org",
          slug: "parent-org",
          active: true,
        },
      };
      const result = mapSchool(withOrg);
      expect(result.organization).toEqual({
        id: "42",
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-02T00:00:00Z",
        name: "Parent Org",
        slug: "parent-org",
        active: true,
      });
    });

    it("leaves optional fields undefined when absent", () => {
      const minimal: BackendSchool = {
        id: 1,
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
        organization_id: 1,
        name: "School",
        slug: "school",
        subdomain: "school",
        active: true,
      };
      const result = mapSchool(minimal);
      expect(result.address).toBeUndefined();
      expect(result.city).toBeUndefined();
      expect(result.zip).toBeUndefined();
      expect(result.phone).toBeUndefined();
      expect(result.email).toBeUndefined();
      expect(result.organization).toBeUndefined();
    });
  });

  describe("mapInvitation", () => {
    const backend: BackendInvitation = {
      id: 99,
      email: "admin@school.de",
      role_id: 5,
      role_name: "School Admin",
      expires_at: "2026-03-01T00:00:00Z",
      first_name: "Max",
      last_name: "Mustermann",
      position: "Schulleitung",
      created_by: 1,
      creator: "Operator",
      delivery_status: "sent",
      email_sent_at: "2026-02-15T10:00:00Z",
      email_error: undefined,
      email_retry_count: 0,
    };

    it("maps all fields with ids as strings", () => {
      const result = mapInvitation(backend);
      expect(result).toEqual({
        id: "99",
        email: "admin@school.de",
        roleId: "5",
        roleName: "School Admin",
        expiresAt: "2026-03-01T00:00:00Z",
        firstName: "Max",
        lastName: "Mustermann",
        position: "Schulleitung",
        createdBy: "1",
        creator: "Operator",
        deliveryStatus: "sent",
        emailSentAt: "2026-02-15T10:00:00Z",
        emailError: undefined,
        emailRetryCount: 0,
      });
    });

    it("handles missing optional fields", () => {
      const minimal: BackendInvitation = {
        id: 1,
        email: "test@test.de",
        role_id: 1,
        expires_at: "2026-01-01T00:00:00Z",
        created_by: 1,
        delivery_status: "pending",
        email_retry_count: 2,
      };
      const result = mapInvitation(minimal);
      expect(result.roleName).toBeUndefined();
      expect(result.firstName).toBeUndefined();
      expect(result.lastName).toBeUndefined();
      expect(result.position).toBeUndefined();
      expect(result.creator).toBeUndefined();
      expect(result.emailSentAt).toBeUndefined();
      expect(result.emailError).toBeUndefined();
      expect(result.emailRetryCount).toBe(2);
    });
  });

  describe("generateSlug", () => {
    it("lowercases and replaces spaces with hyphens", () => {
      expect(generateSlug("Test School")).toBe("test-school");
    });

    it("replaces German umlauts", () => {
      expect(generateSlug("Köln")).toBe("koeln");
      expect(generateSlug("Müller")).toBe("mueller");
      expect(generateSlug("Über")).toBe("ueber");
      expect(generateSlug("Straße")).toBe("strasse");
    });

    it("replaces uppercase umlauts", () => {
      expect(generateSlug("ÄRGER")).toBe("aerger");
      expect(generateSlug("ÖLFELD")).toBe("oelfeld");
      expect(generateSlug("ÜBER")).toBe("ueber");
    });

    it("removes special characters", () => {
      expect(generateSlug("Test & School!")).toBe("test-school");
    });

    it("trims leading/trailing hyphens", () => {
      expect(generateSlug("  Test  ")).toBe("test");
      expect(generateSlug("---Test---")).toBe("test");
    });

    it("truncates to 100 characters", () => {
      const long = "a".repeat(150);
      expect(generateSlug(long).length).toBe(100);
    });

    it("collapses consecutive hyphens", () => {
      expect(generateSlug("Test  --  School")).toBe("test-school");
    });
  });

  describe("validateSlug", () => {
    it("returns null for valid slug", () => {
      expect(validateSlug("valid-slug")).toBeNull();
      expect(validateSlug("abc123")).toBeNull();
      expect(validateSlug("a")).toBeNull();
    });

    it("returns error for empty slug", () => {
      expect(validateSlug("")).toBe("Slug ist erforderlich");
    });

    it("returns error for slug exceeding 100 chars", () => {
      expect(validateSlug("a".repeat(101))).toBe(
        "Slug darf maximal 100 Zeichen haben",
      );
    });

    it("returns error for invalid characters", () => {
      expect(validateSlug("UPPER")).toBe(
        "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt",
      );
      expect(validateSlug("with space")).toBe(
        "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt",
      );
      expect(validateSlug("special!char")).toBe(
        "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt",
      );
    });

    it("returns error for leading/trailing hyphens", () => {
      expect(validateSlug("-leading")).toBe(
        "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt",
      );
      expect(validateSlug("trailing-")).toBe(
        "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt",
      );
    });

    it("returns error for consecutive hyphens", () => {
      expect(validateSlug("double--hyphen")).toBe(
        "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt",
      );
    });
  });

  describe("validateSubdomain", () => {
    it("returns null for valid subdomain", () => {
      expect(validateSubdomain("valid-subdomain")).toBeNull();
    });

    it("returns error for empty subdomain", () => {
      expect(validateSubdomain("")).toBe("Subdomain ist erforderlich");
    });

    it("returns error for subdomain exceeding 63 chars", () => {
      expect(validateSubdomain("a".repeat(64))).toBe(
        "Subdomain darf maximal 63 Zeichen haben",
      );
    });

    it("accepts subdomain at exactly 63 chars", () => {
      expect(validateSubdomain("a".repeat(63))).toBeNull();
    });

    it("returns error for invalid characters", () => {
      expect(validateSubdomain("UPPER")).toBe(
        "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt",
      );
    });
  });

  describe("conflict matchers", () => {
    describe("isSlugConflict", () => {
      it("returns true when message contains 'slug'", () => {
        expect(isSlugConflict("slug already exists")).toBe(true);
        expect(isSlugConflict("Duplicate SLUG value")).toBe(true);
      });

      it("returns false when message does not contain 'slug'", () => {
        expect(isSlugConflict("email already exists")).toBe(false);
      });
    });

    describe("isSubdomainConflict", () => {
      it("returns true when message contains 'subdomain'", () => {
        expect(isSubdomainConflict("subdomain already taken")).toBe(true);
        expect(isSubdomainConflict("Duplicate SUBDOMAIN")).toBe(true);
      });

      it("returns false when message does not contain 'subdomain'", () => {
        expect(isSubdomainConflict("slug already exists")).toBe(false);
      });
    });

    describe("isEmailConflict", () => {
      it("returns true when message contains 'email'", () => {
        expect(isEmailConflict("email already invited")).toBe(true);
        expect(isEmailConflict("EMAIL conflict")).toBe(true);
      });

      it("returns false when message does not contain 'email'", () => {
        expect(isEmailConflict("slug conflict")).toBe(false);
      });
    });
  });

  describe("SLUG_REGEX", () => {
    it("matches valid slugs", () => {
      expect(SLUG_REGEX.test("abc")).toBe(true);
      expect(SLUG_REGEX.test("abc-def")).toBe(true);
      expect(SLUG_REGEX.test("abc123")).toBe(true);
      expect(SLUG_REGEX.test("a1-b2-c3")).toBe(true);
    });

    it("rejects invalid slugs", () => {
      expect(SLUG_REGEX.test("")).toBe(false);
      expect(SLUG_REGEX.test("-abc")).toBe(false);
      expect(SLUG_REGEX.test("abc-")).toBe(false);
      expect(SLUG_REGEX.test("abc--def")).toBe(false);
      expect(SLUG_REGEX.test("ABC")).toBe(false);
    });
  });

  describe("DELIVERY_STATUS_LABELS", () => {
    it("has labels for all statuses", () => {
      expect(DELIVERY_STATUS_LABELS.sent).toBe("Gesendet");
      expect(DELIVERY_STATUS_LABELS.failed).toBe("Fehlgeschlagen");
      expect(DELIVERY_STATUS_LABELS.pending).toBe("Ausstehend");
    });
  });

  describe("DELIVERY_STATUS_STYLES", () => {
    it("has styles for all statuses", () => {
      expect(DELIVERY_STATUS_STYLES.sent).toContain("green");
      expect(DELIVERY_STATUS_STYLES.failed).toContain("red");
      expect(DELIVERY_STATUS_STYLES.pending).toContain("yellow");
    });
  });
});
