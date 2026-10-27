"use client";

// Elternsprechtag in moto schule (#4801): Die Lehrkraft bietet für eine ihrer
// Klassen einen Tag an, die Eltern buchen in der Eltern-App ein freies
// Zeitfenster je Kind. Hier legt sie den Tag an, ändert oder löscht ihn und
// sieht und druckt, wer wann kommt. Der gewählte Tag steht als
// `?sprechtag=…` in der Adresse, damit der Hinweis zu einer Buchung direkt
// dorthin führt.

import { Suspense, useLayoutEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { Plus, Printer } from "lucide-react";

import { BackLink } from "~/components/ui/back-link";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import {
  SlideOver,
  SlideOverBody,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import {
  createSchoolParentConference,
  deleteSchoolParentConference,
  emptyParentConferenceDraft,
  fetchSchoolParentConferences,
  PARENT_CONFERENCE_SLOT_OPTIONS,
  parentConferenceDraftOf,
  parentConferenceFieldErrors,
  parentConferenceRequest,
  parentConferenceWindowLabel,
  SCHOOL_PARENT_CONFERENCES_KEY,
  updateSchoolParentConference,
  type ParentConference,
  type ParentConferenceDraft,
  type ParentConferenceSlot,
} from "~/lib/parent-conferences";
import { schoolClassLabel } from "~/lib/school-class-label";
import { schoolPath } from "~/lib/school-url";
import { useSWRAuth } from "~/lib/swr";

const logger = createLogger({ component: "SchoolParentConferencePage" });

/** Der gewählte Tag in der Adresse. */
const CONFERENCE_PARAM = "sprechtag";

const conferenceColumns: DataTableColumn<ParentConference>[] = [
  {
    key: "date",
    header: "Tag",
    stacked: "title",
    sortValue: (row) => row.date,
    render: (row) => (
      <div className="min-w-0">
        <p className="font-semibold text-gray-900">{formatDate(row.date)}</p>
        <p className="mt-0.5 text-xs text-gray-500">
          {row.start_time}–{row.end_time} Uhr
          {row.location ? ` · ${row.location}` : ""}
        </p>
      </div>
    ),
  },
  {
    key: "class",
    header: "Klasse",
    render: (row) => (
      <span className="text-sm text-gray-700">
        {schoolClassLabel(row.school_class)}
      </span>
    ),
  },
  {
    key: "booked",
    header: "Gebucht",
    stacked: "meta",
    sortValue: (row) => row.booked_count,
    render: (row) => (
      <span className="text-sm text-gray-700">
        {row.booked_count} von {row.slots.length}
      </span>
    ),
  },
];

const slotColumns: DataTableColumn<ParentConferenceSlot>[] = [
  {
    key: "time",
    header: "Uhrzeit",
    stacked: "title",
    render: (row) => (
      <span className="font-semibold text-gray-900">
        {parentConferenceWindowLabel(row)}
      </span>
    ),
  },
  {
    key: "child",
    header: "Kind",
    stacked: "meta",
    render: (row) =>
      row.booked ? (
        <span className="text-sm text-gray-900">
          {row.student_name ?? "Gebucht"}
        </span>
      ) : (
        <StatusBadge tone="gray" label="Frei" />
      ),
  },
];

function SchoolParentConferences() {
  const { data: session } = useSession();
  const router = useRouter();
  const pathname = usePathname();
  const selectedId = useSearchParams().get(CONFERENCE_PARAM);
  const toast = useToast();

  const { data, error, isLoading, mutate } = useSWRAuth(
    session ? SCHOOL_PARENT_CONFERENCES_KEY : null,
    fetchSchoolParentConferences,
  );
  const loadError = useSwrLoadError(error, "Ihre Elternsprechtage", () =>
    mutate(),
  );
  const conferences = data?.conferences ?? [];
  const classes = data?.classes ?? [];
  const selected = conferences.find((row) => row.id === selectedId) ?? null;

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<ParentConference | null>(null);
  const [draft, setDraft] = useState<ParentConferenceDraft>(() =>
    emptyParentConferenceDraft([]),
  );
  const [submitting, setSubmitting] = useState(false);
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const latestSubmitRef = useRef<() => void>(() => undefined);

  const [deleteTarget, setDeleteTarget] = useState<ParentConference | null>(
    null,
  );
  const [deleting, setDeleting] = useState(false);
  const deleteErrors = useApiFormError();
  const latestDeleteRef = useRef<() => void>(() => undefined);

  const select = (id: string | null) => {
    const query = id ? `?${CONFERENCE_PARAM}=${encodeURIComponent(id)}` : "";
    router.replace(`${pathname}${query}`, { scroll: false });
  };

  const update = (patch: Partial<ParentConferenceDraft>) =>
    setDraft((current) => ({ ...current, ...patch }));

  const openCreate = () => {
    formErrors.clear();
    setEditing(null);
    setDraft(emptyParentConferenceDraft(classes));
    setFormOpen(true);
  };

  const openEdit = (conference: ParentConference) => {
    formErrors.clear();
    setEditing(conference);
    setDraft(parentConferenceDraftOf(conference));
    setFormOpen(true);
  };

  const closeForm = () => {
    setFormOpen(false);
    formErrors.clear();
  };

  const submit = async () => {
    if (submitting) return;
    const fields = parentConferenceFieldErrors(draft, berlinTodayISO());
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", fields);
      return;
    }
    formErrors.clear();
    setSubmitting(true);
    try {
      const request = parentConferenceRequest(draft);
      const saved = editing
        ? await updateSchoolParentConference(editing.id, request)
        : await createSchoolParentConference(request);
      await mutate();
      toast.success(
        editing
          ? `Der Elternsprechtag am ${formatDate(saved.date)} ist geändert.`
          : `Der Elternsprechtag am ${formatDate(saved.date)} ist angelegt. Die Eltern der ${schoolClassLabel(saved.school_class)} können jetzt buchen.`,
      );
      closeForm();
      select(saved.id);
    } catch (err) {
      logger.error("school_parent_conference_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await formErrors.show(err, {
        object: "das Speichern des Elternsprechtags",
        retry: () => latestSubmitRef.current(),
      });
    } finally {
      setSubmitting(false);
    }
  };
  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submit();
  });

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void submit();
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    deleteErrors.clear();
    setDeleting(true);
    try {
      await deleteSchoolParentConference(deleteTarget.id);
      toast.success(
        `Der Elternsprechtag am ${formatDate(deleteTarget.date)} ist gelöscht.`,
      );
      setDeleteTarget(null);
      select(null);
      await mutate();
    } catch (err) {
      logger.error("school_parent_conference_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await deleteErrors.show(err, {
        object: "das Löschen des Elternsprechtags",
        retry: () => latestDeleteRef.current(),
      });
    } finally {
      setDeleting(false);
    }
  };
  useLayoutEffect(() => {
    latestDeleteRef.current = () => void confirmDelete();
  });

  const noClasses = !error && data !== undefined && classes.length === 0;
  const classError = formErrors.fieldError("school_class");
  const breakError = formErrors.fieldError("breaks");

  return (
    <div className="-mt-1.5 w-full space-y-6">
      <BackLink
        href={schoolPath("/school/kalender")}
        label="Zurück zum Kalender"
        className="print:hidden"
      />
      <div className="flex flex-wrap items-start justify-between gap-3 print:hidden">
        <div className="min-w-0">
          <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">
            Elternsprechtag
          </h1>
          <p className="mt-1 text-sm text-gray-500">
            Eltern buchen in der Eltern-App ein Zeitfenster je Kind. Hier sehen
            Sie, wer wann kommt.
          </p>
        </div>
        <Button
          type="button"
          variant="primary"
          size="md"
          className="gap-1.5"
          onClick={openCreate}
          disabled={!!error || classes.length === 0}
        >
          <Plus className="h-4 w-4" aria-hidden />
          Neuer Elternsprechtag
        </Button>
      </div>

      {noClasses ? (
        <p className="text-sm text-gray-600 print:hidden">
          Ihnen ist keine Klasse zugeordnet. Bitte fragen Sie die OGS-Leitung.
        </p>
      ) : null}

      {selected ? (
        <SectionCard
          title={`Elternsprechtag ${schoolClassLabel(selected.school_class)} am ${formatDate(selected.date)}`}
          description={`${selected.start_time}–${selected.end_time} Uhr${selected.location ? ` · ${selected.location}` : ""} · ${selected.booked_count} von ${selected.slots.length} Zeitfenstern gebucht`}
          action={
            <div className="flex flex-wrap gap-2 print:hidden">
              <Button
                type="button"
                variant="outline"
                size="md"
                className="gap-1.5"
                onClick={() => window.print()}
              >
                <Printer className="h-4 w-4" aria-hidden />
                Drucken
              </Button>
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={() => openEdit(selected)}
              >
                Ändern
              </Button>
              <Button
                type="button"
                variant="outline_danger"
                size="md"
                onClick={() => {
                  deleteErrors.clear();
                  setDeleteTarget(selected);
                }}
              >
                Löschen
              </Button>
            </div>
          }
        >
          <DataTable
            columns={slotColumns}
            rows={[...selected.slots]}
            getRowKey={(row) => row.start_time}
            stackedOnMobile
          />
          <p className="mt-4 text-sm text-gray-600 print:hidden">
            Eltern buchen, verschieben oder sagen ab, bis 24 Stunden vor dem
            Termin. Danach melden sie sich bei Ihnen.
          </p>
        </SectionCard>
      ) : null}

      <SectionCard title="Meine Elternsprechtage" className="print:hidden">
        {error && loadError ? (
          <LoadErrorAlert error={loadError} />
        ) : (
          <DataTable
            columns={conferenceColumns}
            rows={[...conferences]}
            getRowKey={(row) => row.id}
            isLoading={isLoading && !data}
            stackedOnMobile
            defaultSortKey="date"
            defaultSortDirection="asc"
            onRowClick={(row) => select(row.id)}
            emptyState={
              <EmptyState
                title="Noch kein Elternsprechtag"
                description="Legen Sie einen Tag an. Dann buchen die Eltern Ihrer Klasse ein Zeitfenster."
              />
            }
          />
        )}
      </SectionCard>

      <SlideOver
        open={formOpen}
        onOpenChange={(open) => {
          if (!open && !submitting) closeForm();
        }}
      >
        <SlideOverContent widthClass="sm:w-[640px]">
          <SlideOverHeader className="flex-row items-start justify-between gap-3">
            <div className="min-w-0">
              <SlideOverTitle>
                {editing ? "Elternsprechtag ändern" : "Neuer Elternsprechtag"}
              </SlideOverTitle>
            </div>
            <SlideOverCloseButton disabled={submitting} />
          </SlideOverHeader>
          <SlideOverBody error={formErrors.error}>
            <div ref={formRef}>
              <form className="space-y-5" onSubmit={handleSubmit} noValidate>
                <label htmlFor="parent-conference-class" className="block">
                  <span className="mb-2 block text-sm font-medium text-gray-700">
                    Klasse
                  </span>
                  <CustomSelect
                    id="parent-conference-class"
                    name="school_class"
                    ariaLabel="Klasse"
                    value={draft.schoolClass}
                    placeholder="Klasse wählen"
                    onChange={(next) => update({ schoolClass: next })}
                    disabled={submitting}
                    invalid={!!classError}
                    ariaDescribedBy={
                      classError ? "parent-conference-class-error" : undefined
                    }
                    options={[
                      ...new Set([
                        ...classes,
                        ...(editing ? [editing.school_class] : []),
                      ]),
                    ].map((schoolClass) => ({
                      value: schoolClass,
                      label: schoolClassLabel(schoolClass),
                    }))}
                  />
                  <FieldErrorHint
                    id="parent-conference-class-error"
                    message={classError}
                  />
                </label>

                <ISODatePicker
                  label="Tag"
                  id="parent-conference-date"
                  name="date"
                  controlSize="lg"
                  value={draft.date}
                  error={formErrors.fieldError("date")}
                  onChange={(next) => update({ date: next })}
                  disabled={submitting}
                />

                <div className="grid grid-cols-2 gap-4">
                  <Input
                    label="Beginn"
                    name="start_time"
                    type="time"
                    value={draft.startTime}
                    error={formErrors.fieldError("start_time")}
                    onChange={(event) =>
                      update({ startTime: event.target.value })
                    }
                    disabled={submitting}
                    required
                  />
                  <Input
                    label="Ende"
                    name="end_time"
                    type="time"
                    value={draft.endTime}
                    error={formErrors.fieldError("end_time")}
                    onChange={(event) =>
                      update({ endTime: event.target.value })
                    }
                    disabled={submitting}
                    required
                  />
                </div>

                <label htmlFor="parent-conference-slot" className="block">
                  <span className="mb-2 block text-sm font-medium text-gray-700">
                    Dauer je Gespräch
                  </span>
                  <CustomSelect
                    id="parent-conference-slot"
                    name="slot_minutes"
                    ariaLabel="Dauer je Gespräch"
                    value={draft.slotMinutes}
                    onChange={(next) => update({ slotMinutes: next })}
                    disabled={submitting}
                    options={PARENT_CONFERENCE_SLOT_OPTIONS.map((minutes) => ({
                      value: minutes,
                      label: `${minutes} Minuten`,
                    }))}
                  />
                </label>

                <fieldset className="space-y-2">
                  <legend className="mb-2 block text-sm font-medium text-gray-700">
                    Pause (freiwillig)
                  </legend>
                  <div className="grid grid-cols-2 gap-4">
                    <Input
                      label="Von"
                      name="break_start"
                      type="time"
                      value={draft.breakStart}
                      onChange={(event) =>
                        update({ breakStart: event.target.value })
                      }
                      disabled={submitting}
                      aria-describedby={
                        breakError ? "parent-conference-break-error" : undefined
                      }
                    />
                    <Input
                      label="Bis"
                      name="break_end"
                      type="time"
                      value={draft.breakEnd}
                      onChange={(event) =>
                        update({ breakEnd: event.target.value })
                      }
                      disabled={submitting}
                    />
                  </div>
                  <FieldErrorHint
                    id="parent-conference-break-error"
                    message={breakError}
                  />
                </fieldset>

                <Input
                  label="Raum (freiwillig)"
                  name="location"
                  value={draft.location}
                  onChange={(event) => update({ location: event.target.value })}
                  disabled={submitting}
                  maxLength={200}
                />

                <p className="text-sm text-gray-600">
                  {editing
                    ? "Die Eltern mit Buchung bekommen eine Nachricht. Fällt ein gebuchtes Zeitfenster weg, ist die Buchung abgesagt."
                    : "Die Eltern der Klasse sehen die freien Zeitfenster in der Eltern-App und buchen eines je Kind."}
                </p>

                <div className="flex flex-wrap justify-end gap-2 border-t border-gray-200 pt-4">
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    onClick={closeForm}
                    disabled={submitting}
                  >
                    Abbrechen
                  </Button>
                  <Button
                    type="submit"
                    size="md"
                    isLoading={submitting}
                    loadingText="Wird gespeichert…"
                  >
                    Speichern
                  </Button>
                </div>
              </form>
            </div>
          </SlideOverBody>
        </SlideOverContent>
      </SlideOver>

      <ConfirmDeleteModal
        isOpen={deleteTarget !== null}
        title="Elternsprechtag löschen"
        description={
          <p className="text-gray-700">
            Der Elternsprechtag wird gelöscht. Alle Buchungen sind dann
            abgesagt, und die Eltern bekommen eine Nachricht.
          </p>
        }
        gate={{ mode: "twoStep", firstStepLabel: "Löschen" }}
        confirmLabel="Endgültig löschen"
        onConfirm={() => void confirmDelete()}
        onClose={() => {
          if (!deleting) {
            setDeleteTarget(null);
            deleteErrors.clear();
          }
        }}
        loading={deleting}
        error={deleteErrors.error}
      />
    </div>
  );
}

export default function SchoolParentConferencePage() {
  return (
    // Der gewählte Tag steht in der Adresse; useSearchParams braucht eine
    // Suspense-Grenze.
    <Suspense fallback={<Skeleton className="h-64 w-full" />}>
      <SchoolParentConferences />
    </Suspense>
  );
}
