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
import type { TimetableRosterRow } from "~/lib/timetable-operations-types";

interface FilterCriteria {
  readonly searchTerm: string;
  readonly groupFilter: string;
  readonly yearFilter: string;
}

/** What the search and the filters look at, from a card or a roster row. */
interface FilterableChild {
  readonly names: readonly (string | undefined)[];
  readonly groupName: string | undefined;
  readonly schoolClass: string;
}

function matchesCriteria(
  child: FilterableChild,
  { searchTerm, groupFilter, yearFilter }: FilterCriteria,
): boolean {
  if (searchTerm) {
    const searchLower = searchTerm.toLowerCase();
    const matchesSearch = child.names.some(
      (name) => name?.toLowerCase().includes(searchLower) ?? false,
    );
    if (!matchesSearch) return false;
  }
  if (groupFilter !== "all") {
    const studentGroupName = child.groupName ?? "Unbekannt";
    if (studentGroupName !== groupFilter) return false;
  }
  if (yearFilter !== "all") {
    const studentYear = getSchoolYear(child.schoolClass);
    if (studentYear !== yearFilter) return false;
  }
  return true;
}

function studentAsFilterable(
  student: ActiveSupervisionStudent,
): FilterableChild {
  return {
    names: [student.name, student.first_name, student.second_name],
    groupName: student.group_name,
    schoolClass: student.school_class,
  };
}

function rosterRowAsFilterable(row: TimetableRosterRow): FilterableChild {
  return {
    names: [row.studentName],
    groupName: row.groupName,
    schoolClass: row.schoolClass,
  };
}

export interface StudentFilters {
  readonly searchTerm: string;
  readonly setSearchTerm: (value: string) => void;
  readonly setGroupFilter: (value: string) => void;
  readonly setSelectedYear: (value: string) => void;
  readonly filteredStudents: ActiveSupervisionStudent[];
  /**
   * The same search and filters for the rows of a block list (#3889): it
   * holds expected, absent and departed children too, under their status.
   * Null while nothing is searched or filtered.
   */
  readonly rosterRowFilter: ((row: TimetableRosterRow) => boolean) | null;
  readonly filterConfigs: FilterConfig[];
  readonly activeFilters: ActiveFilter[];
  readonly clearAllFilters: () => void;
}

/**
 * Search / group / year filter state of the visitor list and the block list,
 * with the PageHeaderWithSearch configs derived from the children of both.
 */
export function useStudentFilters(
  students: readonly ActiveSupervisionStudent[],
  rosterRows: readonly TimetableRosterRow[] = [],
): StudentFilters {
  const [searchTerm, setSearchTerm] = useState("");
  const [groupFilter, setGroupFilter] = useState("all");
  const [selectedYear, setSelectedYear] = useState("all");

  const filteredStudents = useMemo(() => {
    const criteria = { searchTerm, groupFilter, yearFilter: selectedYear };
    return (Array.isArray(students) ? students : []).filter((student) =>
      matchesCriteria(studentAsFilterable(student), criteria),
    );
  }, [students, searchTerm, groupFilter, selectedYear]);

  const rosterRowFilter = useMemo(() => {
    if (!searchTerm && groupFilter === "all" && selectedYear === "all") {
      return null;
    }
    const criteria = { searchTerm, groupFilter, yearFilter: selectedYear };
    return (row: TimetableRosterRow) =>
      matchesCriteria(rosterRowAsFilterable(row), criteria);
  }, [searchTerm, groupFilter, selectedYear]);

  const filterConfigs: FilterConfig[] = useMemo(() => {
    // Compute available groups inside useMemo to ensure proper updates
    const groups = Array.from(
      new Set(
        [
          ...students.map((student) => student.group_name),
          ...rosterRows.map((row) => row.groupName),
        ].filter((name): name is string => !!name),
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
  }, [selectedYear, groupFilter, students, rosterRows]);

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
    rosterRowFilter,
    filterConfigs,
    activeFilters,
    clearAllFilters,
  };
}
