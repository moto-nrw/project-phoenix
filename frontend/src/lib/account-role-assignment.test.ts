import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Role } from "~/lib/auth-helpers";
import { loadAccountRoleAssignment } from "./account-role-assignment";

const { mockGetAccountRoles, mockGetRoles } = vi.hoisted(() => ({
  mockGetAccountRoles: vi.fn(),
  mockGetRoles: vi.fn(),
}));

vi.mock("~/lib/auth-service", () => ({
  authService: {
    getAccountRoles: mockGetAccountRoles,
    getRoles: mockGetRoles,
  },
}));

function role(id: string, name: string, isSystem: boolean): Role {
  return {
    id,
    name,
    description: "",
    isSystem,
    createdAt: "",
    updatedAt: "",
  };
}

describe("loadAccountRoleAssignment", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps a tenant-defined role named lehrkraft selectable", async () => {
    mockGetAccountRoles.mockResolvedValue([role("4", "lehrkraft", false)]);
    mockGetRoles.mockResolvedValue([
      role("3", "lehrkraft", true),
      role("4", "lehrkraft", false),
    ]);

    const assignment = await loadAccountRoleAssignment("7");

    expect(assignment.options).toEqual([
      { id: "4", name: "Lehrkraft", systemName: "lehrkraft" },
    ]);
    expect(assignment.currentIsLehrkraft).toBe(false);
  });

  it("keeps the system role lehrkraft read-only", async () => {
    mockGetAccountRoles.mockResolvedValue([role("3", "lehrkraft", true)]);
    mockGetRoles.mockResolvedValue([role("3", "lehrkraft", true)]);

    const assignment = await loadAccountRoleAssignment("7");

    expect(assignment.options).toEqual([]);
    expect(assignment.currentIsLehrkraft).toBe(true);
  });
});
