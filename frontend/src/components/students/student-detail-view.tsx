import { type Student } from "@/lib/api";
import { formatStudentName } from "@/lib/student-helpers";

interface StudentDetailViewProps {
  student: Student;
  onEdit: () => void;
  onDelete: () => void;
}

export function StudentDetailView({ 
  student, 
  onEdit, 
  onDelete 
}: StudentDetailViewProps) {
  return (
    <div className="space-y-6">
      {/* Header with gradient and student info */}
      <div className="relative -mx-6 -mt-6 bg-gradient-to-r from-teal-500 to-blue-600 p-6 text-white">
        <div className="flex items-center">
          <div className="mr-5 flex h-20 w-20 items-center justify-center rounded-full bg-white/30 text-3xl font-bold">
            {student.first_name?.[0] ?? ""}
            {student.second_name?.[0] ?? ""}
          </div>
          <div>
            <h2 className="text-2xl font-bold">{formatStudentName(student)}</h2>
            <p className="opacity-90">{student.school_class}</p>
            {student.group_name && (
              <p className="text-sm opacity-75">
                Gruppe: {student.group_name}
              </p>
            )}
          </div>
        </div>

        {/* Status badges */}
        <div className="absolute top-6 right-6 flex flex-col space-y-2">
          {student.in_house && (
            <span className="rounded-full bg-green-400/80 px-2 py-1 text-xs text-white">
              Im Haus
            </span>
          )}
          {student.wc && (
            <span className="rounded-full bg-blue-400/80 px-2 py-1 text-xs text-white">
              Toilette
            </span>
          )}
          {student.school_yard && (
            <span className="rounded-full bg-yellow-400/80 px-2 py-1 text-xs text-white">
              Schulhof
            </span>
          )}
          {student.bus && (
            <span className="rounded-full bg-orange-400/80 px-2 py-1 text-xs text-white">
              Bus
            </span>
          )}
        </div>
      </div>

      {/* Action buttons */}
      <div className="flex justify-end space-x-2">
        <button
          onClick={onEdit}
          className="rounded-lg bg-blue-50 px-4 py-2 text-blue-600 shadow-sm transition-colors hover:bg-blue-100"
        >
          Bearbeiten
        </button>
        <button
          onClick={onDelete}
          className="rounded-lg bg-red-50 px-4 py-2 text-red-600 shadow-sm transition-colors hover:bg-red-100"
        >
          Löschen
        </button>
      </div>

      {/* Details grid */}
      <div className="grid grid-cols-1 gap-8 md:grid-cols-2">
        {/* Personal Information */}
        <div className="space-y-4">
          <h3 className="border-b border-blue-200 pb-2 text-lg font-medium text-blue-800">
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
              {student.group_id && (
                <span>Gruppe: {student.group_id}</span>
              )}
            </div>
          </div>
        </div>

        {/* Guardian Information and Status */}
        <div className="space-y-8">
          <div className="space-y-4">
            <h3 className="border-b border-purple-200 pb-2 text-lg font-medium text-purple-800">
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
            <h3 className="border-b border-green-200 pb-2 text-lg font-medium text-green-800">
              Status
            </h3>

            <div className="grid grid-cols-2 gap-4">
              <div
                className={`rounded-lg p-3 ${student.in_house ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-500"}`}
              >
                <span className="flex items-center">
                  <span
                    className={`mr-2 inline-block h-3 w-3 rounded-full ${student.in_house ? "bg-green-500" : "bg-gray-300"}`}
                  ></span>
                  Im Haus
                </span>
              </div>

              <div
                className={`rounded-lg p-3 ${student.wc ? "bg-blue-100 text-blue-800" : "bg-gray-100 text-gray-500"}`}
              >
                <span className="flex items-center">
                  <span
                    className={`mr-2 inline-block h-3 w-3 rounded-full ${student.wc ? "bg-blue-500" : "bg-gray-300"}`}
                  ></span>
                  Toilette
                </span>
              </div>

              <div
                className={`rounded-lg p-3 ${student.school_yard ? "bg-yellow-100 text-yellow-800" : "bg-gray-100 text-gray-500"}`}
              >
                <span className="flex items-center">
                  <span
                    className={`mr-2 inline-block h-3 w-3 rounded-full ${student.school_yard ? "bg-yellow-500" : "bg-gray-300"}`}
                  ></span>
                  Schulhof
                </span>
              </div>

              <div
                className={`rounded-lg p-3 ${student.bus ? "bg-orange-100 text-orange-800" : "bg-gray-100 text-gray-500"}`}
              >
                <span className="flex items-center">
                  <span
                    className={`mr-2 inline-block h-3 w-3 rounded-full ${student.bus ? "bg-orange-500" : "bg-gray-300"}`}
                  ></span>
                  Bus
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