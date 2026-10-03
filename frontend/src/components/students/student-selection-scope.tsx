"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import dynamic from "next/dynamic";

import { StudentSelectionBar } from "~/components/students/student-selection-bar";
import { useBulkCheckinActions } from "~/components/students/use-bulk-checkin-actions";
import type { Student } from "~/lib/api";
import type { StudentExportFilters } from "~/lib/student-export-api";
import { useSchoolCheckinMode } from "~/lib/hooks/use-school-checkin-mode";
import { useAttendanceWebEnabled } from "~/lib/tenant-context";

// Loaded when someone opens it: most visits never export.
const StudentExportModal = dynamic(
  () =>
    import("~/components/students/student-export-modal").then(
      (module) => module.StudentExportModal,
    ),
  { ssr: false },
);

/** What a table needs to show and change the marks. */
export interface StudentTableSelection {
  readonly selectedIds: ReadonlySet<string>;
  readonly onChange: (ids: readonly string[], selected: boolean) => void;
  readonly disabled?: boolean;
}

/**
 * Selection, bulk actions and selection export of a table view (#3834).
 * Mounted only while the table shows, so the tiles keep exactly the page
 * they had before; its own check-in hook instance never mixes table marks
 * with the tiles' check-in mode.
 */
export function StudentSelectionScope({
  visibleStudents,
  scopeKey,
  checkinAllowed,
  exportContext,
  children,
}: Readonly<{
  /** The rows on screen; marks outside them are never acted on (review #2372). */
  visibleStudents: readonly Student[];
  /** Changes whenever the list on screen is a different one; clears the marks. */
  scopeKey: string;
  /**
   * Whether the page may check children in or out at all (today, the right
   * presence mode). The school's web attendance setting is checked here.
   */
  checkinAllowed: boolean;
  /**
   * What the export needs besides the marked children: the planning date of
   * a non-today view and the page's sort order.
   */
  exportContext?: Pick<StudentExportFilters, "date" | "sort">;
  children: (selection: StudentTableSelection) => ReactNode;
}>) {
  const schoolCheckin = useSchoolCheckinMode();
  const attendanceWebEnabled = useAttendanceWebEnabled();
  const checkinEnabled = checkinAllowed && attendanceWebEnabled;
  const clearSelection = schoolCheckin.clearSelection;
  useEffect(() => {
    clearSelection();
  }, [scopeKey, clearSelection]);

  const selectedStudents = useMemo(
    () =>
      visibleStudents.filter((student) =>
        schoolCheckin.selectedIds.has(student.id.toString()),
      ),
    [visibleStudents, schoolCheckin.selectedIds],
  );
  const bulkCheckin = useBulkCheckinActions(schoolCheckin, selectedStudents);
  const [isExportOpen, setIsExportOpen] = useState(false);

  return (
    <>
      <StudentSelectionBar
        selectedCount={selectedStudents.length}
        onClear={schoolCheckin.clearSelection}
        onExport={() => setIsExportOpen(true)}
        checkin={
          checkinEnabled
            ? {
                onAction: bulkCheckin.handleBulkAction,
                runningAction: bulkCheckin.runningBulkAction,
                isRunning: schoolCheckin.isBulkRunning,
              }
            : undefined
        }
      />
      {children({
        selectedIds: schoolCheckin.selectedIds,
        onChange: schoolCheckin.setSelected,
        disabled: schoolCheckin.isBulkRunning,
      })}
      {bulkCheckin.dialogs}
      {isExportOpen && (
        <StudentExportModal
          isOpen={isExportOpen}
          heading="Ausgewählte Kinder exportieren"
          filters={{
            ...exportContext,
            student_ids: selectedStudents.map((student) =>
              student.id.toString(),
            ),
          }}
          resultCount={selectedStudents.length}
          onClose={() => setIsExportOpen(false)}
        />
      )}
    </>
  );
}
