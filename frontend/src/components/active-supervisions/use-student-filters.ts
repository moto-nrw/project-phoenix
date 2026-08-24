"use client";

import { useMemo, useState } from "react";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import {
  SCHOOL_YEAR_FILTER_OPTIONS,
  getSchoolYear,
} from "~/lib/student-helpers";
import type { ActiveSupervisionStudent } from "~/components/active-supervisions/view-model";

/** Check if a student matches the current search, group, and year filters */
function matchesStudentFilters(
  student: ActiveSupervisionStudent,
  searchTerm: string,
  groupFilter: string,
  yearFilter: string,
): boolean {
  if (searchTerm) {
    const searchLower = searchTerm.toLowerCase();
    const matchesSearch =
      (student.name?.toLowerCase().includes(searchLower) ?? false) ||
      (student.first_name?.toLowerCase().includes(searchLower) ?? false) ||
      (student.second_name?.toLowerCase().includes(searchLower) ?? false);
    if (!matchesSearch) return false;
  }
  if (groupFilter !== "all") {
    const studentGroupName = student.group_name ?? "Unbekannt";
    if (studentGroupName !== groupFilter) return false;
  }
  if (yearFilter !== "all") {
    const studentYear = getSchoolYear(student.school_class);
    if (studentYear !== yearFilter) return false;
  }
  return true;
}

export interface StudentFilters {
  readonly searchTerm: string;
  readonly setSearchTerm: (value: string) => void;
  readonly setGroupFilter: (value: string) => void;
  readonly setSelectedYear: (value: string) => void;
  readonly filteredStudents: ActiveSupervisionStudent[];
  readonly filterConfigs: FilterConfig[];
  readonly activeFilters: ActiveFilter[];
  readonly clearAllFilters: () => void;
}

/**
 * Search / group / year filter state of the visitor list, with the
 * PageHeaderWithSearch configs derived from the current students.
 */
export function useStudentFilters(
  students: readonly ActiveSupervisionStudent[],
): StudentFilters {
  const [searchTerm, setSearchTerm] = useState("");
  const [groupFilter, setGroupFilter] = useState("all");
  const [selectedYear, setSelectedYear] = useState("all");

  const filteredStudents = useMemo(
    () =>
      (Array.isArray(students) ? students : []).filter((student) =>
        matchesStudentFilters(student, searchTerm, groupFilter, selectedYear),
      ),
    [students, searchTerm, groupFilter, selectedYear],
  );

  const filterConfigs: FilterConfig[] = useMemo(() => {
    // Compute available groups inside useMemo to ensure proper updates
    const groups = Array.from(
      new Set(
        students
          .map((student) => student.group_name)
          .filter((name): name is string => !!name),
      ),
    ).sort((a, b) => a.localeCompare(b, "de"));

    return [
      {
        id: "year",
        label: "Klassenstufe",
        type: "buttons",
        value: selectedYear,
        onChange: (value) => setSelectedYear(value as string),
        options: [...SCHOOL_YEAR_FILTER_OPTIONS],
      },
      {
        id: "group",
        label: "Gruppe",
        type: "dropdown",
        value: groupFilter,
        onChange: (value) => setGroupFilter(value as string),
        options: [
          { value: "all", label: "Alle Gruppen" },
          ...groups.map((groupName) => ({
            value: groupName,
            label: groupName,
          })),
        ],
      },
    ];
  }, [selectedYear, groupFilter, students]);

  const activeFilters: ActiveFilter[] = useMemo(() => {
    const filters: ActiveFilter[] = [];

    if (searchTerm) {
      filters.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    }

    if (selectedYear !== "all") {
      filters.push({
        id: "year",
        label: `Jahr ${selectedYear}`,
        onRemove: () => setSelectedYear("all"),
      });
    }

    if (groupFilter !== "all") {
      filters.push({
        id: "group",
        label: `Gruppe: ${groupFilter}`,
        onRemove: () => setGroupFilter("all"),
      });
    }

    return filters;
  }, [searchTerm, selectedYear, groupFilter]);

  const clearAllFilters = () => {
    setSearchTerm("");
    setGroupFilter("all");
    setSelectedYear("all");
  };

  return {
    searchTerm,
    setSearchTerm,
    setGroupFilter,
    setSelectedYear,
    filteredStudents,
    filterConfigs,
    activeFilters,
    clearAllFilters,
  };
}
