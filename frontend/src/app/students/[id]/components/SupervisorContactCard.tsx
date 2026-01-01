// SupervisorContactCard - extracted from student detail page
"use client";

import type { SupervisorContact } from "~/lib/student-helpers";

interface SupervisorContactCardProps {
  readonly supervisors: SupervisorContact[];
  readonly studentName: string;
}

export function SupervisorContactCard({
  supervisors,
  studentName,
}: SupervisorContactCardProps) {
  if (supervisors.length === 0) return null;

  return (
    <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
      <div className="mb-4 flex items-center justify-between gap-2">
        <div className="flex min-w-0 flex-1 items-center gap-2 sm:gap-3">
          <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-600 sm:h-10 sm:w-10">
            <ContactIcon />
          </div>
          <h2 className="truncate text-base font-semibold text-gray-900 sm:text-lg">
            Ansprechpartner
          </h2>
        </div>
        <ReadOnlyBadge />
      </div>

      <div className="space-y-4">
        {supervisors.map((supervisor, index) => (
          <SupervisorItem
            key={supervisor.id}
            supervisor={supervisor}
            studentName={studentName}
            showDivider={index > 0}
          />
        ))}
      </div>
    </div>
  );
}

interface SupervisorItemProps {
  readonly supervisor: SupervisorContact;
  readonly studentName: string;
  readonly showDivider: boolean;
}

function SupervisorItem({
  supervisor,
  studentName,
  showDivider,
}: SupervisorItemProps) {
  return (
    <div>
      {showDivider && <div className="my-4 border-t border-gray-100" />}
      <div>
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-base font-semibold text-gray-900">
            {supervisor.first_name} {supervisor.last_name}
          </p>
          <span className="inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-700">
            Gruppenleitung
          </span>
        </div>
        {supervisor.email && (
          <p className="mt-1 text-sm text-gray-500">{supervisor.email}</p>
        )}
        {supervisor.email && (
          <button
            onClick={() => {
              window.location.href = `mailto:${supervisor.email}?subject=Anfrage zu ${studentName}`;
            }}
            className="mt-3 inline-flex items-center gap-2 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-[0.98]"
          >
            <MailIcon />
            Kontakt aufnehmen
          </button>
        )}
      </div>
    </div>
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
function ContactIcon() {
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
        d="M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2"
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

function MailIcon() {
  return (
    <svg
      className="h-4 w-4"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
      />
    </svg>
  );
}
