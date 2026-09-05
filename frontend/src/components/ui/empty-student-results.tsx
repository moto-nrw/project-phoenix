// components/ui/empty-student-results.tsx
// Shared empty state component for student search/filter results

import { Search } from "lucide-react";
import { EmptyState } from "~/components/ui/empty-state";

interface EmptyStudentResultsProps {
  /** Total number of students before filtering */
  readonly totalCount: number;
  /** Number of students after filtering */
  readonly filteredCount: number;
}

/**
 * Empty state displayed when no students match the current filters.
 * Used across OGS groups, active supervisions, and student search pages.
 * Thin wrapper over the kit EmptyState so all three pages share one surface.
 */
export function EmptyStudentResults({
  totalCount,
  filteredCount,
}: Readonly<EmptyStudentResultsProps>) {
  return (
    <EmptyState
      icon={<Search className="h-12 w-12" strokeWidth={2} />}
      title="Keine Kinder gefunden"
      description="Versuchen Sie, Ihre Suchkriterien anzupassen."
      action={
        <p className="text-sm text-gray-500">
          {totalCount} Kinder insgesamt, {filteredCount} nach Filtern
        </p>
      }
    />
  );
}
