type RowStatus = "new" | "existing" | "error" | "warning";

interface DisplayStudent {
  row: number;
  status: RowStatus;
  errors: string[];
  first_name: string;
  last_name: string;
  school_class: string;
  group_name: string;
  guardian_info: string;
  health_info: string;
}

interface StudentRowCardProps {
  readonly student: DisplayStudent;
  readonly index: number;
}

function getStatusBadge(rowStatus: RowStatus) {
  switch (rowStatus) {
    case "new":
      return (
        <span className="inline-flex items-center rounded-full bg-green-100 px-2 py-1 text-xs font-medium text-green-700">
          Neu
        </span>
      );
    case "existing":
      return (
        <span className="inline-flex items-center rounded-full bg-blue-100 px-2 py-1 text-xs font-medium text-blue-700">
          Vorhanden
        </span>
      );
    case "error":
      return (
        <span className="inline-flex items-center rounded-full bg-red-100 px-2 py-1 text-xs font-medium text-red-700">
          Fehler
        </span>
      );
    case "warning":
      return (
        <span className="inline-flex items-center rounded-full bg-amber-100 px-2 py-1 text-xs font-medium text-amber-700">
          Warnung
        </span>
      );
  }
}

export function StudentRowCard({ student, index }: StudentRowCardProps) {
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
          <div className="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-gray-500">
            {student.school_class && <span>{student.school_class}</span>}
            {student.group_name && (
              <>
                <span>•</span>
                <span>{student.group_name}</span>
              </>
            )}
            {student.guardian_info && (
              <>
                <span>•</span>
                <span>{student.guardian_info}</span>
              </>
            )}
          </div>
          {student.errors.length > 0 && (
            <p className="mt-1 text-xs text-red-600">
              {student.errors.join(", ")}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
