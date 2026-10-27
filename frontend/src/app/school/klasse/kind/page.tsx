"use client";

// Beobachtungen der OGS zu einem Kind in moto schule (#4797): die Notizen,
// die die OGS für die Schule freigegeben hat. Nur die Lehrkraft der Klasse
// bekommt sie (education.class_teachers); die Schulverwaltung nicht. Die
// Lehrkraft liest nur. Jeder Abruf wird im Backend protokolliert. Klasse,
// Kind und Tag stehen als Query-Parameter in der Adresse, wie auf der
// Klassenseite (`CLASS_DAY_CLASS_PARAM`).

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";

import {
  CLASS_DAY_CHILD_PARAM,
  CLASS_DAY_CLASS_PARAM,
  CLASS_DAY_DATE_PARAM,
  classDayClassParam,
  classDayDateParam,
  classDayPath,
} from "~/components/class-day/routes";
import { BackLink } from "~/components/ui/back-link";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { formatDate } from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  fetchSharedNotesSchool,
  type SharedNote,
  type StudentSharedNotes,
} from "~/lib/school-class-day-api";
import { schoolClassLabel } from "~/lib/school-class-label";
import { schoolPath } from "~/lib/school-url";
import { useSWRAuth } from "~/lib/swr";

/** Wann und von wem: der beschriebene Tag oder „Dauerhafter Hinweis“. */
function noteMeta(note: SharedNote): string {
  const parts = [
    note.subject_date ? formatDate(note.subject_date) : "Dauerhafter Hinweis",
  ];
  if (note.author_name) parts.push(note.author_name);
  if (note.edited) parts.push("geändert");
  return parts.join(" · ");
}

function SharedNotesFromAddress() {
  const params = useSearchParams();
  const schoolClass = classDayClassParam(params.get(CLASS_DAY_CLASS_PARAM));
  const studentId = params.get(CLASS_DAY_CHILD_PARAM)?.trim() ?? "";
  const dateISO = classDayDateParam(params.get(CLASS_DAY_DATE_PARAM));
  const { data: session } = useSession();

  const { data, error, isLoading, mutate } = useSWRAuth<StudentSharedNotes>(
    session && schoolClass && studentId
      ? `school-shared-notes-${schoolClass}-${studentId}`
      : null,
    () => fetchSharedNotesSchool(schoolClass, studentId),
    { revalidateOnFocus: false },
  );
  const loadError = useSwrLoadError(
    error,
    "die Beobachtungen der OGS",
    () => void mutate(),
  );

  const back = (
    <BackLink
      href={schoolPath(classDayPath(schoolClass, dateISO))}
      label={`Zurück zur ${schoolClassLabel(schoolClass) || "Klasse"}`}
    />
  );
  const name = data ? `${data.first_name} ${data.last_name}`.trim() : "";

  return (
    <div className="-mt-1.5 w-full space-y-6">
      {back}
      <div>
        <h1 className="text-xl font-semibold break-words text-gray-900 sm:text-2xl">
          {name || "Beobachtungen der OGS"}
        </h1>
        <p className="mt-1 text-sm text-gray-500">
          {schoolClassLabel(schoolClass)}
        </p>
      </div>

      {loadError ? <LoadErrorAlert error={loadError} /> : null}

      {!schoolClass || !studentId ? (
        <EmptyState
          title="Kein Kind ausgewählt"
          description="Öffnen Sie die Beobachtungen über die Klassenseite."
        />
      ) : null}

      {isLoading && !data ? (
        <SkeletonRegion label="Beobachtungen werden geladen…">
          <ListSkeleton rows={3} avatar={false} />
        </SkeletonRegion>
      ) : null}

      {data ? (
        <SectionCard
          title="Beobachtungen der OGS"
          description="Die OGS hat diese Notizen für Sie freigegeben. Sie können sie nur lesen."
        >
          {data.notes.length === 0 ? (
            <EmptyState
              variant="compact"
              title="Keine Beobachtungen"
              description="Die OGS hat für dieses Kind noch nichts freigegeben."
            />
          ) : (
            <ul className="space-y-4">
              {data.notes.map((note) => (
                <li key={note.id} className="space-y-1">
                  <p className="text-sm whitespace-pre-line text-gray-900">
                    {note.body}
                  </p>
                  <p className="text-xs text-gray-500">{noteMeta(note)}</p>
                </li>
              ))}
            </ul>
          )}
        </SectionCard>
      ) : null}
    </div>
  );
}

export default function SchoolSharedNotesPage() {
  return (
    // useSearchParams (Klasse, Kind und Tag stehen in der Adresse) braucht
    // eine Suspense-Grenze.
    <Suspense fallback={<Skeleton className="h-64 w-full" />}>
      <SharedNotesFromAddress />
    </Suspense>
  );
}
