"use client";

import type { ReactNode } from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import type { Organization, School } from "~/lib/operator/provisioning-helpers";

export function OrgSchoolFilter({
  idPrefix,
  organizations,
  filteredSchools,
  filterOrgId,
  selectedSchool,
  onOrgChange,
  onSchoolChange,
  children,
}: {
  readonly idPrefix: string;
  readonly organizations: Organization[] | undefined;
  readonly filteredSchools: School[];
  readonly filterOrgId: string;
  readonly selectedSchool: School | null;
  readonly onOrgChange: (orgId: string) => void;
  readonly onSchoolChange: (schoolId: string) => void;
  /** Ein weiteres Feld derselben Zeile (Status der RFID-Scans). */
  readonly children?: ReactNode;
}) {
  const orgOptions = [
    { value: "", label: "Alle Träger" },
    ...(organizations?.map((org) => ({ value: org.id, label: org.name })) ??
      []),
  ];
  const schoolOptions = [
    { value: "", label: filterOrgId ? "Alle Schulen" : "Schule auswählen…" },
    ...filteredSchools.map((school) => ({
      value: school.id,
      label: `${school.name}${
        school.organization ? ` (${school.organization.name})` : ""
      }`,
    })),
  ];

  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
      <div className="flex-1">
        <label
          id={`filter-${idPrefix}-org-label`}
          htmlFor={`filter-${idPrefix}-org`}
          className="mb-1 block text-xs font-medium text-gray-500"
        >
          Träger
        </label>
        <CustomSelect
          id={`filter-${idPrefix}-org`}
          ariaLabelledBy={`filter-${idPrefix}-org-label`}
          value={filterOrgId}
          options={orgOptions}
          onChange={onOrgChange}
        />
      </div>
      <div className="flex-1">
        <label
          id={`filter-${idPrefix}-school-label`}
          htmlFor={`filter-${idPrefix}-school`}
          className="mb-1 block text-xs font-medium text-gray-500"
        >
          Schule
        </label>
        <CustomSelect
          id={`filter-${idPrefix}-school`}
          ariaLabelledBy={`filter-${idPrefix}-school-label`}
          value={selectedSchool?.id ?? ""}
          options={schoolOptions}
          onChange={onSchoolChange}
        />
      </div>
      {children}
    </div>
  );
}
