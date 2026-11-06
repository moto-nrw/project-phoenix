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
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { TenantPage } from "~/components/ui/tenant-page";
import { formatDate } from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  fetchSharedNotesSchool,
  type SharedNote,
  type StudentSharedNotes,
} from "~/lib/school-class-day-api";
import { schoolClassLabel } from "~/lib/school-class-label";
import { schoolPath } from "~/lib/school-url";
import { IsolatedSWRCache, useSWRAuth } from "~/lib/swr";

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

  return (
    // Each opening (including a change of child) must make its own audited
    // request before disclosing data. Never reuse the portal's notes cache.
    <IsolatedSWRCache key={`${schoolClass}-${studentId}`}>
      <SharedNotes
        schoolClass={schoolClass}
        studentId={studentId}
        dateISO={dateISO}
      />
    </IsolatedSWRCache>
  );
}

function SharedNotes({
  schoolClass,
  studentId,
  dateISO,
}: {
  readonly schoolClass: string;
  readonly studentId: string;
  readonly dateISO: string;
}) {
  const { data: session } = useSession();

  const {
    data: cachedData,
    error,
    isLoading,
    mutate,
  } = useSWRAuth<StudentSharedNotes>(
    session && schoolClass && studentId
      ? `school-shared-notes-${schoolClass}-${studentId}`
      : null,
    async () => {
      try {
        return await fetchSharedNotesSchool(schoolClass, studentId);
      } catch (error) {
        // Clear the previous disclosure before SWR records the load error.
        await mutate(undefined, { revalidate: false });
        throw error;
      }
    },
    { revalidateOnFocus: false, keepPreviousData: false },
  );
  // A failed revalidation must neither display nor retain sensitive data.
  const data = error ? undefined : cachedData;
  const loadError = useSwrLoadError(
    error,
    "die Beobachtungen der OGS",
    () => void mutate(),
  );

  const name = data ? `${data.first_name} ${data.last_name}`.trim() : "";
  const nothingChosen = !schoolClass || !studentId;

  return (
    <TenantPage
      title={name || "Beobachtungen der OGS"}
      back
      backHref={schoolPath(classDayPath(schoolClass, dateISO))}
      backLabel={`Zurück zur ${schoolClassLabel(schoolClass) || "Klasse"}`}
      stats={schoolClassLabel(schoolClass)}
      loading={!nothingChosen && isLoading && !data}
      loadingLabel="Beobachtungen werden geladen…"
      error={loadError}
      empty={
        nothingChosen
          ? {
              title: "Kein Kind ausgewählt",
              description:
                "Öffnen Sie die Beobachtungen über die Klassenseite.",
            }
          : null
      }
    >
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
    </TenantPage>
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
