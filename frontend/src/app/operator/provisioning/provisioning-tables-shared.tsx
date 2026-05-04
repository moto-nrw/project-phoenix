"use client";

import type { Organization, School } from "~/lib/operator/provisioning-helpers";

export function OrgSchoolFilter({
  idPrefix,
  organizations,
  filteredSchools,
  filterOrgId,
  selectedSchool,
  onOrgChange,
  onSchoolChange,
}: {
  readonly idPrefix: string;
  readonly organizations: Organization[] | undefined;
  readonly filteredSchools: School[];
  readonly filterOrgId: string;
  readonly selectedSchool: School | null;
  readonly onOrgChange: (orgId: string) => void;
  readonly onSchoolChange: (schoolId: string) => void;
}) {
  return (
    <div className="mt-4 mb-4 flex flex-col gap-3 sm:flex-row sm:items-end">
      <div className="flex-1">
        <label
          htmlFor={`filter-${idPrefix}-org`}
          className="mb-1 block text-xs font-medium text-gray-500"
        >
          Träger
        </label>
        <select
          id={`filter-${idPrefix}-org`}
          value={filterOrgId}
          onChange={(e) => onOrgChange(e.target.value)}
          className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:ring-1 focus:ring-gray-400 focus:outline-none"
        >
          <option value="">Alle Träger</option>
          {organizations?.map((org) => (
            <option key={org.id} value={org.id}>
              {org.name}
            </option>
          ))}
        </select>
      </div>
      <div className="flex-1">
        <label
          htmlFor={`filter-${idPrefix}-school`}
          className="mb-1 block text-xs font-medium text-gray-500"
        >
          Schule
        </label>
        <select
          id={`filter-${idPrefix}-school`}
          value={selectedSchool?.id ?? ""}
          onChange={(e) => onSchoolChange(e.target.value)}
          className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:ring-1 focus:ring-gray-400 focus:outline-none"
        >
          <option value="">
            {filterOrgId ? "Alle Schulen" : "Schule auswählen…"}
          </option>
          {filteredSchools.map((school) => (
            <option key={school.id} value={school.id}>
              {school.name}
              {school.organization ? ` (${school.organization.name})` : ""}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}
