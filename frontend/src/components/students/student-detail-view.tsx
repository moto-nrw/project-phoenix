import { type Student } from "@/lib/api";
import { formatStudentName } from "@/lib/student-helpers";
import {
  isHomeLocation,
  isPresentLocation,
  isSchoolyardLocation,
  isTransitLocation,
} from "@/lib/location-helper";

/**
 * Formats the location_since timestamp for display.
 * Shows only the time (HH:MM) since it's for "current" location.
 */
function formatLocationSince(isoTimestamp: string | undefined): string | null {
  if (!isoTimestamp) return null;

  try {
    const date = new Date(isoTimestamp);
    if (isNaN(date.getTime())) return null;

    return date.toLocaleTimeString("de-DE", {
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return null;
  }
}

interface StudentDetailViewProps {
  student: Student;
  onEdit: () => void;
  onDelete: () => void;
}

export function StudentDetailView({
  student,
  onEdit,
  onDelete,
}: StudentDetailViewProps) {
  const isPresent = isPresentLocation(student.current_location);
  const isSchoolyard = isSchoolyardLocation(student.current_location);
  const isTransit = isTransitLocation(student.current_location);
  const isHome = isHomeLocation(student.current_location);

  return (
    <div className="space-y-6">
      {/* Header with gradient and student info */}
      <div className="relative -mx-4 -mt-4 bg-gradient-to-r from-teal-500 to-blue-600 p-4 text-white md:-mx-6 md:-mt-6 md:p-6">
        <div className="flex items-center">
          <div className="mr-3 flex h-16 w-16 items-center justify-center rounded-full bg-white/30 text-2xl font-bold md:mr-5 md:h-20 md:w-20 md:text-3xl">
            {student.first_name?.[0] ?? ""}
            {student.second_name?.[0] ?? ""}
          </div>
          <div>
            <h2 className="text-xl font-bold md:text-2xl">
              {formatStudentName(student)}
            </h2>
            <p className="text-sm opacity-90 md:text-base">
              {student.school_class}
            </p>
            {student.group_name && (
              <p className="text-xs opacity-75 md:text-sm">
                Gruppe: {student.group_name}
              </p>
            )}
          </div>
        </div>

        {/* Status badges */}
        <div className="absolute top-4 right-4 flex flex-col items-end space-y-2 md:top-6 md:right-6">
          {isPresent && (
            <div className="flex flex-col items-end">
              <span className="rounded-full bg-green-400/80 px-2 py-1 text-xs text-white">
                Im Haus
              </span>
              {formatLocationSince(student.location_since) && (
                <span className="mt-0.5 text-[10px] text-white/80">
                  seit {formatLocationSince(student.location_since)} Uhr
                </span>
              )}
            </div>
          )}
          {isTransit && (
            <span className="rounded-full bg-fuchsia-400/80 px-2 py-1 text-xs text-white">
              Unterwegs
            </span>
          )}
          {isSchoolyard && (
            <span className="rounded-full bg-yellow-400/80 px-2 py-1 text-xs text-white">
              Schulhof
            </span>
          )}
          {isHome && (
            <div className="flex flex-col items-end">
              <span className="rounded-full bg-red-400/80 px-2 py-1 text-xs text-white">
                Zuhause
              </span>
              {formatLocationSince(student.location_since) && (
                <span className="mt-0.5 text-[10px] text-white/80">
                  seit {formatLocationSince(student.location_since)} Uhr
                </span>
              )}
            </div>
          )}
          {student.bus && (
            <span className="rounded-full bg-orange-400/80 px-2 py-1 text-xs text-white">
              Bus
            </span>
          )}
        </div>
      </div>

      {/* Action buttons */}
      <div className="flex flex-col gap-2 sm:flex-row sm:justify-end">
        <button
          onClick={onEdit}
          className="min-h-[44px] rounded-lg bg-gradient-to-r from-blue-500 to-blue-600 px-6 py-2 text-sm font-medium text-white shadow-sm transition-all duration-200 hover:from-blue-600 hover:to-blue-700 hover:shadow-md active:scale-[0.98]"
        >
          Bearbeiten
        </button>
        <button
          onClick={onDelete}
          className="min-h-[44px] rounded-lg border border-red-300 bg-white px-4 py-2 text-sm font-medium text-red-600 shadow-sm transition-all duration-200 hover:bg-red-50 active:scale-[0.98]"
        >
          Löschen
        </button>
      </div>

      {/* Details grid */}
      <div className="grid grid-cols-1 gap-8 md:grid-cols-2">
        {/* Personal Information */}
        <div className="space-y-4">
          <h3 className="border-b border-blue-200 pb-2 text-base font-medium text-blue-800 md:text-lg">
            Persönliche Daten
          </h3>

          <div>
            <div className="text-sm text-gray-500">Vorname</div>
            <div className="text-base">{student.first_name}</div>
          </div>

          <div>
            <div className="text-sm text-gray-500">Nachname</div>
            <div className="text-base">{student.second_name}</div>
          </div>

          <div>
            <div className="text-sm text-gray-500">Klasse</div>
            <div className="text-base">{student.school_class}</div>
          </div>

          <div>
            <div className="text-sm text-gray-500">Gruppe</div>
            <div className="text-base">
              {student.group_name ?? "Keine Gruppe zugewiesen"}
            </div>
          </div>

          <div>
            <div className="text-sm text-gray-500">IDs</div>
            <div className="flex flex-col text-xs text-gray-600">
              <span>Student: {student.id}</span>
              {student.custom_users_id && (
                <span>Benutzer: {student.custom_users_id}</span>
              )}
              {student.group_id && <span>Gruppe: {student.group_id}</span>}
            </div>
          </div>
        </div>

        {/* Guardian Information and Status */}
        <div className="space-y-8">
          <div className="space-y-4">
            <h3 className="border-b border-purple-200 pb-2 text-base font-medium text-purple-800 md:text-lg">
              Erziehungsberechtigte
            </h3>

            <div>
              <div className="text-sm text-gray-500">Name</div>
              <div className="text-base">
                {student.name_lg ?? "Nicht angegeben"}
              </div>
            </div>

            <div>
              <div className="text-sm text-gray-500">Kontakt</div>
              <div className="text-base">
                {student.contact_lg ?? "Nicht angegeben"}
              </div>
            </div>
          </div>

          <div className="space-y-4">
            <h3 className="border-b border-green-200 pb-2 text-base font-medium text-green-800 md:text-lg">
              Status
            </h3>

            <div className="grid grid-cols-2 gap-2 md:gap-4">
              <div
                className={`rounded-lg p-2 text-sm md:p-3 ${isPresent ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-500"}`}
              >
                <span className="flex items-center">
                  <span
                    className={`mr-2 inline-block h-3 w-3 flex-shrink-0 rounded-full ${isPresent ? "bg-green-500" : "bg-gray-300"}`}
                  ></span>
                  <span className="truncate">Im Haus</span>
                </span>
              </div>

              <div
                className={`rounded-lg p-2 text-sm md:p-3 ${isTransit ? "bg-fuchsia-100 text-fuchsia-800" : "bg-gray-100 text-gray-500"}`}
              >
                <span className="flex items-center">
                  <span
                    className={`mr-2 inline-block h-3 w-3 flex-shrink-0 rounded-full ${isTransit ? "bg-fuchsia-500" : "bg-gray-300"}`}
                  ></span>
                  <span className="truncate">Unterwegs</span>
                </span>
              </div>

              <div
                className={`rounded-lg p-2 text-sm md:p-3 ${isSchoolyard ? "bg-yellow-100 text-yellow-800" : "bg-gray-100 text-gray-500"}`}
              >
                <span className="flex items-center">
                  <span
                    className={`mr-2 inline-block h-3 w-3 flex-shrink-0 rounded-full ${isSchoolyard ? "bg-yellow-500" : "bg-gray-300"}`}
                  ></span>
                  <span className="truncate">Schulhof</span>
                </span>
              </div>

              <div
                className={`rounded-lg p-2 text-sm md:p-3 ${student.bus ? "bg-orange-100 text-orange-800" : "bg-gray-100 text-gray-500"}`}
              >
                <span className="flex items-center">
                  <span
                    className={`mr-2 inline-block h-3 w-3 flex-shrink-0 rounded-full ${student.bus ? "bg-orange-500" : "bg-gray-300"}`}
                  ></span>
                  <span className="truncate">Bus</span>
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default StudentDetailView;
