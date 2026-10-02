"use client";

import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  NOTE_KIND_PERMANENT,
  NOTE_ORIGIN_MASTER_DATA,
  NOTE_VISIBILITY_ALL_STAFF,
  studentNotesService,
  type StudentNote,
} from "~/lib/student-notes-api";
import { useSWRAuth } from "~/lib/swr";

// The durable hints of one child on the Stammdaten tab (#3632). They replace
// the former free-text field "Betreuernotizen": same place, same purpose, but
// each hint now carries who wrote it and who may read it.
//
// Read-only on purpose. Writing happens where the chronicle is, so there is
// one place that owns a note and one form that writes it; this card carries no
// button, no chevron and no pointer, per the Missverständnis-Check.

const VISIBILITY_LABELS: Record<string, string> = {
  care_team: "Betreuungsteam",
  group_leads: "Gruppenleitung",
};

function HintRow({ note }: { readonly note: StudentNote }) {
  const author =
    note.origin === NOTE_ORIGIN_MASTER_DATA
      ? "Übernommen aus den Betreuernotizen"
      : note.authorName;
  return (
    <li className="space-y-1">
      <p className="text-sm whitespace-pre-line text-gray-900">{note.body}</p>
      <div className="flex flex-wrap items-center gap-2">
        {author !== "" ? (
          <span className="text-xs text-gray-500">{author}</span>
        ) : null}
        {note.visibility !== NOTE_VISIBILITY_ALL_STAFF ? (
          <StatusBadge
            compact
            tone="blue"
            label={VISIBILITY_LABELS[note.visibility] ?? note.visibility}
            accessibleLabel={`Sichtbar für: ${VISIBILITY_LABELS[note.visibility] ?? note.visibility}`}
          />
        ) : null}
      </div>
    </li>
  );
}

export function StudentPermanentNotesCard({
  studentId,
}: {
  readonly studentId: string;
}) {
  const { data, isLoading, error } = useSWRAuth<StudentNote[]>(
    `student-permanent-notes-${studentId}`,
    () => studentNotesService.list(studentId, NOTE_KIND_PERMANENT),
  );

  return (
    <SectionCard
      title="Dauerhafte Hinweise"
      description="Was für dieses Kind immer gilt. Neue Hinweise schreiben Sie im Reiter Notizen."
    >
      {isLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-4 w-1/2" />
        </div>
      ) : null}

      {error ? (
        <Alert
          type="error"
          message="Die Hinweise konnten nicht geladen werden. Bitte laden Sie die Seite neu."
        />
      ) : null}

      {!isLoading && !error ? (
        (data ?? []).length === 0 ? (
          <EmptyState
            variant="compact"
            title="Keine dauerhaften Hinweise"
            description="Im Reiter Notizen können Sie einen anlegen."
          />
        ) : (
          <ul className="space-y-4">
            {(data ?? []).map((note) => (
              <HintRow key={note.id} note={note} />
            ))}
          </ul>
        )
      ) : null}
    </SectionCard>
  );
}
