type RowStatus = "new" | "existing" | "error" | "warning";

interface DisplayRow {
  row: number;
  status: RowStatus;
  errors: string[];
  first_name: string;
  last_name: string;
  /**
   * Secondary info chips rendered under the name, separated by "•" (e.g.
   * class + group + guardian for students, or role + position for staff).
   * Empty entries are skipped so callers can pass optional fields straight
   * through without pre-filtering.
   */
  meta: string[];
}

interface StudentRowCardProps {
  readonly student: DisplayRow;
  readonly index: number;
}

function getStatusBadge(rowStatus: RowStatus) {
  switch (rowStatus) {
    case "new":
      return (
        <span className="bg-moto-green-soft text-moto-green-strong inline-flex items-center rounded-full px-2 py-1 text-xs font-medium">
          Neu
        </span>
      );
    case "existing":
      return (
        <span className="bg-moto-blue-soft text-moto-blue-strong inline-flex items-center rounded-full px-2 py-1 text-xs font-medium">
          Vorhanden
        </span>
      );
    case "error":
      return (
        <span className="bg-moto-red-soft text-moto-red-strong inline-flex items-center rounded-full px-2 py-1 text-xs font-medium">
          Fehler
        </span>
      );
    case "warning":
      return (
        <span className="bg-moto-amber-soft text-moto-amber-strong inline-flex items-center rounded-full px-2 py-1 text-xs font-medium">
          Warnung
        </span>
      );
  }
}

/**
 * Row card shared by every import preview list (students, staff, ...). Reuse
 * this instead of duplicating the numbered-avatar + status-badge layout: pass
 * the domain-specific secondary fields via `meta`.
 */
export function StudentRowCard({ student, index }: StudentRowCardProps) {
  const metaChips = student.meta.filter((chip) => chip.length > 0);

  return (
    <div className="rounded-xl border border-gray-100 bg-white p-3">
      <div className="flex items-center gap-3">
        <div className="flex h-9 w-9 items-center justify-center rounded-full bg-gray-100 text-xs font-semibold text-gray-600">
          {student.row || index + 1}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h4 className="text-sm font-semibold text-gray-900">
              {student.first_name} {student.last_name}
            </h4>
            {getStatusBadge(student.status)}
          </div>
          {metaChips.length > 0 && (
            <div className="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-gray-500">
              {metaChips.map((chip, i) => (
                <span key={`${i}-${chip}`} className="flex items-center gap-2">
                  {i > 0 && <span aria-hidden="true">•</span>}
                  <span>{chip}</span>
                </span>
              ))}
            </div>
          )}
          {student.errors.length > 0 && (
            <p className="text-moto-red-strong mt-1 text-xs">
              {student.errors.join(", ")}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
