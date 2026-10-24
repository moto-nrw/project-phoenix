"use client";

// Kalender in moto schule (#4798): die Termine, zu denen die OGS die
// Lehrkraft einlädt, die Schließtage und Ferien der Schule und die eigenen
// Schultermine. Ein Schultermin (Ausflug, Pädagogischer Tag,
// Unterrichtsende) erreicht immer die OGS und die Lehrkräfte, die Eltern nur,
// wenn die Lehrkraft das beim Anlegen wählt.

import { useLayoutEffect, useMemo, useRef, useState } from "react";
import type { FormEvent } from "react";
import { useSession } from "next-auth/react";

import { CalendarSubscribePanel } from "~/components/calendar/calendar-subscribe-panel";
import {
  calendarEmptyLabel,
  calendarFetchRange,
  PersonalCalendar,
  PersonalCalendarChrome,
  visibleCalendarEvents,
  type CalendarViewMode,
} from "~/components/calendar/personal-calendar";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
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
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import { berlinTodayISO, toISODate } from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import {
  createSchoolAppointment,
  deleteSchoolAppointment,
  getSchoolCalendar,
  respondSchoolCalendar,
  SCHOOL_EVENT_TYPE_LABELS,
  updateSchoolAppointment,
  type CalendarEvent,
  type SchoolAppointmentParents,
  type SchoolCalendarResponse,
  type SchoolEventType,
} from "~/lib/personal-calendar-api";
import {
  emptySchoolAppointmentDraft,
  schoolAppointmentDraftFromEvent,
  schoolAppointmentFieldErrors,
  schoolAppointmentRequest,
  schoolCalendarEvents,
  type SchoolAppointmentDraft,
} from "~/lib/school-calendar";
import { schoolClassLabel } from "~/lib/school-class-label";
import { useSWRAuth } from "~/lib/swr";

const logger = createLogger({ component: "SchoolCalendarPage" });

function logFailure(event: string, error: unknown) {
  logger.error(event, {
    error: error instanceof Error ? error.message : String(error),
  });
}

const eventTypeOptions = (
  Object.entries(SCHOOL_EVENT_TYPE_LABELS) as Array<[SchoolEventType, string]>
).map(([value, label]) => ({ value, label }));

const parentOptions: ReadonlyArray<{
  value: SchoolAppointmentParents;
  label: string;
}> = [
  { value: "none", label: "Keine Eltern" },
  { value: "classes", label: "Eltern meiner Klassen" },
  { value: "school", label: "Alle Eltern der Schule" },
];

