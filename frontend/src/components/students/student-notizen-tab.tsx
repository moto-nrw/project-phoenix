"use client";

import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { NotebookPen } from "lucide-react";

import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { EmptyState } from "~/components/ui/empty-state";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Loading } from "~/components/ui/loading";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  NOTE_KIND_JOURNAL,
  NOTE_KIND_PERMANENT,
  NOTE_ORIGIN_MASTER_DATA,
  NOTE_ORIGIN_SCHOOL,
  NOTE_SHARED_WITH_SCHOOL_BADGE,
  NOTE_VISIBILITY_ALL_STAFF,
  NOTE_VISIBILITY_CARE_TEAM,
  NOTE_VISIBILITY_GROUP_LEADS,
  studentNotesService,
  type StudentNote,
  type StudentNoteActivity,
  type StudentNoteDraft,
} from "~/lib/student-notes-api";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";

// Notizen tab (#3632): the chronicle of one child. The backend resolves who
// may read which note and sends can_edit / can_delete with every entry, so
// this tab renders exactly the authority it was given and computes none of
// its own.

const logger = createLogger({ component: "StudentNotizenTab" });

const MAX_BODY_LENGTH = 4000;

const KIND_ITEMS = [
  { value: NOTE_KIND_JOURNAL, label: "Eintrag" },
  { value: NOTE_KIND_PERMANENT, label: "Dauerhafter Hinweis" },
];

const VISIBILITY_OPTIONS = [
  { value: NOTE_VISIBILITY_ALL_STAFF, label: "Ganzes Team" },
  { value: NOTE_VISIBILITY_CARE_TEAM, label: "Betreuungsteam des Kindes" },
];

const VISIBILITY_LABELS: Record<string, string> = {
  [NOTE_VISIBILITY_ALL_STAFF]: "Ganzes Team",
  [NOTE_VISIBILITY_CARE_TEAM]: "Betreuungsteam",
};

// The leadership audience is the leadership of what the note refers to: the
// supervisors of an activity, or the leads of the OGS group.
const ACTIVITY_LEADS_LABEL = "Angebotsleitung";
const GROUP_LEADS_LABEL = "Gruppenleitung";

/** The reference picker encodes "nothing", a group or an activity in one value. */
const REFERENCE_NONE = "";
const REFERENCE_GROUP_PREFIX = "group:";
const REFERENCE_ACTIVITY_PREFIX = "activity:";

function referenceValue(draft: StudentNoteDraft): string {
  if (draft.activityGroupId) {
    return `${REFERENCE_ACTIVITY_PREFIX}${draft.activityGroupId}`;
  }
  if (draft.educationGroupId) {
    return `${REFERENCE_GROUP_PREFIX}${draft.educationGroupId}`;
  }
  return REFERENCE_NONE;
}

function withReference(
  draft: StudentNoteDraft,
  value: string,
): StudentNoteDraft {
  const activityGroupId = value.startsWith(REFERENCE_ACTIVITY_PREFIX)
    ? value.slice(REFERENCE_ACTIVITY_PREFIX.length)
    : undefined;
  const educationGroupId = value.startsWith(REFERENCE_GROUP_PREFIX)
    ? value.slice(REFERENCE_GROUP_PREFIX.length)
    : undefined;
  const next = { ...draft, activityGroupId, educationGroupId };
  // Without a reference there is no leadership to address. Fall back to the
  // next narrower audience, never silently to the whole team.
  if (
    activityGroupId == null &&
    educationGroupId == null &&
    draft.visibility === NOTE_VISIBILITY_GROUP_LEADS
  ) {
    next.visibility = NOTE_VISIBILITY_CARE_TEAM;
  }
  return next;
}

const CATEGORY_OPTIONS = [
  { value: "", label: "Ohne Thema" },
  { value: "general", label: "Allgemein" },
  { value: "behaviour", label: "Verhalten" },
  { value: "conversation", label: "Gespräch" },
  { value: "incident", label: "Vorfall" },
  { value: "positive", label: "Erfreuliches" },
  { value: "parent_contact", label: "Elternkontakt" },
];

