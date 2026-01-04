// components/ui/empty-student-results.tsx
// Shared empty state component for student search/filter results

interface EmptyStudentResultsProps {
  /** Total number of students before filtering */
  readonly totalCount: number;
  /** Number of students after filtering */
  readonly filteredCount: number;
}

/**
 * Empty state displayed when no students match the current filters.
 * Used across OGS groups, active supervisions, and student search pages.
 */
export function EmptyStudentResults({
  totalCount,
  filteredCount,
}: Readonly<EmptyStudentResultsProps>) {
  return (
    <div className="py-12 text-center">
      <div className="flex flex-col items-center gap-4">
        <svg
          className="h-12 w-12 text-gray-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
          />
        </svg>
        <div>
          <h3 className="text-lg font-medium text-gray-900">
            Keine Schüler gefunden
          </h3>
          <p className="text-gray-600">
            Versuche deine Suchkriterien anzupassen.
          </p>
          <p className="mt-2 text-sm text-gray-500">
            {totalCount} Schüler insgesamt, {filteredCount} nach Filtern
          </p>
        </div>
      </div>
    </div>
  );
}
