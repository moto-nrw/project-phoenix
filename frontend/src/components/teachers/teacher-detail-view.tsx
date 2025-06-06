import type { Teacher } from "@/lib/teacher-api";

interface TeacherDetailViewProps {
  teacher: Teacher;
  onEdit: () => void;
  onDelete: () => void;
}

export function TeacherDetailView({ teacher, onEdit, onDelete }: TeacherDetailViewProps) {
  return (
    <div className="space-y-6">
      {/* Header with gradient and teacher info */}
      <div className="relative -mx-4 md:-mx-6 -mt-4 md:-mt-6 bg-gradient-to-r from-purple-500 to-indigo-600 p-4 md:p-6 text-white">
        <div className="flex items-center">
          <div className="mr-3 md:mr-5 flex h-16 w-16 md:h-20 md:w-20 items-center justify-center rounded-full bg-white/30 text-2xl md:text-3xl font-bold">
            {teacher.first_name?.[0] ?? ""}
            {teacher.last_name?.[0] ?? ""}
          </div>
          <div>
            <h2 className="text-xl md:text-2xl font-bold">{teacher.name}</h2>
            <p className="text-sm md:text-base opacity-90">{teacher.specialization}</p>
            {teacher.role && (
              <p className="text-xs md:text-sm opacity-75">
                Rolle: {teacher.role}
              </p>
            )}
          </div>
        </div>

        {/* Status badges */}
        <div className="absolute top-4 md:top-6 right-4 md:right-6 flex flex-col space-y-2">
          {teacher.email && (
            <span className="rounded-full bg-blue-400/80 px-2 py-1 text-xs text-white">
              Account vorhanden
            </span>
          )}
          {teacher.tag_id && (
            <span className="rounded-full bg-green-400/80 px-2 py-1 text-xs text-white">
              RFID zugewiesen
            </span>
          )}
        </div>
      </div>

      {/* Action buttons */}
      <div className="flex flex-col sm:flex-row gap-2 sm:justify-end">
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
          <h3 className="border-b border-purple-200 pb-2 text-base md:text-lg font-medium text-purple-800">
            Persönliche Daten
          </h3>

          <div>
            <div className="text-sm text-gray-500">Vorname</div>
            <div className="text-base">{teacher.first_name}</div>
          </div>

          <div>
            <div className="text-sm text-gray-500">Nachname</div>
            <div className="text-base">{teacher.last_name}</div>
          </div>

          {teacher.email && (
            <div>
              <div className="text-sm text-gray-500">E-Mail</div>
              <div className="text-base">{teacher.email}</div>
            </div>
          )}

          {teacher.tag_id && (
            <div>
              <div className="text-sm text-gray-500">RFID-Karte</div>
              <div className="text-base">{teacher.tag_id}</div>
            </div>
          )}
        </div>

        {/* Professional Information */}
        <div className="space-y-4">
          <h3 className="border-b border-purple-200 pb-2 text-base md:text-lg font-medium text-purple-800">
            Berufliche Informationen
          </h3>

          <div>
            <div className="text-sm text-gray-500">Fachgebiet</div>
            <div className="text-base">{teacher.specialization}</div>
          </div>

          {teacher.role && (
            <div>
              <div className="text-sm text-gray-500">Rolle</div>
              <div className="text-base">{teacher.role}</div>
            </div>
          )}

          {teacher.qualifications && (
            <div>
              <div className="text-sm text-gray-500">Qualifikationen</div>
              <div className="text-base">{teacher.qualifications}</div>
            </div>
          )}
        </div>
      </div>

      {/* Additional Information if present */}
      {teacher.staff_notes && (
        <div className="rounded-lg bg-gray-50 p-4">
          <h3 className="mb-2 text-base font-medium text-gray-800">Notizen</h3>
          <p className="text-sm text-gray-600 whitespace-pre-wrap">{teacher.staff_notes}</p>
        </div>
      )}

      {/* Timestamps */}
      <div className="rounded-lg bg-gray-50 p-4">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {teacher.created_at && (
            <div>
              <div className="text-xs text-gray-500">Erstellt am</div>
              <div className="text-sm">
                {new Date(teacher.created_at).toLocaleDateString("de-DE", {
                  day: "2-digit",
                  month: "2-digit",
                  year: "numeric",
                  hour: "2-digit",
                  minute: "2-digit",
                })}
              </div>
            </div>
          )}
          {teacher.updated_at && (
            <div>
              <div className="text-xs text-gray-500">Aktualisiert am</div>
              <div className="text-sm">
                {new Date(teacher.updated_at).toLocaleDateString("de-DE", {
                  day: "2-digit",
                  month: "2-digit",
                  year: "numeric",
                  hour: "2-digit",
                  minute: "2-digit",
                })}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}