const CATEGORY_LABELS: Record<string, string> = Object.fromEntries(
  CATEGORY_OPTIONS.filter((option) => option.value !== "").map((option) => [
    option.value,
    option.label,
  ]),
);

// The leadership audience only means something when the note points at a
// group or an activity, and it names whose leadership that is.
function visibilityOptionsFor(reference: {
  activityGroupId?: string;
  educationGroupId?: string;
}): readonly { value: string; label: string }[] {
  if (reference.activityGroupId) {
    return [
      ...VISIBILITY_OPTIONS,
      { value: NOTE_VISIBILITY_GROUP_LEADS, label: ACTIVITY_LEADS_LABEL },
    ];
  }
  if (reference.educationGroupId) {
    return [
      ...VISIBILITY_OPTIONS,
      { value: NOTE_VISIBILITY_GROUP_LEADS, label: GROUP_LEADS_LABEL },
    ];
  }
  return VISIBILITY_OPTIONS;
}

function visibilityLabel(note: StudentNote): string {
  if (note.visibility === NOTE_VISIBILITY_GROUP_LEADS) {
    return note.activityGroupId !== ""
      ? ACTIVITY_LEADS_LABEL
      : GROUP_LEADS_LABEL;
  }
  return VISIBILITY_LABELS[note.visibility] ?? note.visibility;
}

/** What the note is about besides the child, named for the chronicle. */
interface ReferenceNames {
  readonly activities: readonly StudentNoteActivity[];
  readonly educationGroupId: string;
  readonly educationGroupName: string;
}

function referenceLabel(note: StudentNote, names: ReferenceNames): string {
  if (note.activityGroupId !== "") {
    const activity = names.activities.find(
      (candidate) => candidate.id === note.activityGroupId,
    );
    // An activity the child no longer attends keeps its reference, only the
    // name is not part of today's list.
    return activity ? `Angebot ${activity.name}` : "Angebot";
  }
  if (note.educationGroupId !== "") {
    return note.educationGroupId === names.educationGroupId &&
      names.educationGroupName !== ""
      ? `OGS-Gruppe ${names.educationGroupName}`
      : "OGS-Gruppe";
  }
  return "";
}

function referenceOptions(
  names: ReferenceNames,
): readonly { value: string; label: string }[] {
  const options = [{ value: REFERENCE_NONE, label: "Nur das Kind" }];
  if (names.educationGroupId !== "") {
    options.push({
      value: `${REFERENCE_GROUP_PREFIX}${names.educationGroupId}`,
      label:
        names.educationGroupName !== ""
          ? `OGS-Gruppe ${names.educationGroupName}`
          : "OGS-Gruppe",
    });
  }
  for (const activity of names.activities) {
    options.push({
      value: `${REFERENCE_ACTIVITY_PREFIX}${activity.id}`,
      label: `Angebot ${activity.name}`,
    });
  }
  return options;
}

function emptyDraft(): StudentNoteDraft {
  return {
    kind: NOTE_KIND_JOURNAL,
    visibility: NOTE_VISIBILITY_ALL_STAFF,
    category: "",
    body: "",
    subjectDate: berlinTodayISO(),
  };
}

function monthLabel(isoDate: string): string {
  const [year, month] = isoDate.split("-");
  const names = [
    "Januar",
    "Februar",
    "März",
    "April",
    "Mai",
    "Juni",
    "Juli",
    "August",
    "September",
    "Oktober",
    "November",
    "Dezember",
  ];
  const index = Number(month) - 1;
  return names[index] != null ? `${names[index]} ${year}` : (year ?? "");
}

/** The day an entry belongs to: the day it describes, else the day it was written. */
function noteDay(note: StudentNote): string {
  return note.subjectDate !== ""
    ? note.subjectDate
    : berlinTodayISO(new Date(note.createdAt));
}

