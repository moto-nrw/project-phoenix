import { describe, it, expect } from "vitest";
import {
  generateSlug,
  isValidSlug,
  mapOrganization,
  mapSchool,
  mapInvitation,
  mapSchoolAccount,
  mapOrgAccount,
  mapOperatorDevice,
  mapOperatorPerson,
  mapUnregisteredTagScan,
  mapSchoolPWAUsage,
  summaryToOrganization,
  summaryToSchool,
} from "./provisioning-helpers";
import type {
  BackendOrganization,
  BackendSchool,
  BackendInvitation,
  BackendSchoolAccount,
  BackendOrgAccount,
  BackendOperatorDevice,
  BackendOperatorPerson,
  BackendUnregisteredTagScan,
  OrganizationSummary,
  SchoolSummary,
} from "./provisioning-helpers";

describe("generateSlug", () => {
  it("converts name to lowercase kebab-case", () => {
    expect(generateSlug("My Organization")).toBe("my-organization");
  });

  it("handles German umlauts", () => {
    expect(generateSlug("Städtische Kinderbetreuung")).toBe(
      "staedtische-kinderbetreuung",
    );
    expect(generateSlug("Große Öffnung")).toBe("grosse-oeffnung");
  });

  it("handles ß", () => {
    expect(generateSlug("Straße")).toBe("strasse");
  });

  it("handles uppercase umlauts", () => {
    expect(generateSlug("ÄRGER")).toBe("aerger");
    expect(generateSlug("ÖFFNUNG")).toBe("oeffnung");
    expect(generateSlug("ÜBER")).toBe("ueber");
  });

  it("handles multiple umlauts in one string", () => {
    expect(generateSlug("Müller-Straße")).toBe("mueller-strasse");
  });

  it("handles complex German organization names", () => {
    expect(generateSlug("Förderverein Grundschule Köln-Süd e.V.")).toBe(
      "foerderverein-grundschule-koeln-sued-e-v",
    );
  });

  it("handles string with only special characters", () => {
    expect(generateSlug("!@#$%")).toBe("");
  });

  it("handles string with only spaces", () => {
    expect(generateSlug("   ")).toBe("");
  });

  it("passes through already valid slug", () => {
    expect(generateSlug("already-valid")).toBe("already-valid");
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

  it("rejects underscores and dots", () => {
    expect(isValidSlug("my_slug")).toBe(false);
    expect(isValidSlug("my.slug")).toBe(false);
  });

  it("rejects a single hyphen", () => {
    expect(isValidSlug("-")).toBe(false);
  });

  it("rejects umlauts", () => {
    expect(isValidSlug("müller")).toBe(false);
  });

  it("accepts single character", () => {
    expect(isValidSlug("a")).toBe(true);
    expect(isValidSlug("1")).toBe(true);
  });

  it("accepts slug starting and ending with numbers", () => {
    expect(isValidSlug("1slug1")).toBe(true);
  });
});

describe("mapOrganization", () => {
  it("maps backend organization to frontend format", () => {
    const backend: BackendOrganization = {
      id: 42,
      name: "Test Org",
      slug: "test-org",
      active: true,
      deleted_at: null,
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
      hidden: false,
      deleted_at: null,
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
    expect(result.hidden).toBe(false);
    expect(result.organization).toBeUndefined();
  });

  it("maps hidden: true correctly", () => {
    const backend: BackendSchool = {
      id: 10,
      organization_id: 42,
      name: "Hidden School",
      slug: "hidden-school",
      subdomain: "hidden",
      address: "",
      city: "",
      zip: "",
      phone: "",
      email: "",
      active: true,
      hidden: true,
      deleted_at: null,
      settings: null,
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-02T00:00:00Z",
    };

    const result = mapSchool(backend);

    expect(result.hidden).toBe(true);
  });

  it("maps deleted_at timestamp correctly", () => {
    const backend: BackendSchool = {
      id: 10,
      organization_id: 42,
      name: "Deleted School",
      slug: "deleted-school",
      subdomain: "deleted",
      address: "",
      city: "",
      zip: "",
      phone: "",
      email: "",
      active: false,
      hidden: false,
      deleted_at: "2026-03-15T10:00:00Z",
      settings: null,
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-02T00:00:00Z",
    };

    const result = mapSchool(backend);

    expect(result.deletedAt).toBe("2026-03-15T10:00:00Z");
  });

  it("maps deleted_at: null correctly", () => {
    const backend: BackendSchool = {
      id: 10,
      organization_id: 42,
      name: "Active School",
      slug: "active-school",
      subdomain: "active",
      address: "",
      city: "",
      zip: "",
      phone: "",
      email: "",
      active: true,
      hidden: false,
      deleted_at: null,
      settings: null,
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-02T00:00:00Z",
    };

    const result = mapSchool(backend);

    expect(result.deletedAt).toBeNull();
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
      hidden: false,
      deleted_at: null,
      settings: null,
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-02T00:00:00Z",
      organization: {
        id: 42,
        name: "Parent Org",
        slug: "parent-org",
        active: true,
        deleted_at: null,
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
      caregiver_enabled: true,
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
    expect(result.caregiverEnabled).toBe(true);
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
    expect(result.caregiverEnabled).toBe(false);
    expect(result.roleName).toBe("");
    expect(result.creator).toBe("");
    expect(result.emailSentAt).toBeNull();
    expect(result.emailError).toBeNull();
  });

  it("handles email_error when present", () => {
    const backend: BackendInvitation = {
      id: 6,
      email: "fail@test.de",
      role_id: 1,
      expires_at: "2025-12-31T00:00:00Z",
      created_by: 3,
      delivery_status: "failed",
      email_retry_count: 2,
      email_error: "SMTP connection refused",
    };

    const result = mapInvitation(backend);
    expect(result.emailError).toBe("SMTP connection refused");
    expect(result.emailRetryCount).toBe(2);
    expect(result.deliveryStatus).toBe("failed");
  });
});

describe("summary adapters", () => {
  it("drops organization summary counts for base organization consumers", () => {
    const summary: OrganizationSummary = {
      id: "5",
      name: "Stadt Köln",
      slug: "stadt-koeln",
      active: true,
      deletedAt: null,
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-02T00:00:00Z",
      schulenCount: 3,
      kontenCount: 24,
      geraeteCount: 7,
      personenCount: 100,
    };

    expect(summaryToOrganization(summary)).toEqual({
      id: "5",
      name: "Stadt Köln",
      slug: "stadt-koeln",
      active: true,
      deletedAt: null,
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-02T00:00:00Z",
    });
  });

  it("drops school summary counts for base school consumers", () => {
    const summary: SchoolSummary = {
      id: "10",
      organizationId: "5",
      organizationName: "Stadt Köln",
      name: "GGS Europaschule",
      slug: "ggs-europa",
      subdomain: "ggs-europa",
      address: "Hauptstr. 1",
      city: "Köln",
      zip: "50667",
      phone: "0221123456",
      email: "info@ggs-europa.de",
      active: true,
      hidden: false,
      deletedAt: null,
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-02T00:00:00Z",
      kontenCount: 12,
      geraeteCount: 4,
      personenCount: 80,
    };

    expect(summaryToSchool(summary)).toEqual({
      id: "10",
      organizationId: "5",
      name: "GGS Europaschule",
      slug: "ggs-europa",
      subdomain: "ggs-europa",
      address: "Hauptstr. 1",
      city: "Köln",
      zip: "50667",
      phone: "0221123456",
      email: "info@ggs-europa.de",
      active: true,
      hidden: false,
      deletedAt: null,
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-02T00:00:00Z",
    });
  });
});

describe("mapSchoolAccount", () => {
  it("maps all fields from backend to frontend format", () => {
    const backend: BackendSchoolAccount = {
      account_id: 55,
      email: "teacher@school.de",
      active: true,
      first_name: "Anna",
      last_name: "Schmidt",
      role_name: "Teacher",
      pedagogic_role: "Erzieher",
      status: "active",
    };

    const result = mapSchoolAccount(backend);

    expect(result).toEqual({
      accountId: "55",
      email: "teacher@school.de",
      active: true,
      firstName: "Anna",
      lastName: "Schmidt",
      roleName: "Teacher",
      pedagogicRole: "Erzieher",
      status: "active",
      hasAdminRole: false,
      hasUserRole: false,
      hasCaregiverProfile: false,
      isActiveCaregiver: false,
    });
  });

  it("converts numeric account_id to string", () => {
    const backend: BackendSchoolAccount = {
      account_id: 0,
      email: "",
      active: false,
      first_name: "",
      last_name: "",
      role_name: "",
      pedagogic_role: "",
      status: "inactive",
    };

    expect(mapSchoolAccount(backend).accountId).toBe("0");
  });

  it("preserves boolean active field", () => {
    const backend: BackendSchoolAccount = {
      account_id: 1,
      email: "inactive@school.de",
      active: false,
      first_name: "Max",
      last_name: "Muster",
      role_name: "Teacher",
      pedagogic_role: "",
      status: "inactive",
    };

    expect(mapSchoolAccount(backend).active).toBe(false);
  });
});

describe("mapOrgAccount", () => {
  it("maps all fields including school info", () => {
    const backend: BackendOrgAccount = {
      account_id: 77,
      email: "org-admin@org.de",
      active: true,
      first_name: "Klaus",
      last_name: "Weber",
      role_name: "OrgAdmin",
      pedagogic_role: "",
      status: "active",
      school_id: 10,
      school_name: "Test School",
    };

    const result = mapOrgAccount(backend);

    expect(result).toEqual({
      accountId: "77",
      email: "org-admin@org.de",
      active: true,
      firstName: "Klaus",
      lastName: "Weber",
      roleName: "OrgAdmin",
      pedagogicRole: "",
      status: "active",
      hasAdminRole: false,
      hasUserRole: false,
      hasCaregiverProfile: false,
      isActiveCaregiver: false,
      schoolId: "10",
      schoolName: "Test School",
    });
  });

  it("converts school_id to string", () => {
    const backend: BackendOrgAccount = {
      account_id: 1,
      email: "",
      active: true,
      first_name: "",
      last_name: "",
      role_name: "",
      pedagogic_role: "",
      status: "active",
      school_id: 999,
      school_name: "School 999",
    };

    expect(mapOrgAccount(backend).schoolId).toBe("999");
  });

  it("includes all SchoolAccount fields via spread", () => {
    const backend: BackendOrgAccount = {
      account_id: 88,
      email: "test@org.de",
      active: false,
      first_name: "Test",
      last_name: "User",
      role_name: "Teacher",
      pedagogic_role: "Pädagoge",
      status: "pending",
      school_id: 5,
      school_name: "My School",
    };

    const result = mapOrgAccount(backend);

    expect(result.accountId).toBe("88");
    expect(result.email).toBe("test@org.de");
    expect(result.active).toBe(false);
    expect(result.firstName).toBe("Test");
    expect(result.lastName).toBe("User");
    expect(result.roleName).toBe("Teacher");
    expect(result.pedagogicRole).toBe("Pädagoge");
    expect(result.status).toBe("pending");
    expect(result.schoolId).toBe("5");
    expect(result.schoolName).toBe("My School");
  });
});

describe("mapOperatorDevice", () => {
  const fullBackendDevice: BackendOperatorDevice = {
    id: 200,
    device_id: "dev-001",
    device_type: "raspberry_pi",
    name: "Entrance Reader",
    status: "active",
    api_key: "secret-key-123",
    masked_api_key: "sec***123",
    last_seen: "2026-03-20T15:30:00Z",
    is_online: true,
    school_id: 10,
    school_name: "Test School",
    organization_id: 5,
    organization_name: "Test Org",
    created_at: "2026-01-15T00:00:00Z",
    updated_at: "2026-03-20T15:30:00Z",
  };

  it("maps all fields from backend to frontend format", () => {
    const result = mapOperatorDevice(fullBackendDevice);

    expect(result).toEqual({
      id: "200",
      deviceId: "dev-001",
      deviceType: "raspberry_pi",
      name: "Entrance Reader",
      status: "active",
      apiKey: "secret-key-123",
      maskedApiKey: "sec***123",
      lastSeen: "2026-03-20T15:30:00Z",
      isOnline: true,
      schoolId: "10",
      schoolName: "Test School",
      organizationId: "5",
      organizationName: "Test Org",
      createdAt: "2026-01-15T00:00:00Z",
      updatedAt: "2026-03-20T15:30:00Z",
    });
  });

  it("defaults name to empty string when undefined", () => {
    const backend: BackendOperatorDevice = {
      ...fullBackendDevice,
      name: undefined,
    };

    expect(mapOperatorDevice(backend).name).toBe("");
  });

  it("defaults api_key to empty string when undefined", () => {
    const backend: BackendOperatorDevice = {
      ...fullBackendDevice,
      api_key: undefined,
    };

    expect(mapOperatorDevice(backend).apiKey).toBe("");
  });

  it("defaults last_seen to null when undefined", () => {
    const backend: BackendOperatorDevice = {
      ...fullBackendDevice,
      last_seen: undefined,
    };

    expect(mapOperatorDevice(backend).lastSeen).toBeNull();
  });

  it("handles offline device with missing optional fields", () => {
    const backend: BackendOperatorDevice = {
      ...fullBackendDevice,
      is_online: false,
      last_seen: undefined,
      name: undefined,
      api_key: undefined,
      status: "inactive",
    };

    const result = mapOperatorDevice(backend);
    expect(result.isOnline).toBe(false);
    expect(result.lastSeen).toBeNull();
    expect(result.name).toBe("");
    expect(result.apiKey).toBe("");
    expect(result.status).toBe("inactive");
  });

  it("converts all numeric ids to strings", () => {
    const result = mapOperatorDevice(fullBackendDevice);
    expect(result.id).toBe("200");
    expect(result.schoolId).toBe("10");
    expect(result.organizationId).toBe("5");
  });
});

describe("mapOperatorPerson", () => {
  const fullBackendPerson: BackendOperatorPerson = {
    id: 42,
    first_name: "Max",
    last_name: "Mustermann",
    has_account: true,
    account_email: "max@test.de",
    has_rfid_card: true,
    is_staff: true,
    is_student: false,
    school_id: 10,
    school_name: "Testschule",
    organization_id: 5,
    organization_name: "Testträger",
    created_at: "2026-01-01T00:00:00Z",
  };

  it("maps all fields from backend to frontend format", () => {
    const result = mapOperatorPerson(fullBackendPerson);

    expect(result).toEqual({
      id: "42",
      firstName: "Max",
      lastName: "Mustermann",
      fullName: "Max Mustermann",
      hasAccount: true,
      accountEmail: "max@test.de",
      hasRfidCard: true,
      isStaff: true,
      isStudent: false,
      schoolId: "10",
      schoolName: "Testschule",
      organizationId: "5",
      organizationName: "Testträger",
      createdAt: "2026-01-01T00:00:00Z",
    });
  });

  it("converts all numeric ids to strings", () => {
    const result = mapOperatorPerson(fullBackendPerson);
    expect(result.id).toBe("42");
    expect(result.schoolId).toBe("10");
    expect(result.organizationId).toBe("5");
  });

  it("constructs fullName from first and last name", () => {
    const result = mapOperatorPerson(fullBackendPerson);
    expect(result.fullName).toBe("Max Mustermann");
  });

  it("handles null account_email", () => {
    const backend: BackendOperatorPerson = {
      ...fullBackendPerson,
      account_email: null,
    };

    expect(mapOperatorPerson(backend).accountEmail).toBeNull();
  });

  it("handles undefined account_email", () => {
    const backend: BackendOperatorPerson = {
      ...fullBackendPerson,
      account_email: undefined,
    };

    expect(mapOperatorPerson(backend).accountEmail).toBeNull();
  });

  it("maps student-only person correctly", () => {
    const backend: BackendOperatorPerson = {
      ...fullBackendPerson,
      is_staff: false,
      is_student: true,
      has_account: false,
      account_email: null,
      has_rfid_card: false,
    };

    const result = mapOperatorPerson(backend);
    expect(result.isStaff).toBe(false);
    expect(result.isStudent).toBe(true);
    expect(result.hasAccount).toBe(false);
    expect(result.accountEmail).toBeNull();
    expect(result.hasRfidCard).toBe(false);
  });

  it("preserves boolean fields accurately", () => {
    const backend: BackendOperatorPerson = {
      ...fullBackendPerson,
      has_account: false,
      has_rfid_card: false,
      is_staff: false,
      is_student: false,
    };

    const result = mapOperatorPerson(backend);
    expect(result.hasAccount).toBe(false);
    expect(result.hasRfidCard).toBe(false);
    expect(result.isStaff).toBe(false);
    expect(result.isStudent).toBe(false);
  });
});

describe("mapUnregisteredTagScan", () => {
  const fullBackendScan: BackendUnregisteredTagScan = {
    id: 300,
    tenant_id: 10,
    tag_uid: "04AABBCCDD",
    device_id: 200,
    scanned_at: "2026-05-01T08:00:00Z",
    resolved_at: "2026-05-01T09:00:00Z",
    resolved_by_operator_id: 15,
    resolution_note: "Issued replacement card",
    created_at: "2026-05-01T08:00:00Z",
    updated_at: "2026-05-01T09:00:00Z",
    school_id: 10,
    school_name: "Testschule",
    organization_id: 5,
    organization_name: "Testträger",
    device_identifier: "reader-entrance",
    device_name: "Eingang",
  };

  it("maps all backend fields and converts ids to strings", () => {
    expect(mapUnregisteredTagScan(fullBackendScan)).toEqual({
      id: "300",
      tenantId: "10",
      tagUid: "04AABBCCDD",
      deviceId: "200",
      scannedAt: "2026-05-01T08:00:00Z",
      resolvedAt: "2026-05-01T09:00:00Z",
      resolvedByOperatorId: "15",
      resolutionNote: "Issued replacement card",
      createdAt: "2026-05-01T08:00:00Z",
      updatedAt: "2026-05-01T09:00:00Z",
      schoolId: "10",
      schoolName: "Testschule",
      organizationId: "5",
      organizationName: "Testträger",
      deviceIdentifier: "reader-entrance",
      deviceName: "Eingang",
    });
  });

  it("normalizes nullable backend fields to null", () => {
    const backend: BackendUnregisteredTagScan = {
      ...fullBackendScan,
      device_id: null,
      resolved_at: undefined,
      resolved_by_operator_id: null,
      resolution_note: undefined,
      device_identifier: undefined,
      device_name: null,
    };

    const result = mapUnregisteredTagScan(backend);

    expect(result.deviceId).toBeNull();
    expect(result.resolvedAt).toBeNull();
    expect(result.resolvedByOperatorId).toBeNull();
    expect(result.resolutionNote).toBeNull();
    expect(result.deviceIdentifier).toBeNull();
    expect(result.deviceName).toBeNull();
  });
});

describe("mapSchoolPWAUsage", () => {
  it("maps snake_case portal buckets to camelCase", () => {
    const result = mapSchoolPWAUsage({
      window_days: 30,
      staff: { standalone_users: 3, eligible_users: 12 },
      parent: { standalone_users: 47, eligible_users: 210 },
    });

    expect(result).toEqual({
      windowDays: 30,
      staff: { standaloneUsers: 3, eligibleUsers: 12 },
      parent: { standaloneUsers: 47, eligibleUsers: 210 },
    });
  });
});
