/**
 * Tests for useOrgSchoolFilter — guards the Papierkorb-hiding contract that
 * lives in the activeSchools / filteredSchools memos. Issue #1435.
 */
import { describe, it, expect, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import type { Organization, School } from "~/lib/operator/provisioning-helpers";

const mockUseSession = vi.fn(() => ({ status: "authenticated" }));
vi.mock("next-auth/react", () => ({
  useSession: () => mockUseSession(),
}));

const mockUseRouter = vi.fn(() => ({
  replace: vi.fn(),
  push: vi.fn(),
}));
const mockUseSearchParams = vi.fn(() => new URLSearchParams());
vi.mock("next/navigation", () => ({
  useRouter: () => mockUseRouter(),
  useSearchParams: () => mockUseSearchParams(),
}));

const mockSWRData = vi.fn();
vi.mock("swr", () => ({
  default: (key: string | null) => {
    const data = mockSWRData(key);
    return { data, error: undefined, isLoading: false, mutate: vi.fn() };
  },
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    listOrganizations: vi.fn(),
    listSchools: vi.fn(),
  },
}));

import { useOrgSchoolFilter } from "./use-org-school-filter";

const org = (id: string, deletedAt: string | null = null): Organization =>
  ({
    id,
    name: `Org ${id}`,
    slug: `org-${id}`,
    active: true,
    createdAt: "2025-01-01T00:00:00Z",
    updatedAt: "2025-01-01T00:00:00Z",
    deletedAt,
  }) as unknown as Organization;

const school = (
  id: string,
  organizationId: string,
  deletedAt: string | null = null,
): School =>
  ({
    id,
    organizationId,
    name: `School ${id}`,
    slug: `school-${id}`,
    subdomain: `school-${id}`,
    active: true,
    hidden: false,
    deletedAt,
    createdAt: "2025-01-01T00:00:00Z",
    updatedAt: "2025-01-01T00:00:00Z",
    organization: org(organizationId),
  }) as unknown as School;

function setupData(orgs: Organization[], schools: School[]): void {
  mockSWRData.mockImplementation((key: string | null) => {
    if (key === "operator-organizations") return orgs;
    if (key === "operator-schools") return schools;
    return undefined;
  });
}

describe("useOrgSchoolFilter — Papierkorb filtering (issue #1435)", () => {
  it("activeSchools excludes soft-deleted schools", () => {
    setupData(
      [org("1")],
      [
        school("10", "1"),
        school("11", "1", "2025-06-01T00:00:00Z"),
        school("12", "1"),
      ],
    );

    const { result } = renderHook(() => useOrgSchoolFilter("/devices"));

    expect(result.current.activeSchools.map((s) => s.id)).toEqual(["10", "12"]);
    expect(result.current.schools).toHaveLength(3);
  });

  it("filteredSchools without orgId returns activeSchools (no deleted)", () => {
    setupData(
      [org("1")],
      [school("10", "1"), school("11", "1", "2025-06-01T00:00:00Z")],
    );

    const { result } = renderHook(() => useOrgSchoolFilter("/devices"));

    expect(result.current.filteredSchools.map((s) => s.id)).toEqual(["10"]);
  });

  it("filteredSchools with orgId still excludes deleted schools", () => {
    mockUseSearchParams.mockReturnValueOnce(new URLSearchParams("orgId=1"));
    setupData(
      [org("1"), org("2")],
      [
        school("10", "1"),
        school("11", "1", "2025-06-01T00:00:00Z"),
        school("20", "2"),
      ],
    );

    const { result } = renderHook(() => useOrgSchoolFilter("/devices"));

    expect(result.current.filteredSchools.map((s) => s.id)).toEqual(["10"]);
  });

  it("activeSchools is empty when schools is undefined (SWR not loaded yet)", () => {
    mockSWRData.mockReturnValue(undefined);

    const { result } = renderHook(() => useOrgSchoolFilter("/devices"));

    expect(result.current.activeSchools).toEqual([]);
    expect(result.current.filteredSchools).toEqual([]);
  });
});