function NoteMeta({
  note,
  names,
}: {
  readonly note: StudentNote;
  readonly names: ReferenceNames;
}) {
  const parts: string[] = [];
  if (note.subjectDate !== "") {
    parts.push(formatDate(note.subjectDate));
  }
  const reference = referenceLabel(note, names);
  if (reference !== "") {
    parts.push(reference);
  }
  if (note.origin === NOTE_ORIGIN_MASTER_DATA) {
    parts.push("Übernommen aus den Betreuernotizen");
  } else if (note.origin === NOTE_ORIGIN_SCHOOL) {
    // Geschrieben in moto schule; die OGS ändert ihn nicht (#4795).
    parts.push(
      note.authorName !== ""
        ? `Hinweis der Lehrkraft ${note.authorName}`
        : "Hinweis der Lehrkraft",
    );
  } else if (note.authorName !== "") {
    parts.push(note.authorName);
  }
  if (note.edited) {
    parts.push("geändert");
  }
  return (
    <div className="flex flex-wrap items-center gap-2">
      {parts.length > 0 ? (
        <span className="text-xs text-gray-500">{parts.join(" · ")}</span>
      ) : null}
      {note.category !== "" && CATEGORY_LABELS[note.category] != null ? (
        <StatusBadge
          compact
          tone="gray"
          label={CATEGORY_LABELS[note.category] ?? note.category}
        />
      ) : null}
      {note.visibility !== NOTE_VISIBILITY_ALL_STAFF ? (
        <StatusBadge
          compact
          tone="blue"
          label={visibilityLabel(note)}
          accessibleLabel={`Sichtbar für: ${visibilityLabel(note)}`}
        />
      ) : null}
      {/* Die Lehrkraft der Klasse liest diese Notiz (#4797). */}
      {note.sharedWithSchool ? (
        <StatusBadge {...NOTE_SHARED_WITH_SCHOOL_BADGE} />
      ) : null}
    </div>
  );
}

function NoteForm({
  draft,
  onChange,
  onCancel,
  onSave,
  saving,
  visibilityOptions,
  referenceChoices,
  referenceHint,
  idPrefix,
  fieldError,
}: {
  readonly fieldError: (name: string) => string | undefined;
  /** Omitted while editing: a note keeps the reference it was written with. */
  readonly referenceChoices?: readonly { value: string; label: string }[];
  readonly referenceHint?: string;
  readonly draft: StudentNoteDraft;
  readonly onChange: (draft: StudentNoteDraft) => void;
  readonly onCancel: () => void;
  readonly onSave: () => void;
  readonly saving: boolean;
  readonly visibilityOptions: readonly { value: string; label: string }[];
  readonly idPrefix: string;
}) {
  const tooLong = draft.body.length > MAX_BODY_LENGTH;
  const sharedWithSchool = draft.sharedWithSchool ?? false;
  return (
    <div className="space-y-4">
      <Textarea
        id={`${idPrefix}-body`}
        name="body"
        label="Notiz"
        rows={4}
        value={draft.body}
        maxLength={MAX_BODY_LENGTH + 100}
        placeholder="Was ist passiert? Kurz und sachlich."
        onChange={(event) => onChange({ ...draft, body: event.target.value })}
        error={
          tooLong ? "Die Notiz ist zu lang. Bitte kürzen." : fieldError("body")
        }
      />
      <div className="grid gap-4 sm:grid-cols-2">
        {draft.kind === NOTE_KIND_PERMANENT ? null : (
          <div>
            <ISODatePicker
              label="Tag"
              value={draft.subjectDate ?? ""}
              max={berlinTodayISO()}
              onChange={(subjectDate) => onChange({ ...draft, subjectDate })}
            />
          </div>
        )}
        <div>
          <span className="mb-2 block text-sm font-medium text-gray-700">
            Art
          </span>
          <SegmentedControl
            items={KIND_ITEMS}
            value={draft.kind}
            onChange={(kind) =>
              onChange({
                ...draft,
                kind,
                // Ein dauerhafter Hinweis gilt ohne Tag.
                subjectDate:
                  kind === NOTE_KIND_PERMANENT ? undefined : berlinTodayISO(),
              })
            }
            fullWidth
          />
        </div>
        {referenceChoices ? (
          <div>
            <label
              htmlFor={`${idPrefix}-reference`}
              className="mb-2 block text-sm font-medium text-gray-700"
            >
              Bezug
            </label>
            <CustomSelect
              id={`${idPrefix}-reference`}
              value={referenceValue(draft)}
              options={referenceChoices}
              onChange={(value) => onChange(withReference(draft, value))}
            />
            {referenceHint ? (
              <p className="mt-1 text-xs text-gray-500">{referenceHint}</p>
            ) : null}
          </div>
        ) : null}
        <div>
          <label
            htmlFor={`${idPrefix}-visibility`}
            className="mb-2 block text-sm font-medium text-gray-700"
          >
            Wer sieht die Notiz?
          </label>
          <CustomSelect
            id={`${idPrefix}-visibility`}
            value={draft.visibility}
            options={visibilityOptions}
            onChange={(visibility) => onChange({ ...draft, visibility })}
          />
        </div>
        <div>
          <label
            htmlFor={`${idPrefix}-category`}
            className="mb-2 block text-sm font-medium text-gray-700"
          >
            Thema
          </label>
          <CustomSelect
            id={`${idPrefix}-category`}
            value={draft.category ?? ""}
            options={CATEGORY_OPTIONS}
            onChange={(category) => onChange({ ...draft, category })}
          />
        </div>
      </div>
      {/* Freigabe für die Schule (#4797): standardmäßig aus. */}
      <ChoiceTile selected={sharedWithSchool} className="items-start p-3">
        <Checkbox
          id={`${idPrefix}-shared-with-school`}
          checked={sharedWithSchool}
          onChange={(event) =>
            onChange({ ...draft, sharedWithSchool: event.target.checked })
          }
        />
        <span className="min-w-0">
          <span className="block text-sm font-medium text-gray-900">
            Für die Schule freigeben
          </span>
          <span className="block text-xs text-gray-500">
            Die Lehrkraft der Klasse liest die Notiz in moto schule. Sie kann
            sie nicht ändern.
          </span>
        </span>
      </ChoiceTile>
      <EditActions
        onCancel={onCancel}
        onSave={onSave}
        saving={saving}
        disabled={draft.body.trim() === "" || tooLong}
      />
    </div>
  );
}

