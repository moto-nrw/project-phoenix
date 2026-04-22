"use client";

import { useCallback, useState } from "react";
import type { Organization, School } from "~/lib/operator/provisioning-helpers";

export type SortDirection = "asc" | "desc";

export interface SortState<K extends string> {
  key: K;
  direction: SortDirection;
}

export function useSort<K extends string>(
  defaultKey: K,
  defaultDirection: SortDirection = "asc",
) {
  const [sort, setSort] = useState<SortState<K>>({
    key: defaultKey,
    direction: defaultDirection,
  });

  const toggle = useCallback((key: K) => {
    setSort((prev) =>
      prev.key === key
        ? { key, direction: prev.direction === "asc" ? "desc" : "asc" }
        : { key, direction: "asc" },
    );
  }, []);

  return { sort, toggle };
}

export function SortIndicator({
  active,
  direction,
}: {
  readonly active: boolean;
  readonly direction: SortDirection;
}) {
  return (
    <span
      className={`ml-1 inline-block ${active ? "text-gray-700" : "text-gray-300"}`}
    >
      {active ? (direction === "asc" ? "↑" : "↓") : "↕"}
    </span>
  );
}

export function SortableHeader<K extends string>({
  label,
  sortKey,
  sort,
  onToggle,
}: {
  readonly label: string;
  readonly sortKey: K;
  readonly sort: SortState<K>;
  readonly onToggle: (key: K) => void;
}) {
  return (
    <th
      className="cursor-pointer px-5 py-3 transition-colors select-none hover:text-gray-700"
      onClick={() => onToggle(sortKey)}
    >
      {label}
      <SortIndicator active={sort.key === sortKey} direction={sort.direction} />
    </th>
  );
}

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