export default function SchoolCalendarPage() {
  const { data: session } = useSession();
  const toast = useToast();
  const { show: showActionError } = useApiErrorDisplay();

  const [referenceDate, setReferenceDate] = useState(() => new Date());
  const [viewMode, setViewMode] = useState<CalendarViewMode>("week");
  const [showWeekend, setShowWeekend] = useState(false);

  const range = useMemo(
    () => calendarFetchRange(referenceDate, viewMode),
    [referenceDate, viewMode],
  );
  const calendarKey = session
    ? `school-calendar-${toISODate(range.from)}-${toISODate(range.to)}`
    : null;
  const { data, error, isLoading, mutate } = useSWRAuth<SchoolCalendarResponse>(
    calendarKey,
    () => getSchoolCalendar(range.from, range.to),
  );
  const events = useMemo(() => schoolCalendarEvents(data), [data]);
  const classes = data?.classes ?? [];
  const loadError = useSwrLoadError(error, "den Kalender", () => mutate());

  // Formular: angelegt wird ein neuer Schultermin, geändert ein eigener.
  const [formOpen, setFormOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<SchoolAppointmentDraft>(() =>
    emptySchoolAppointmentDraft(berlinTodayISO()),
  );
  const [submitting, setSubmitting] = useState(false);
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);

  const [deleteTarget, setDeleteTarget] = useState<CalendarEvent | null>(null);
  const [busyAppointmentId, setBusyAppointmentId] = useState<string | null>(
    null,
  );
  const deleteErrors = useApiFormError();
  const [respondingRecipientId, setRespondingRecipientId] = useState<
    string | null
  >(null);

  // „Wiederholen“ im Fehlerhinweis läuft über die jeweils aktuelle Fassung.
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const latestDeleteRef = useRef<() => void>(() => undefined);
  const latestRespondRef = useRef<
    (recipientId: string, status: "accepted" | "declined") => void
  >(() => undefined);

  const update = (patch: Partial<SchoolAppointmentDraft>) =>
    setDraft((current) => ({ ...current, ...patch }));

  const openCreate = () => {
    formErrors.clear();
    setEditingId(null);
    setDraft(emptySchoolAppointmentDraft(berlinTodayISO()));
    setFormOpen(true);
  };

  const openEdit = (event: CalendarEvent) => {
    if (!event.appointment_id) return;
    formErrors.clear();
    setEditingId(event.appointment_id);
    setDraft(schoolAppointmentDraftFromEvent(event));
    setFormOpen(true);
  };

  const closeForm = () => {
    setFormOpen(false);
    setEditingId(null);
    formErrors.clear();
  };

  // Gespeichert ist schon. Scheitert nur das Neuladen, zeigt die Seite den
  // Ladefehler mit Wiederholen; deshalb kein zweiter Fehler hier.
  const refreshCalendar = async () => {
    try {
      await mutate();
    } catch (err) {
      logFailure("school_calendar_refresh_failed", err);
    }
  };

  const submit = async () => {
    const fields = schoolAppointmentFieldErrors(draft, !editingId);
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", fields);
      return;
    }
    formErrors.clear();
    setSubmitting(true);
    try {
      const body = schoolAppointmentRequest(draft);
      if (editingId) {
        await updateSchoolAppointment(editingId, body);
      } else {
        await createSchoolAppointment(body);
      }
      toast.success(`Der Termin „${body.title}“ ist gespeichert.`);
      closeForm();
    } catch (err) {
      logFailure("school_appointment_save_failed", err);
      await formErrors.show(err, {
        object: "das Speichern des Termins",
        retry: () => latestSubmitRef.current(),
      });
      return;
    } finally {
      setSubmitting(false);
    }
    await refreshCalendar();
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void submit();
  };

  const confirmDelete = async () => {
    const appointmentId = deleteTarget?.appointment_id;
    if (!deleteTarget || !appointmentId) return;
    deleteErrors.clear();
    setBusyAppointmentId(appointmentId);
    try {
      await deleteSchoolAppointment(appointmentId);
      toast.success(`Der Termin „${deleteTarget.title}“ ist gelöscht.`);
      setDeleteTarget(null);
    } catch (err) {
      logFailure("school_appointment_delete_failed", err);
      await deleteErrors.show(err, {
        object: "das Löschen des Termins",
        retry: () => latestDeleteRef.current(),
      });
      return;
    } finally {
      setBusyAppointmentId(null);
    }
    await refreshCalendar();
  };

  const handleRespond = async (
    recipientId: string,
    status: "accepted" | "declined",
  ) => {
    setRespondingRecipientId(recipientId);
    try {
      await respondSchoolCalendar(recipientId, status);
      toast.success(
        status === "accepted"
          ? "Ihre Zusage ist gespeichert."
          : "Ihre Absage ist gespeichert.",
      );
    } catch (err) {
      logFailure("school_calendar_response_failed", err);
      void showActionError(err, {
        object: "das Speichern Ihrer Antwort",
        retry: () => latestRespondRef.current(recipientId, status),
      });
      return;
    } finally {
      setRespondingRecipientId(null);
    }
    await refreshCalendar();
  };

  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submit();
    latestDeleteRef.current = () => void confirmDelete();
    latestRespondRef.current = (recipientId, status) =>
      void handleRespond(recipientId, status);
  });

  const eventCount = visibleCalendarEvents(
    events,
    showWeekend,
    viewMode,
  ).length;
  const creating = !editingId;
  const classesError = formErrors.fieldError("classes");

  return (
    <div className="-mt-1.5 w-full space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">
          Kalender
        </h1>
        <p className="mt-1 text-sm text-gray-500">
          Termine der OGS, Schließtage, Ferien und Ihre Schultermine.
        </p>
      </div>

      <SectionCard>
        <PersonalCalendarChrome
          events={events}
          referenceDate={referenceDate}
          viewMode={viewMode}
          showWeekend={showWeekend}
          onShowWeekendChange={setShowWeekend}
          onDateChange={setReferenceDate}
          onViewModeChange={setViewMode}
          onCreate={openCreate}
        />
      </SectionCard>

      {error && loadError ? (
        <LoadErrorAlert error={loadError} />
      ) : !isLoading && eventCount === 0 ? (
        <SectionCard>
          <EmptyState
            title={calendarEmptyLabel(viewMode)}
            description="Legen Sie einen Schultermin an oder wechseln Sie den Zeitraum."
          />
        </SectionCard>
      ) : (
        <PersonalCalendar
          events={events}
          referenceDate={referenceDate}
          viewMode={viewMode}
          showWeekend={showWeekend}
          loading={isLoading}
          onRespond={(recipientId, status) =>
            void handleRespond(recipientId, status)
          }
          respondingRecipientId={respondingRecipientId}
          onEdit={openEdit}
          onDelete={(event) => {
            deleteErrors.clear();
            setDeleteTarget(event);
          }}
          busyAppointmentId={busyAppointmentId}
        />
      )}

      <CalendarSubscribePanel audience="school" />

      {/* Das Formular hat mehr als sechs Felder und steht deshalb im Panel
          neben dem Kalender, wie im OGS-Portal. */}
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
                {creating ? "Schultermin anlegen" : "Schultermin bearbeiten"}
              </SlideOverTitle>
            </div>
            <SlideOverCloseButton disabled={submitting} />
          </SlideOverHeader>
          <SlideOverBody error={formErrors.error}>
            <div ref={formRef}>
              <form className="space-y-5" onSubmit={handleSubmit} noValidate>
                <div className="grid gap-4 md:grid-cols-2">
                  <Input
                    label="Titel"
                    name="title"
                    value={draft.title}
                    onChange={(event) => update({ title: event.target.value })}
                    error={formErrors.fieldError("title")}
                    disabled={submitting}
                    required
                  />
                  <label htmlFor="school-event-type" className="block">
                    <span className="mb-2 block text-sm font-medium text-gray-700">
                      Art
                    </span>
                    <CustomSelect
                      id="school-event-type"
                      ariaLabel="Art"
                      value={draft.eventType}
                      onChange={(next) =>
                        update({ eventType: next as SchoolEventType })
                      }
                      disabled={submitting}
                      options={eventTypeOptions}
                    />
                  </label>
                  <ISODatePicker
                    label="Erster Tag"
                    id="school-start-date"
                    name="start_date"
                    controlSize="lg"
                    value={draft.startDate}
                    error={formErrors.fieldError("start_date")}
                    onChange={(next) =>
                      update({
                        startDate: next,
                        ...(draft.endDate < next ? { endDate: next } : {}),
                      })
                    }
                    disabled={submitting}
                    calendarLayout="popover"
                    hideClearButton
                    required
                  />
                  <ISODatePicker
                    label="Letzter Tag"
                    id="school-end-date"
                    name="end_date"
                    controlSize="lg"
                    value={draft.endDate}
                    error={formErrors.fieldError("end_date")}
                    min={draft.startDate}
                    onChange={(next) => update({ endDate: next })}
                    disabled={submitting}
                    calendarLayout="popover"
                    hideClearButton
                    required
                  />
                </div>

                <label
                  htmlFor="school-all-day"
                  className="flex items-center gap-2 text-sm font-medium text-gray-700"
                >
                  <Checkbox
                    id="school-all-day"
                    checked={draft.allDay}
                    onChange={(event) =>
                      update({ allDay: event.target.checked })
                    }
                    disabled={submitting}
                  />
                  Ganztägig
                </label>

                {draft.allDay ? null : (
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
                )}

                <Input
                  label="Ort"
                  name="location"
                  value={draft.location}
                  onChange={(event) => update({ location: event.target.value })}
                  error={formErrors.fieldError("location")}
                  disabled={submitting}
                />
                <Textarea
                  label="Beschreibung"
                  name="description"
                  value={draft.description}
                  onChange={(event) =>
                    update({ description: event.target.value })
                  }
                  error={formErrors.fieldError("description")}
                  disabled={submitting}
                  rows={3}
                />

                <fieldset className="space-y-3">
                  <legend className="text-sm font-medium text-gray-700">
                    Wer sieht den Termin?
                  </legend>
                  <p className="text-sm text-gray-600">
                    Die OGS und alle Lehrkräfte sehen ihn immer.
                    {creating
                      ? " Wählen Sie, ob auch Eltern ihn sehen."
                      : " Wer ihn sieht, bleibt beim Ändern gleich."}
                  </p>
                  {creating ? (
                    <div className="grid gap-2">
                      {parentOptions.map((option) => {
                        const noClasses =
                          option.value === "classes" && classes.length === 0;
                        return (
                          <ChoiceTile
                            key={option.value}
                            htmlFor={`school-parents-${option.value}`}
                            selected={draft.parents === option.value}
                            disabled={submitting || noClasses}
                          >
                            <Radio
                              id={`school-parents-${option.value}`}
                              name="parents"
                              value={option.value}
                              checked={draft.parents === option.value}
                              disabled={submitting || noClasses}
                              onChange={() =>
                                update({ parents: option.value, classes: [] })
                              }
                            />
                            <span className="min-w-0 flex-1">
                              {option.label}
                              {noClasses ? (
                                <span className="mt-0.5 block text-xs font-normal text-gray-500">
                                  Ihnen ist keine Klasse zugeordnet.
                                </span>
                              ) : null}
                            </span>
                          </ChoiceTile>
                        );
                      })}
                    </div>
                  ) : null}
                  {creating && draft.parents === "classes" ? (
                    <div
                      role="group"
                      aria-label="Klassen"
                      aria-describedby={
                        classesError ? "school-classes-error" : undefined
                      }
                      className="flex flex-wrap gap-2"
                    >
                      {classes.map((schoolClass) => {
                        const checked = draft.classes.includes(schoolClass);
                        const id = `school-class-${schoolClass}`;
                        return (
                          <ChoiceTile
                            key={schoolClass}
                            htmlFor={id}
                            selected={checked}
                            disabled={submitting}
                          >
                            <Checkbox
                              id={id}
                              checked={checked}
                              disabled={submitting}
                              onChange={(event) =>
                                update({
                                  classes: event.target.checked
                                    ? [...draft.classes, schoolClass]
                                    : draft.classes.filter(
                                        (value) => value !== schoolClass,
                                      ),
                                })
                              }
                            />
                            {schoolClassLabel(schoolClass)}
                          </ChoiceTile>
                        );
                      })}
                    </div>
                  ) : null}
                  <FieldErrorHint
                    id="school-classes-error"
                    message={creating ? classesError : null}
                  />
                </fieldset>

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
                    loadingText="Speichert…"
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
        title="Termin löschen"
        description={
          <p className="text-gray-700">
            Der Termin wird gelöscht. Danach sieht ihn niemand mehr.
          </p>
        }
        gate={{ mode: "twoStep", firstStepLabel: "Löschen" }}
        confirmLabel="Endgültig löschen"
        onConfirm={() => void confirmDelete()}
        onClose={() => {
          if (!busyAppointmentId) {
            setDeleteTarget(null);
            deleteErrors.clear();
          }
        }}
        loading={Boolean(busyAppointmentId)}
        error={deleteErrors.error}
      />
    </div>
  );
}