export function StudentNotizenTab({
  studentId,
  educationGroupId = "",
  educationGroupName = "",
}: {
  readonly studentId: string;
  /** The child's OGS group, so a note can refer to it. */
  readonly educationGroupId?: string;
  readonly educationGroupName?: string;
}) {
  const { data, isLoading, error, mutate } = useSWRAuth<StudentNote[]>(
    `student-notes-${studentId}`,
    () => studentNotesService.list(studentId),
  );
  // The activities name the references in the chronicle and fill the picker.
  // Without them the card still works; only the activity names are missing.
  const { data: activityData, error: activityError } = useSWRAuth<
    StudentNoteActivity[]
  >(`student-note-activities-${studentId}`, () =>
    studentNotesService.listActivities(studentId),
  );
  useEffect(() => {
    if (activityError) {
      logger.warn("student_note_activities_load_failed", {
        error:
          activityError instanceof Error
            ? activityError.message
            : String(activityError),
      });
    }
  }, [activityError]);
  const referenceNames = useMemo<ReferenceNames>(
    () => ({
      activities: activityData ?? [],
      educationGroupId,
      educationGroupName,
    }),
    [activityData, educationGroupId, educationGroupName],
  );
  const tenantMutate = useTenantMutate();

  const revalidateNotes = async (permanentNotesChanged: boolean) => {
    const revalidations: Promise<unknown>[] = [mutate()];
    if (permanentNotesChanged) {
      revalidations.push(tenantMutate(`student-permanent-notes-${studentId}`));
    }
    await Promise.all(revalidations);
  };

  const [composing, setComposing] = useState(false);
  const [draft, setDraft] = useState<StudentNoteDraft>(emptyDraft);
  const [editing, setEditing] = useState<StudentNote | null>(null);
  const [editDraft, setEditDraft] = useState<StudentNoteDraft>(emptyDraft);
  const [deleteTarget, setDeleteTarget] = useState<StudentNote | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const formAreaRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formAreaRef);
  const deleteErrors = useApiFormError();
  const load = useApiLoadError();
  // „Wiederholen“ sendet den aktuellen Entwurf, nicht den vom Fehlerzeitpunkt.
  const latestSaveRef = useRef<() => Promise<void>>(async () => undefined);

  const showLoadError = load.show;
  const clearLoadError = load.clear;
  useEffect(() => {
    if (error) {
      void showLoadError(error, {
        object: "die Liste der Notizen",
        retry: () => void mutate(),
      });
    } else {
      clearLoadError();
    }
  }, [error, mutate, showLoadError, clearLoadError]);

  const notes = useMemo(() => data ?? [], [data]);
  // Dauerhafte Hinweise stehen außerhalb der Chronik: sie beschreiben keinen
  // Tag, also gehören sie unter keine Monatsüberschrift.
  const permanentNotes = useMemo(
    () => notes.filter((note) => note.kind === NOTE_KIND_PERMANENT),
    [notes],
  );
  const months = useMemo(() => {
    const grouped = new Map<string, StudentNote[]>();
    for (const note of notes) {
      if (note.kind === NOTE_KIND_PERMANENT) continue;
      const key = noteDay(note).slice(0, 7);
      const bucket = grouped.get(key);
      if (bucket) {
        bucket.push(note);
      } else {
        grouped.set(key, [note]);
      }
    }
    // Gelesen wird nach dem Tag, den ein Eintrag beschreibt; zwei Einträge zum
    // selben Tag ordnet die Schreibzeit.
    for (const bucket of grouped.values()) {
      bucket.sort((a, b) => {
        const byDay = noteDay(b).localeCompare(noteDay(a));
        return byDay !== 0 ? byDay : b.createdAt.localeCompare(a.createdAt);
      });
    }
    return [...grouped.entries()].sort((a, b) => b[0].localeCompare(a[0]));
  }, [notes]);

  const startCompose = () => {
    setEditing(null);
    formErrors.clear();
    setDraft(emptyDraft());
    setComposing(true);
  };

  const startEdit = (note: StudentNote) => {
    setComposing(false);
    formErrors.clear();
    setEditing(note);
    setEditDraft({
      kind: note.kind,
      visibility: note.visibility,
      category: note.category,
      body: note.body,
      subjectDate: note.subjectDate === "" ? undefined : note.subjectDate,
      sharedWithSchool: note.sharedWithSchool,
    });
  };

  const saveNew = async () => {
    setSaving(true);
    formErrors.clear();
    try {
      await studentNotesService.create(studentId, {
        ...draft,
        body: draft.body.trim(),
      });
      setComposing(false);
      setDraft(emptyDraft());
      await revalidateNotes(draft.kind === NOTE_KIND_PERMANENT);
    } catch (caught) {
      logger.error("student_note_create_failed", {
        error: caught instanceof Error ? caught.message : String(caught),
      });
      await formErrors.show(caught, {
        object: "die Notiz",
        retry: () => void latestSaveRef.current(),
      });
    } finally {
      setSaving(false);
    }
  };

  const saveEdit = async () => {
    if (!editing) {
      return;
    }
    setSaving(true);
    formErrors.clear();
    try {
      await studentNotesService.update(studentId, editing.id, {
        ...editDraft,
        body: editDraft.body.trim(),
      });
      setEditing(null);
      await revalidateNotes(
        editing.kind === NOTE_KIND_PERMANENT ||
          editDraft.kind === NOTE_KIND_PERMANENT,
      );
    } catch (caught) {
      logger.error("student_note_update_failed", {
        error: caught instanceof Error ? caught.message : String(caught),
      });
      await formErrors.show(caught, {
        object: "die Notiz",
        retry: () => void latestSaveRef.current(),
      });
    } finally {
      setSaving(false);
    }
  };

  // Bearbeiten und Neu schließen sich aus; der Ref zeigt auf den offenen.
  useLayoutEffect(() => {
    latestSaveRef.current = editing ? saveEdit : saveNew;
  });

  const confirmDelete = async () => {
    if (!deleteTarget) {
      return;
    }
    setDeleting(true);
    deleteErrors.clear();
    try {
      await studentNotesService.remove(studentId, deleteTarget.id);
      setDeleteTarget(null);
      await revalidateNotes(deleteTarget.kind === NOTE_KIND_PERMANENT);
    } catch (caught) {
      logger.error("student_note_delete_failed", {
        error: caught instanceof Error ? caught.message : String(caught),
      });
      await deleteErrors.show(caught, {
        object: "die Notiz",
        retry: () => void confirmDelete(),
      });
    } finally {
      setDeleting(false);
    }
  };

  const renderNote = (note: StudentNote) => (
    <li
      key={note.id}
      className="moto-content-surface rounded-2xl border p-4 shadow-sm"
    >
      {editing?.id === note.id ? (
        <NoteForm
          idPrefix={`note-${note.id}`}
          draft={editDraft}
          onChange={setEditDraft}
          onCancel={() => {
            formErrors.clear();
            setEditing(null);
          }}
          onSave={() => void saveEdit()}
          saving={saving}
          fieldError={formErrors.fieldError}
          visibilityOptions={visibilityOptionsFor(note)}
        />
      ) : (
        <div className="flex items-start gap-3">
          <div className="min-w-0 flex-1 space-y-2">
            <p className="text-sm whitespace-pre-line text-gray-900">
              {note.body}
            </p>
            <NoteMeta note={note} names={referenceNames} />
          </div>
          {note.canEdit || note.canDelete ? (
            <OverflowMenu
              ariaLabel="Aktionen zur Notiz"
              items={[
                ...(note.canEdit
                  ? [
                      {
                        label: "Bearbeiten",
                        onClick: () => startEdit(note),
                      },
                    ]
                  : []),
                ...(note.canDelete
                  ? [
                      {
                        label: "Löschen",
                        destructive: true,
                        onClick: () => {
                          deleteErrors.clear();
                          setDeleteTarget(note);
                        },
                      },
                    ]
                  : []),
              ]}
            />
          ) : null}
        </div>
      )}
    </li>
  );

  if (isLoading) {
    return <Loading />;
  }

  if (error) {
    return <LoadErrorAlert error={load.error} />;
  }

  return (
    <SectionCard
      title="Notizen"
      description="Einträge zu diesem Kind. Eltern sehen sie nicht."
      icon={NotebookPen}
      action={
        composing ? undefined : (
          <Button type="button" size="md" onClick={startCompose}>
            Neue Notiz
          </Button>
        )
      }
    >
      <div ref={formAreaRef} className="space-y-6">
        <FormErrorAlert message={formErrors.error} />

        {composing ? (
          <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
            <NoteForm
              idPrefix="new-note"
              draft={draft}
              onChange={setDraft}
              onCancel={() => {
                formErrors.clear();
                setComposing(false);
              }}
              onSave={() => void saveNew()}
              saving={saving}
              fieldError={formErrors.fieldError}
              visibilityOptions={visibilityOptionsFor(draft)}
              referenceChoices={referenceOptions(referenceNames)}
              referenceHint={
                activityError
                  ? "Die Angebote konnten nicht geladen werden. Sie können die Notiz trotzdem ohne Angebot speichern."
                  : undefined
              }
            />
          </div>
        ) : null}

        {notes.length === 0 && !composing ? (
          <EmptyState
            title="Noch keine Notizen"
            description="Schreiben Sie die erste Notiz über „Neue Notiz“ oben."
          />
        ) : null}

        {permanentNotes.length > 0 ? (
          <section className="space-y-3">
            <h3 className="text-sm font-semibold text-gray-900">
              Dauerhafte Hinweise
            </h3>
            <ul className="space-y-3">{permanentNotes.map(renderNote)}</ul>
          </section>
        ) : null}

        {months.map(([month, entries]) => (
          <section key={month} className="space-y-3">
            <h3 className="text-sm font-semibold text-gray-900">
              {monthLabel(`${month}-01`)}
            </h3>
            <ul className="space-y-3">{entries.map(renderNote)}</ul>
          </section>
        ))}
      </div>

      <ConfirmDeleteModal
        isOpen={deleteTarget !== null}
        title="Notiz löschen"
        description="Die Notiz verschwindet aus allen Ansichten."
        gate={{ mode: "twoStep" }}
        onConfirm={confirmDelete}
        onClose={() => setDeleteTarget(null)}
        loading={deleting}
        error={deleteErrors.error}
      />
    </SectionCard>
  );
}
