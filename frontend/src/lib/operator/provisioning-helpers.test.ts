import { describe, it, expect } from "vitest";
import {
  generateSlug,
  isValidSlug,
  mapOrganization,
  mapSchool,
  mapInvitation,
} from "./provisioning-helpers";
import type {
  BackendOrganization,
  BackendSchool,
  BackendInvitation,
} from "./provisioning-helpers";

describe("generateSlug", () => {
  it("converts name to lowercase kebab-case", () => {
    expect(generateSlug("My Organization")).toBe("my-organization");
  });

  it("handles German umlauts", () => {
    expect(generateSlug("Städtische Schülerbetreuung")).toBe(
      "staedtische-schuelerbetreuung",
    );
    expect(generateSlug("Große Öffnung")).toBe("grosse-oeffnung");
  });

  it("handles ß", () => {
    expect(generateSlug("Straße")).toBe("strasse");
  });

  it("removes special characters", () => {
    expect(generateSlug("School (Main)")).toBe("school-main");
  });

  it("trims leading/trailing hyphens", () => {
    expect(generateSlug("  Test  ")).toBe("test");
    expect(generateSlug("-test-")).toBe("test");
  });

  it("collapses multiple hyphens", () => {
    expect(generateSlug("a   b")).toBe("a-b");
  });

  it("handles empty string", () => {
    expect(generateSlug("")).toBe("");
  });
});

describe("isValidSlug", () => {
  it("accepts valid slugs", () => {
    expect(isValidSlug("my-org")).toBe(true);
    expect(isValidSlug("school1")).toBe(true);
    expect(isValidSlug("a")).toBe(true);
    expect(isValidSlug("abc-123-def")).toBe(true);
  });

  it("rejects invalid slugs", () => {
    expect(isValidSlug("")).toBe(false);
    expect(isValidSlug("-starts-with-hyphen")).toBe(false);
    expect(isValidSlug("ends-with-hyphen-")).toBe(false);
    expect(isValidSlug("UPPERCASE")).toBe(false);
    expect(isValidSlug("has spaces")).toBe(false);
    expect(isValidSlug("special!chars")).toBe(false);
  });
});

describe("mapOrganization", () => {
  it("maps backend organization to frontend format", () => {
    const backend: BackendOrganization = {
      id: 42,
      name: "Test Org",
      slug: "test-org",
      active: true,
      settings: null,
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-02T00:00:00Z",
    };

    const result = mapOrganization(backend);

    expect(result.id).toBe("42");
    expect(result.name).toBe("Test Org");
    expect(result.slug).toBe("test-org");
    expect(result.active).toBe(true);
    expect(result.createdAt).toBe("2025-01-01T00:00:00Z");
    expect(result.updatedAt).toBe("2025-01-02T00:00:00Z");
  });
});

describe("mapSchool", () => {
  it("maps backend school to frontend format", () => {
    const backend: BackendSchool = {
      id: 10,
      organization_id: 42,
      name: "Test School",
      slug: "test-school",
      subdomain: "test",
      address: "Main St 1",
      city: "Berlin",
      zip: "10115",
      phone: "+49123456",
      email: "info@test.de",
      active: true,
      settings: null,
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-02T00:00:00Z",
    };

    const result = mapSchool(backend);

    expect(result.id).toBe("10");
    expect(result.organizationId).toBe("42");
    expect(result.name).toBe("Test School");
    expect(result.subdomain).toBe("test");
    expect(result.address).toBe("Main St 1");
    expect(result.organization).toBeUndefined();
  });

  it("maps nested organization when present", () => {
    const backend: BackendSchool = {
      id: 10,
      organization_id: 42,
      name: "Test School",
      slug: "test-school",
      subdomain: "test",
      address: "",
      city: "",
      zip: "",
      phone: "",
      email: "",
      active: true,
      settings: null,
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-02T00:00:00Z",
      organization: {
        id: 42,
        name: "Parent Org",
        slug: "parent-org",
        active: true,
        settings: null,
        created_at: "2025-01-01T00:00:00Z",
        updated_at: "2025-01-02T00:00:00Z",
      },
    };

    const result = mapSchool(backend);

    expect(result.organization).toBeDefined();
    expect(result.organization?.id).toBe("42");
    expect(result.organization?.name).toBe("Parent Org");
  });
});

describe("mapInvitation", () => {
  it("maps backend invitation to frontend format", () => {
    const backend: BackendInvitation = {
      id: 5,
      email: "admin@test.de",
      role_id: 1,
      role_name: "admin",
      expires_at: "2025-12-31T00:00:00Z",
      first_name: "Max",
      last_name: "Mustermann",
      position: "Schulleiter",
      created_by: 3,
      creator: "operator@test.de",
      delivery_status: "sent",
      email_sent_at: "2025-01-01T00:00:00Z",
      email_error: null,
      email_retry_count: 0,
    };

    const result = mapInvitation(backend);

    expect(result.id).toBe("5");
    expect(result.email).toBe("admin@test.de");
    expect(result.roleId).toBe("1");
    expect(result.roleName).toBe("admin");
    expect(result.firstName).toBe("Max");
    expect(result.lastName).toBe("Mustermann");
    expect(result.position).toBe("Schulleiter");
    expect(result.createdBy).toBe("3");
    expect(result.deliveryStatus).toBe("sent");
    expect(result.emailSentAt).toBe("2025-01-01T00:00:00Z");
    expect(result.emailError).toBeNull();
  });

  it("handles null optional fields", () => {
    const backend: BackendInvitation = {
      id: 5,
      email: "admin@test.de",
      role_id: 1,
      expires_at: "2025-12-31T00:00:00Z",
      created_by: 3,
      delivery_status: "pending",
      email_retry_count: 0,
    };

    const result = mapInvitation(backend);

    expect(result.firstName).toBeNull();
    expect(result.lastName).toBeNull();
    expect(result.position).toBeNull();
    expect(result.roleName).toBe("");
    expect(result.creator).toBe("");
    expect(result.emailSentAt).toBeNull();
    expect(result.emailError).toBeNull();
  });
});
