"use client";

import { useCallback, useMemo } from "react";
import { useRouter, useSearchParams } from "next/navigation";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
import { useSession } from "next-auth/react";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { Organization, School } from "~/lib/operator/provisioning-helpers";
import { operatorPath } from "~/lib/operator-url";

// Shared URL-param filter plumbing for the three operator list pages that
// filter by Träger + Schule via `?orgId=…&schoolId=…` (accounts, devices,
// persons). Callers must be wrapped in a <Suspense> boundary because this
// hook uses useSearchParams().
export function useOrgSchoolFilter(routePath: string): {
  readonly isAuthenticated: boolean;
  readonly organizations: Organization[] | undefined;
  readonly schools: School[] | undefined;
  readonly activeOrganizations: Organization[];
  readonly filterOrgId: string;
  readonly urlSchoolId: string;
  readonly selectedSchool: School | null;
  readonly filteredSchools: School[];
  readonly handleOrgFilterChange: (orgId: string) => void;
  readonly handleSchoolFilterChange: (schoolId: string) => void;
} {
  const { status } = useSession();
  const isAuthenticated = status === "authenticated";

  const router = useRouter();
  const searchParams = useSearchParams();
  const filterOrgId = searchParams.get("orgId") ?? "";
  const urlSchoolId = searchParams.get("schoolId") ?? "";

  const { data: organizations } = useSWR(
    isAuthenticated ? "operator-organizations" : null,
    () => operatorProvisioningService.listOrganizations(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const { data: schools } = useSWR(
    isAuthenticated ? "operator-schools" : null,
    () => operatorProvisioningService.listSchools(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const activeOrganizations = useMemo(
    () => organizations?.filter((o) => o.deletedAt == null) ?? [],
    [organizations],
  );

  const selectedSchool = useMemo<School | null>(() => {
    if (!urlSchoolId) return null;
    return schools?.find((s) => s.id === urlSchoolId) ?? null;
  }, [urlSchoolId, schools]);

  const filteredSchools = useMemo(() => {
    if (!schools) return [];
    if (!filterOrgId) return schools;
    return schools.filter((s) => s.organizationId === filterOrgId);
  }, [schools, filterOrgId]);

  const updateQuery = useCallback(
    (next: { orgId?: string; schoolId?: string }) => {
      const params = new URLSearchParams(searchParams.toString());
      if (next.orgId !== undefined) {
        if (next.orgId) params.set("orgId", next.orgId);
        else params.delete("orgId");
      }
      if (next.schoolId !== undefined) {
        if (next.schoolId) params.set("schoolId", next.schoolId);
        else params.delete("schoolId");
      }
      const query = params.toString();
      router.replace(operatorPath(`${routePath}${query ? `?${query}` : ""}`));
    },
    [router, searchParams, routePath],
  );

  const handleOrgFilterChange = useCallback(
    (orgId: string) => {
      if (selectedSchool && orgId && selectedSchool.organizationId !== orgId) {
        updateQuery({ orgId, schoolId: "" });
      } else {
        updateQuery({ orgId });
      }
    },
    [selectedSchool, updateQuery],
  );

  const handleSchoolFilterChange = useCallback(
    (schoolId: string) => {
      updateQuery({ schoolId });
    },
    [updateQuery],
  );

  return {
    isAuthenticated,
    organizations,
    schools,
    activeOrganizations,
    filterOrgId,
    urlSchoolId,
    selectedSchool,
    filteredSchools,
    handleOrgFilterChange,
    handleSchoolFilterChange,
  };
}
