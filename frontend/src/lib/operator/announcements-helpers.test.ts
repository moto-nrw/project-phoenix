import { describe, it, expect } from "vitest";
import {
  mapViewDetail,
  mapAnnouncement,
  TYPE_LABELS,
  SEVERITY_LABELS,
  ANNOUNCEMENT_STATUS_LABELS,
  SYSTEM_ROLE_LABELS,
  TYPE_STYLES,
  SEVERITY_STYLES,
  ANNOUNCEMENT_STATUS_STYLES,
} from "./announcements-helpers";
import type {
  BackendAnnouncementViewDetail,
  BackendAnnouncement,
} from "./announcements-helpers";

describe("mapViewDetail", () => {
  it("maps backend view detail to frontend type", () => {
    const backendDetail: BackendAnnouncementViewDetail = {
      user_id: 123,
      user_name: "John Doe",
      seen_at: "2024-01-15T10:30:00Z",
      dismissed: true,
    };

    const result = mapViewDetail(backendDetail);

    expect(result.userId).toBe("123");
    expect(result.userName).toBe("John Doe");
    expect(result.seenAt).toBe("2024-01-15T10:30:00Z");
    expect(result.dismissed).toBe(true);
  });

  it("maps view detail with dismissed false", () => {
    const backendDetail: BackendAnnouncementViewDetail = {
      user_id: 456,
      user_name: "Jane Smith",
      seen_at: "2024-02-20T14:45:00Z",
      dismissed: false,
    };

    const result = mapViewDetail(backendDetail);

    expect(result.userId).toBe("456");
    expect(result.dismissed).toBe(false);
  });
});

