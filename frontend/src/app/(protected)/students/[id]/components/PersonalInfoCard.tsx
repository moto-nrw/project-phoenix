// PersonalInfoCard - extracted from student detail page
"use client";

import { InfoItem } from "~/components/ui/info-card";

interface ExtendedStudent {
  name: string;
  school_class: string;
  group_name?: string;
  birthday?: string;
  buskind?: boolean;
  pickup_status?: string;
  health_info?: string;
  supervisor_notes?: string;
  extra_info?: string;
  sick?: boolean;
  sick_since?: string;
}

interface PersonalInfoCardProps {
  readonly student: ExtendedStudent;
  readonly isEditing: boolean;
  readonly hasFullAccess: boolean;
  readonly onEdit: () => void;
}

export function PersonalInfoCard({
  student,
  isEditing,
  hasFullAccess,
  onEdit,
}: PersonalInfoCardProps) {
  return (
    <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
      <div className="mb-4 flex items-center justify-between gap-2">
        <div className="flex min-w-0 flex-1 items-center gap-2 sm:gap-3">
          <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-600 sm:h-10 sm:w-10">
            <PersonIcon />
          </div>
          <h2 className="truncate text-base font-semibold text-gray-900 sm:text-lg">
            Persönliche Informationen
          </h2>
        </div>
        {hasFullAccess && !isEditing ? (
          <button
            onClick={onEdit}
            className="rounded-lg p-2 text-gray-600 transition-colors hover:bg-gray-100"
            title="Bearbeiten"
          >
            <EditIcon />
          </button>
        ) : (
          !hasFullAccess && <ReadOnlyBadge />
        )}
      </div>
      <div className="space-y-3">
        <PersonalInfoDisplay student={student} hasFullAccess={hasFullAccess} />
      </div>
    </div>
  );
}

interface PersonalInfoDisplayProps {
  readonly student: ExtendedStudent;
  readonly hasFullAccess: boolean;
}

function PersonalInfoDisplay({
  student,
  hasFullAccess,
}: PersonalInfoDisplayProps) {
  return (
    <>
      <InfoItem label="Vollständiger Name" value={student.name} />
      <InfoItem label="Klasse" value={student.school_class} />
      <InfoItem
        label="Gruppe"
        value={student.group_name ?? "Nicht zugewiesen"}
      />
      <InfoItem
        label="Geburtsdatum"
        value={
          student.birthday
            ? new Date(student.birthday).toLocaleDateString("de-DE", {
                day: "2-digit",
                month: "2-digit",
                year: "numeric",
              })
            : "Nicht angegeben"
        }
      />
      <InfoItem label="Buskind" value={student.buskind ? "Ja" : "Nein"} />
      <InfoItem
        label="Abholstatus"
        value={student.pickup_status ?? "Nicht gesetzt"}
      />

      {/* Sickness status - only for full access */}
      {hasFullAccess && (
        <InfoItem
          label="Krankheitsstatus"
          value={
            student.sick ? (
              <SickBadge sickSince={student.sick_since} />
            ) : (
              <HealthyBadge />
            )
          }
        />
      )}

      {student.health_info && (
        <InfoItem
          label="Gesundheitsinformationen"
          value={student.health_info}
        />
      )}
      {hasFullAccess && student.supervisor_notes && (
        <InfoItem label="Betreuernotizen" value={student.supervisor_notes} />
      )}
      {hasFullAccess && student.extra_info && (
        <InfoItem label="Elternnotizen" value={student.extra_info} />
      )}
    </>
  );
}

function SickBadge({ sickSince }: Readonly<{ sickSince?: string }>) {
  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
      <span
        className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium text-white"
        style={{ backgroundColor: "#EAB308" }}
      >
        <span className="h-2 w-2 rounded-full bg-white/80" /> <span>Krank</span>
      </span>
      {sickSince && (
        <span className="text-sm text-gray-500">
          seit{" "}
          {new Date(sickSince).toLocaleDateString("de-DE", {
            day: "2-digit",
            month: "2-digit",
            year: "numeric",
          })}
        </span>
      )}
    </div>
  );
}

function HealthyBadge() {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-green-100 px-2.5 py-1 text-xs font-medium text-green-800">
      <span className="h-2 w-2 rounded-full bg-green-500" />{" "}
      <span>Nicht krankgemeldet</span>
    </span>
  );
}

function ReadOnlyBadge() {
  return (
    <span className="inline-flex flex-shrink-0 items-center gap-1 rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 sm:px-2.5">
      <EyeIcon />
      <span className="hidden sm:inline">Nur Ansicht</span>
      <span className="sm:hidden">Ansicht</span>
    </span>
  );
}

// Icons
function PersonIcon() {
  return (
    <svg
      className="h-5 w-5"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
      />
    </svg>
  );
}

function EditIcon() {
  return (
    <svg
      className="h-5 w-5"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
      />
    </svg>
  );
}

function EyeIcon() {
  return (
    <svg
      className="h-3 w-3 sm:h-3.5 sm:w-3.5"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
      />
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
      />
    </svg>
  );
}
