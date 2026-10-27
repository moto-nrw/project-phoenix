"use client";

// Elternbriefe in moto schule (#4800): die Lehrkraft schreibt an die Eltern
// einer eigenen Klasse, die Schulverwaltung an jede Klasse. Der Brief ist
// eine Elternmitteilung wie im OGS-Portal: dieselben Eltern, dieselbe
// Lesebestätigung, dasselbe Einverständnis mit Unterschrift. Die Liste zeigt
// nur die eigenen Briefe; die OGS liest sie in ihrer Übersicht mit.

import { useLayoutEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { useRouter } from "next/navigation";
import { useSession } from "next-auth/react";
import { Plus } from "lucide-react";

import { AnnouncementStatusBadge } from "~/components/announcements/announcement-meta";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { Radio } from "~/components/ui/radio";
import { SectionCard } from "~/components/ui/section-card";
import {
  SlideOver,
  SlideOverBody,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import {
  createAnnouncement,
  fetchAnnouncements,
  publishAnnouncement,
  SCHOOL_PARENT_LETTERS_BASE,
  type Announcement,
} from "~/lib/parent-announcements-api";
import { fetchClassDayClassesSchool } from "~/lib/school-class-day-api";
import { schoolClassLabel } from "~/lib/school-class-label";
import {
  emptySchoolParentLetterDraft,
  SCHOOL_PARENT_LETTER_KINDS,
  SCHOOL_PARENT_LETTERS_KEY,
  SCHOOL_PARENT_LETTERS_ROUTE,
  schoolParentLetterFieldErrors,
  schoolParentLetterKindLabel,
  schoolParentLetterRequest,
  type SchoolParentLetterDraft,
} from "~/lib/school-parent-letters";
import { schoolPath } from "~/lib/school-url";
import { useSWRAuth } from "~/lib/swr";

const logger = createLogger({ component: "SchoolParentLettersPage" });

const columns: DataTableColumn<Announcement>[] = [
  {
    key: "title",
    header: "Betreff",
    stacked: "title",
    sortValue: (row) => row.title.toLowerCase(),
    render: (row) => (
      <div className="min-w-0">
        <p className="truncate font-semibold text-gray-900">{row.title}</p>
        <p className="mt-0.5 text-xs text-gray-500">
          {schoolParentLetterKindLabel(row.delivery_mode)}
        </p>
      </div>
    ),
  },
  {
    key: "class",
    header: "Klasse",
    render: (row) => (
      <span className="text-sm text-gray-700">
        {schoolClassLabel(row.sender_school_class ?? "")}
      </span>
    ),
  },
  {
    key: "status",
    header: "Status",
    stacked: "meta",
    sortValue: (row) => row.status,
    render: (row) => <AnnouncementStatusBadge status={row.status} />,
  },
  {
    key: "sent",
    header: "Gesendet",
    sortValue: (row) => row.published_at ?? null,
    render: (row) => (
      <span className="text-sm text-gray-700">
        {row.published_at ? formatDate(row.published_at) : "Noch nicht"}
      </span>
    ),
  },
];

export default function SchoolParentLettersPage() {
  const { data: session } = useSession();
  const router = useRouter();
  const toast = useToast();

  const {
    data: letters,
    error,
    isLoading,
    mutate,
  } = useSWRAuth<Announcement[]>(
    session ? SCHOOL_PARENT_LETTERS_KEY : null,
    () => fetchAnnouncements(false, SCHOOL_PARENT_LETTERS_BASE),
  );
  const loadError = useSwrLoadError(error, "Ihre Elternbriefe", () => mutate());
  const { data: classInfo } = useSWRAuth(
    session ? "school-parent-letter-classes" : null,
    fetchClassDayClassesSchool,
    { revalidateOnFocus: false },
  );
  const classes = classInfo?.classes ?? [];

  const [formOpen, setFormOpen] = useState(false);
  const [draft, setDraft] = useState<SchoolParentLetterDraft>(() =>
    emptySchoolParentLetterDraft([]),
  );
  const [submitting, setSubmitting] = useState(false);
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const latestSubmitRef = useRef<() => void>(() => undefined);

  const update = (patch: Partial<SchoolParentLetterDraft>) =>
    setDraft((current) => ({ ...current, ...patch }));

  const openCreate = () => {
    formErrors.clear();
    setDraft(emptySchoolParentLetterDraft(classes));
    setFormOpen(true);
  };

  const closeForm = () => {
    setFormOpen(false);
    formErrors.clear();
  };

  // Anlegen und Senden in einem Schritt: die Lehrkraft schreibt einen Brief
  // und schickt ihn ab. Scheitert nur das Senden, bleibt der Entwurf in der
  // Liste und lässt sich dort senden.
  const submit = async () => {
    const fields = schoolParentLetterFieldErrors(draft);
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", fields);
      return;
    }
    formErrors.clear();
    setSubmitting(true);
    try {
      const created = await createAnnouncement(
        schoolParentLetterRequest(draft),
        SCHOOL_PARENT_LETTERS_BASE,
      );
      try {
        await publishAnnouncement(created.id, SCHOOL_PARENT_LETTERS_BASE);
      } finally {
        await mutate();
      }
      toast.success(
        `Der Elternbrief „${created.title}“ ist an die Eltern der ${schoolClassLabel(draft.schoolClass)} verschickt.`,
      );
      closeForm();
    } catch (err) {
      logger.error("school_parent_letter_send_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await formErrors.show(err, {
        object: "das Senden des Elternbriefs",
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

  const noClasses = classInfo !== undefined && classes.length === 0;
  const rows = letters ?? [];
  const classError = formErrors.fieldError("school_class");

  return (
    <div className="-mt-1.5 w-full space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">
            Elternbriefe
          </h1>
          <p className="mt-1 text-sm text-gray-500">
            Briefe an die Eltern Ihrer Klasse. Die Eltern lesen sie in moto.
          </p>
        </div>
        <Button
          type="button"
          variant="primary"
          size="md"
          className="gap-1.5"
          onClick={openCreate}
          disabled={noClasses}
        >
          <Plus className="h-4 w-4" aria-hidden />
          Neuer Elternbrief
        </Button>
      </div>

      {noClasses ? (
        <p className="text-sm text-gray-600">
          Ihnen ist keine Klasse zugeordnet. Bitte fragen Sie die OGS-Leitung.
        </p>
      ) : null}

      <SectionCard title="Meine Elternbriefe">
        {error && loadError ? (
          <LoadErrorAlert error={loadError} />
        ) : (
          <DataTable
            columns={columns}
            rows={rows}
            getRowKey={(row) => row.id}
            isLoading={isLoading && !letters}
            stackedOnMobile
            defaultSortKey="sent"
            defaultSortDirection="desc"
            onRowClick={(row) =>
              router.push(
                schoolPath(
                  `${SCHOOL_PARENT_LETTERS_ROUTE}/${encodeURIComponent(row.id)}`,
                ),
              )
            }
            emptyState={
              <EmptyState
                title="Noch keine Elternbriefe"
                description="Ihre gesendeten Briefe stehen hier, mit dem Stand, wer sie gelesen hat."
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
              <SlideOverTitle>Neuer Elternbrief</SlideOverTitle>
            </div>
            <SlideOverCloseButton disabled={submitting} />
          </SlideOverHeader>
          <SlideOverBody error={formErrors.error}>
            <div ref={formRef}>
              <form className="space-y-5" onSubmit={handleSubmit} noValidate>
                <label htmlFor="parent-letter-class" className="block">
                  <span className="mb-2 block text-sm font-medium text-gray-700">
                    Klasse
                  </span>
                  <CustomSelect
                    id="parent-letter-class"
                    name="school_class"
                    ariaLabel="Klasse"
                    value={draft.schoolClass}
                    placeholder="Klasse wählen"
                    onChange={(next) => update({ schoolClass: next })}
                    disabled={submitting}
                    invalid={!!classError}
                    ariaDescribedBy={
                      classError ? "parent-letter-class-error" : undefined
                    }
                    options={classes.map((schoolClass) => ({
                      value: schoolClass,
                      label: schoolClassLabel(schoolClass),
                    }))}
                  />
                  <FieldErrorHint
                    id="parent-letter-class-error"
                    message={classError}
                  />
                </label>

                <Input
                  label="Betreff"
                  name="title"
                  value={draft.title}
                  onChange={(event) => update({ title: event.target.value })}
                  error={formErrors.fieldError("title")}
                  disabled={submitting}
                  maxLength={200}
                  required
                />

                <Textarea
                  label="Text"
                  name="body"
                  rows={8}
                  value={draft.body}
                  onChange={(event) => update({ body: event.target.value })}
                  error={formErrors.fieldError("body")}
                  disabled={submitting}
                  maxLength={4000}
                  required
                />

                <fieldset className="space-y-2">
                  <legend className="mb-2 block text-sm font-medium text-gray-700">
                    Was sollen die Eltern tun?
                  </legend>
                  <div className="grid gap-2">
                    {SCHOOL_PARENT_LETTER_KINDS.map((option) => (
                      <ChoiceTile
                        key={option.value}
                        htmlFor={`parent-letter-kind-${option.value}`}
                        selected={draft.kind === option.value}
                        disabled={submitting}
                      >
                        <Radio
                          id={`parent-letter-kind-${option.value}`}
                          name="kind"
                          value={option.value}
                          checked={draft.kind === option.value}
                          disabled={submitting}
                          onChange={() => update({ kind: option.value })}
                        />
                        <span className="min-w-0 flex-1">
                          {option.label}
                          <span className="mt-0.5 block text-xs font-normal text-gray-500">
                            {option.hint}
                          </span>
                        </span>
                      </ChoiceTile>
                    ))}
                  </div>
                </fieldset>

                <p className="text-sm text-gray-600">
                  Die Eltern der Klasse bekommen den Brief in moto und eine
                  E-Mail. Als Absender sehen sie Ihren Namen, die Klasse und die
                  Schule. Die OGS kann den Brief lesen.
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
                    loadingText="Wird gesendet…"
                  >
                    Senden
                  </Button>
                </div>
              </form>
            </div>
          </SlideOverBody>
        </SlideOverContent>
      </SlideOver>
    </div>
  );
}