describe("mapAnnouncement", () => {
  it("maps backend announcement with all fields", () => {
    const backendAnnouncement: BackendAnnouncement = {
      id: 99,
      title: "System Maintenance",
      content: "The system will be down for maintenance.",
      type: "maintenance",
      severity: "warning",
      version: "2.1.0",
      active: true,
      published_at: "2024-03-01T09:00:00Z",
      expires_at: "2024-03-31T23:59:59Z",
      target_roles: ["admin", "user"],
      target_org_ids: [],
      target_tenant_ids: [],
      created_by: 42,
      created_at: "2024-02-28T10:00:00Z",
      updated_at: "2024-03-01T08:00:00Z",
      status: "published",
    };

    const result = mapAnnouncement(backendAnnouncement);

    expect(result.id).toBe("99");
    expect(result.title).toBe("System Maintenance");
    expect(result.content).toBe("The system will be down for maintenance.");
    expect(result.type).toBe("maintenance");
    expect(result.severity).toBe("warning");
    expect(result.version).toBe("2.1.0");
    expect(result.active).toBe(true);
    expect(result.publishedAt).toBe("2024-03-01T09:00:00Z");
    expect(result.expiresAt).toBe("2024-03-31T23:59:59Z");
    expect(result.targetRoles).toEqual(["admin", "user"]);
    expect(result.createdBy).toBe("42");
    expect(result.createdAt).toBe("2024-02-28T10:00:00Z");
    expect(result.updatedAt).toBe("2024-03-01T08:00:00Z");
    expect(result.status).toBe("published");
  });

  it("maps announcement with null optional fields", () => {
    const backendAnnouncement: BackendAnnouncement = {
      id: 100,
      title: "Important Notice",
      content: "Please read this.",
      type: "announcement",
      severity: "info",
      version: null,
      active: false,
      published_at: null,
      expires_at: null,
      target_roles: [],
      target_org_ids: [],
      target_tenant_ids: [],
      created_by: 1,
      created_at: "2024-04-01T08:00:00Z",
      updated_at: "2024-04-01T08:00:00Z",
      status: "draft",
    };

    const result = mapAnnouncement(backendAnnouncement);

    expect(result.id).toBe("100");
    expect(result.version).toBeNull();
    expect(result.publishedAt).toBeNull();
    expect(result.expiresAt).toBeNull();
    expect(result.targetRoles).toEqual([]);
    expect(result.active).toBe(false);
    expect(result.status).toBe("draft");
  });

  it("maps announcement with non-empty org and tenant targeting", () => {
    const backendAnnouncement: BackendAnnouncement = {
      id: 200,
      title: "Org-Scoped Notice",
      content: "Only for specific orgs and tenants.",
      type: "announcement",
      severity: "info",
      version: null,
      active: true,
      published_at: "2024-06-01T00:00:00Z",
      expires_at: null,
      target_roles: ["admin"],
      target_org_ids: [1, 2],
      target_tenant_ids: [5, 10, 15],
      created_by: 1,
      created_at: "2024-05-30T00:00:00Z",
      updated_at: "2024-06-01T00:00:00Z",
      status: "published",
    };

    const result = mapAnnouncement(backendAnnouncement);

    expect(result.targetOrgIds).toEqual([1, 2]);
    expect(result.targetTenantIds).toEqual([5, 10, 15]);
    expect(result.targetRoles).toEqual(["admin"]);
  });

  it("maps announcement with empty org and tenant arrays", () => {
    const backendAnnouncement: BackendAnnouncement = {
      id: 201,
      title: "Global Notice",
      content: "Visible to all.",
      type: "announcement",
      severity: "info",
      version: null,
      active: true,
      published_at: "2024-06-01T00:00:00Z",
      expires_at: null,
      target_roles: [],
      target_org_ids: [],
      target_tenant_ids: [],
      created_by: 1,
      created_at: "2024-05-30T00:00:00Z",
      updated_at: "2024-06-01T00:00:00Z",
      status: "published",
    };

    const result = mapAnnouncement(backendAnnouncement);

    expect(result.targetOrgIds).toEqual([]);
    expect(result.targetTenantIds).toEqual([]);
  });

  it("maps release announcement correctly", () => {
    const backendAnnouncement: BackendAnnouncement = {
      id: 101,
      title: "New Release Available",
      content: "Version 3.0 is here!",
      type: "release",
      severity: "critical",
      version: "3.0.0",
      active: true,
      published_at: "2024-05-01T00:00:00Z",
      expires_at: null,
      target_roles: ["admin", "user", "guardian"],
      target_org_ids: [],
      target_tenant_ids: [],
      created_by: 5,
      created_at: "2024-04-30T12:00:00Z",
      updated_at: "2024-05-01T00:00:00Z",
      status: "published",
    };

    const result = mapAnnouncement(backendAnnouncement);

    expect(result.type).toBe("release");
    expect(result.version).toBe("3.0.0");
    expect(result.targetRoles).toHaveLength(3);
  });
});

describe("TYPE_LABELS", () => {
  it("contains all announcement type labels", () => {
    expect(TYPE_LABELS.announcement).toBe("Ankündigung");
    expect(TYPE_LABELS.release).toBe("Release");
    expect(TYPE_LABELS.maintenance).toBe("Wartung");
  });
});

describe("SEVERITY_LABELS", () => {
  it("contains all severity labels", () => {
    expect(SEVERITY_LABELS.info).toBe("Info");
    expect(SEVERITY_LABELS.warning).toBe("Warnung");
    expect(SEVERITY_LABELS.critical).toBe("Kritisch");
  });
});

describe("ANNOUNCEMENT_STATUS_LABELS", () => {
  it("contains all status labels", () => {
    expect(ANNOUNCEMENT_STATUS_LABELS.draft).toBe("Entwurf");
    expect(ANNOUNCEMENT_STATUS_LABELS.published).toBe("Veröffentlicht");
    expect(ANNOUNCEMENT_STATUS_LABELS.expired).toBe("Abgelaufen");
  });
});

describe("SYSTEM_ROLE_LABELS", () => {
  it("contains all role labels", () => {
    expect(SYSTEM_ROLE_LABELS.admin).toBe("Administratoren");
    expect(SYSTEM_ROLE_LABELS.user).toBe("Lehrer/Personal");
    expect(SYSTEM_ROLE_LABELS.guardian).toBe("Erziehungsberechtigte");
  });
});

describe("TYPE_STYLES", () => {
  it("contains style classes for all types", () => {
    expect(TYPE_STYLES.announcement).toContain("bg-blue-100");
    expect(TYPE_STYLES.release).toContain("bg-green-100");
    expect(TYPE_STYLES.maintenance).toContain("bg-orange-100");
  });
});

describe("SEVERITY_STYLES", () => {
  it("contains style classes for all severities", () => {
    expect(SEVERITY_STYLES.info).toContain("bg-gray-100");
    expect(SEVERITY_STYLES.warning).toContain("bg-yellow-100");
    expect(SEVERITY_STYLES.critical).toContain("bg-red-100");
  });
});

describe("ANNOUNCEMENT_STATUS_STYLES", () => {
  it("contains style classes for all statuses", () => {
    expect(ANNOUNCEMENT_STATUS_STYLES.draft).toContain("bg-gray-100");
    expect(ANNOUNCEMENT_STATUS_STYLES.published).toContain("bg-green-100");
    expect(ANNOUNCEMENT_STATUS_STYLES.expired).toContain("bg-red-100");
  });
});

describe("mapAnnouncement edge cases", () => {
  it("defaults undefined target_org_ids to empty array", () => {
    const data = {
      id: 300,
      title: "No Targeting",
      content: "Content",
      type: "announcement" as const,
      severity: "info" as const,
      version: null,
      active: true,
      published_at: null,
      expires_at: null,
      target_roles: [],
      created_by: 1,
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-01T00:00:00Z",
      status: "draft" as const,
    } as unknown as BackendAnnouncement;

    // Simulate missing fields (as if backend didn't include them)
    const raw = data as unknown as Record<string, unknown>;
    delete raw["target_org_ids"];
    delete raw["target_tenant_ids"];

    const result = mapAnnouncement(data);

    expect(result.targetOrgIds).toEqual([]);
    expect(result.targetTenantIds).toEqual([]);
  });

  it("handles single org targeting correctly", () => {
    const data: BackendAnnouncement = {
      id: 301,
      title: "Single Org",
      content: "Content",
      type: "announcement",
      severity: "info",
      version: null,
      active: true,
      published_at: "2024-06-01T00:00:00Z",
      expires_at: null,
      target_roles: [],
      target_org_ids: [42],
      target_tenant_ids: [],
      created_by: 1,
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-01T00:00:00Z",
      status: "published",
    };

    const result = mapAnnouncement(data);

    expect(result.targetOrgIds).toEqual([42]);
    expect(result.targetTenantIds).toEqual([]);
  });

  it("handles single tenant targeting correctly", () => {
    const data: BackendAnnouncement = {
      id: 302,
      title: "Single Tenant",
      content: "Content",
      type: "announcement",
      severity: "info",
      version: null,
      active: true,
      published_at: "2024-06-01T00:00:00Z",
      expires_at: null,
      target_roles: ["user"],
      target_org_ids: [],
      target_tenant_ids: [99],
      created_by: 1,
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-01T00:00:00Z",
      status: "published",
    };

    const result = mapAnnouncement(data);

    expect(result.targetOrgIds).toEqual([]);
    expect(result.targetTenantIds).toEqual([99]);
    expect(result.targetRoles).toEqual(["user"]);
  });

  it("maps OR-union targeting (both org and tenant)", () => {
    const data: BackendAnnouncement = {
      id: 303,
      title: "OR Union",
      content: "Content",
      type: "announcement",
      severity: "critical",
      version: null,
      active: true,
      published_at: "2024-06-01T00:00:00Z",
      expires_at: null,
      target_roles: ["admin", "guardian"],
      target_org_ids: [1, 2, 3],
      target_tenant_ids: [10, 20],
      created_by: 1,
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-01T00:00:00Z",
      status: "published",
    };

    const result = mapAnnouncement(data);

    expect(result.targetOrgIds).toEqual([1, 2, 3]);
    expect(result.targetTenantIds).toEqual([10, 20]);
    expect(result.targetRoles).toEqual(["admin", "guardian"]);
    expect(result.severity).toBe("critical");
  });
});
