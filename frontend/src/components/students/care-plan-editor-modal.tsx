"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronDown, Clock, Loader2, StickyNote } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { FormModal } from "~/components/ui/form-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import { useToast } from "~/contexts/ToastContext";
import {
  type ArrivalDayData,
  type ArrivalScheduleFormEntry,
  WEEKDAYS,
  formatShortDate,
  getWeekdayLabel,
} from "~/lib/arrival-schedule-helpers";
import type {
  DayData as PickupDayData,
  PickupScheduleFormData,
} from "~/lib/pickup-schedule-helpers";
import {
  formatPickupTime,
  pickupScheduleSourceLabel,
} from "~/lib/pickup-schedule-helpers";

/**
 * The care plan is edited through exactly two doors, and each one owns one kind
 * of thing (issue #893):
 *
 *   day card         -> an EXCEPTION for that date, with a Grund
 *   "Wochenplan"     -> the recurring times of the week, with Notizen
 *
 * That split is the whole fix: before, two identical-looking pencils both led
 * to dialogs that mixed exceptions and notes, and a note gave no hint whether
 * it applied once or every week.
 */
type ArrivalMode = "regular" | "time" | "absent";
type PickupMode = "regular" | "time" | "none";

/** One leg (arrival or pickup) of a day exception. */
export type CareLegSubmit =
  | { kind: "regular" }
  | { kind: "time"; time: string; reason: string | null }
  | { kind: "none"; reason: string | null };

export interface CareExceptionSubmit {
  /** The single ISO day this exception applies to. */
  readonly date: string;
  /** null = this leg was not touched and must stay exactly as it is. */
  readonly arrival: CareLegSubmit | null;
  readonly pickup: CareLegSubmit | null;
}

export interface CarePlanWeeklySubmit {
  readonly arrivalSchedules: ArrivalScheduleFormEntry[];
  readonly pickupSchedules: PickupScheduleFormData[];
}

interface CarePlanEditorModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  /** The day the editor was opened from. null = the weekly plan. */
  readonly date: Date | null;
  readonly arrivalDay: ArrivalDayData | null;
  readonly pickupDay: PickupDayData | null;
  readonly weeklyArrival: ArrivalScheduleFormEntry[];
  readonly weeklyPickup: PickupScheduleFormData[];
  readonly onSubmitException: (payload: CareExceptionSubmit) => Promise<void>;
  readonly onSubmitWeekly: (payload: CarePlanWeeklySubmit) => Promise<void>;
  /** Setzt die reguläre Gehzeit des Wochentags auf die Angebots-Gehzeit
   * zurück (#2290); nur angeboten, wenn der Tag von Hand gepflegt ist. */
  readonly onResetPickupToOffering?: (
    weekday: number,
    date: string,
  ) => Promise<void>;
  readonly onCreateArrivalNote?: (
    date: string,
    content: string,
  ) => Promise<void>;
  readonly onUpdateArrivalNote?: (
    date: string,
    noteId: number,
    content: string,
  ) => Promise<void>;
  readonly onDeleteArrivalNote?: (noteId: number) => Promise<void>;
  readonly onCreatePickupNote?: (
    date: string,
    content: string,
  ) => Promise<void>;
  readonly onUpdatePickupNote?: (
    date: string,
    noteId: string,
    content: string,
  ) => Promise<void>;
  readonly onDeletePickupNote?: (noteId: string) => Promise<void>;
}

const TIME_PATTERN = /^([01]?\d|2[0-3]):[0-5]\d$/;
const noopNoteAction = async () => undefined;

interface WeeklyRow {
  readonly weekday: number;
  readonly arrivalTime: string;
  readonly arrivalNotes: string;
  readonly pickupTime: string;
  readonly pickupNotes: string;
}

export function CarePlanEditorModal({
  isOpen,
  onClose,
  date,
  arrivalDay,
  pickupDay,
  weeklyArrival,
  weeklyPickup,
  onSubmitException,
  onSubmitWeekly,
  onResetPickupToOffering,
  onCreateArrivalNote = noopNoteAction,
  onUpdateArrivalNote = noopNoteAction,
  onDeleteArrivalNote = noopNoteAction,
  onCreatePickupNote = noopNoteAction,
  onUpdatePickupNote = noopNoteAction,
  onDeletePickupNote = noopNoteAction,
}: CarePlanEditorModalProps) {
  const toast = useToast();
  const isException =
    date !== null && arrivalDay !== null && pickupDay !== null;

  const [arrivalMode, setArrivalMode] = useState<ArrivalMode>("regular");
  const [arrivalTime, setArrivalTime] = useState("");
  const [arrivalReason, setArrivalReason] = useState("");
  const [pickupMode, setPickupMode] = useState<PickupMode>("regular");
  const [pickupTime, setPickupTime] = useState("");
  const [isResettingPickup, setIsResettingPickup] = useState(false);
  const [pickupReason, setPickupReason] = useState("");
  const [weeklyRows, setWeeklyRows] = useState<WeeklyRow[]>([]);
  const [expandedWeekdays, setExpandedWeekdays] = useState<Set<number>>(
    () => new Set(),
  );
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showRemovalConfirm, setShowRemovalConfirm] = useState(false);
  const [showParentConfirm, setShowParentConfirm] = useState(false);
  const initializedExceptionKey = useRef<string | null>(null);
  const initializedWeeklyEditor = useRef(false);
  // The form steps aside while the confirmation is up so the two dialogs never
  // stack. Kept out of `isOpen` on purpose: the reset effect keys on `isOpen`,
  // so toggling this preserves what the user typed.
  const [formVisible, setFormVisible] = useState(true);

  useEffect(() => {
    if (!isOpen || !isException) {
      initializedExceptionKey.current = null;
      return;
    }

    const exceptionKey = toDayISO(arrivalDay.date);
    if (initializedExceptionKey.current === exceptionKey) return;
    initializedExceptionKey.current = exceptionKey;

    setError(null);
    setShowRemovalConfirm(false);
    setShowParentConfirm(false);
    setFormVisible(true);

    const arrivalInit = arrivalInitialState(arrivalDay);
    setArrivalMode(arrivalInit.mode);
    setArrivalTime(arrivalInit.time);
    setArrivalReason(arrivalInit.reason);

    const pickupInit = pickupInitialState(pickupDay);
    setPickupMode(pickupInit.mode);
    setPickupTime(pickupInit.time);
    setPickupReason(pickupInit.reason);
  }, [isOpen, isException, arrivalDay, pickupDay]);

  useEffect(() => {
    if (!isOpen || isException) {
      initializedWeeklyEditor.current = false;
      return;
    }
    if (initializedWeeklyEditor.current) return;
    initializedWeeklyEditor.current = true;

    setError(null);
    setShowRemovalConfirm(false);
    setShowParentConfirm(false);
    setFormVisible(true);

    const rows = buildWeeklyRows(weeklyArrival, weeklyPickup);
    setWeeklyRows(rows);
    setExpandedWeekdays(
      new Set(
        rows
          .filter((row) => row.arrivalNotes || row.pickupNotes)
          .map((row) => row.weekday),
      ),
    );
  }, [isOpen, isException, weeklyArrival, weeklyPickup]);

  if (!isOpen) return null;

  const arrivalInit = arrivalInitialState(arrivalDay);
  const pickupInit = pickupInitialState(pickupDay);
  // An untouched leg is NOT re-saved: re-saving routes through the staff
  // override path, which reclaims a guardian-authored row and drops the
  // parent's editability.
  const arrivalChanged = legChanged(
    { mode: arrivalMode, time: arrivalTime, reason: arrivalReason },
    arrivalInit,
  );
  const pickupChanged = legChanged(
    { mode: pickupMode, time: pickupTime, reason: pickupReason },
    pickupInit,
  );

  const weeklyRemovals = collectWeeklyRemovals(
    weeklyRows,
    weeklyArrival,
    weeklyPickup,
  );

  const parentAuthored =
    arrivalDay?.exception?.source === "guardian" ||
    pickupDay?.exception?.source === "guardian";
  const overwritesParent =
    (arrivalDay?.exception?.source === "guardian" && arrivalChanged) ||
    (pickupDay?.exception?.source === "guardian" && pickupChanged);

  const title = isException
    ? `Ausnahme für ${getWeekdayLabel(arrivalDay.weekday)}, ${formatShortDate(arrivalDay.date)}`
    : "Wochenplan bearbeiten";

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);

    if (!isException) {
      const invalidWeekly = validateWeeklyRows(weeklyRows);
      if (invalidWeekly) {
        setError(invalidWeekly);
        return;
      }
      if (weeklyRemovals.length > 0) {
        setFormVisible(false);
        setShowRemovalConfirm(true);
        return;
      }
      void performSave();
      return;
    }

    if (arrivalMode === "time" && !TIME_PATTERN.test(arrivalTime)) {
      setError("Bitte eine gültige Ankunftszeit eingeben.");
      return;
    }
    if (pickupMode === "time" && !TIME_PATTERN.test(pickupTime)) {
      setError("Bitte eine gültige Abholzeit eingeben.");
      return;
    }
    if (!arrivalChanged && !pickupChanged) {
      setError("Bitte zuerst eine Zeit ändern.");
      return;
    }
    if (overwritesParent) {
      setFormVisible(false);
      setShowRemovalConfirm(false);
      setShowParentConfirm(true);
      return;
    }

    void performSave();
  };

  const cancelConfirm = () => {
    setShowRemovalConfirm(false);
    setShowParentConfirm(false);
    setFormVisible(true);
  };

  const handleNoteError = (err: unknown) => {
    const message =
      err instanceof Error
        ? err.message
        : "Hinweis konnte nicht gespeichert werden";
    setError(message);
    toast.error(message);
  };

  const handleResetPickupToOffering = async (weekday: number, date: string) => {
    if (!onResetPickupToOffering) return;
    setError(null);
    setIsResettingPickup(true);
    try {
      await onResetPickupToOffering(weekday, date);
    } catch {
      const message =
        "Die Abholung konnte nicht zurückgesetzt werden. Bitte versuchen Sie es noch einmal.";
      setError(message);
      toast.error(message);
    } finally {
      setIsResettingPickup(false);
    }
  };

  const performSave = async () => {
    setError(null);
    setIsSubmitting(true);
    try {
      if (!isException) {
        await onSubmitWeekly(toWeeklySubmit(weeklyRows));
        toast.success("Wochenplan wurde gespeichert");
      } else {
        await onSubmitException({
          date: toDayISO(arrivalDay.date),
          arrival: arrivalChanged
            ? toLegSubmit(arrivalMode, arrivalTime, arrivalReason)
            : null,
          pickup: pickupChanged
            ? toLegSubmit(pickupMode, pickupTime, pickupReason)
            : null,
        });
        toast.success("Ausnahme wurde gespeichert");
      }
      setShowParentConfirm(false);
      onClose();
    } catch (err) {
      const raw =
        err instanceof Error
          ? err.message
          : "Änderung konnte nicht gespeichert werden";
      // The backend refuses to let an account without a staff profile overwrite
      // a parent-set time. Surface that as a readable reason, not a raw 403.
      const message = raw.includes("staff_profile_required")
        ? "Diese Zeit wurde von den Eltern gesetzt und kann nur von Mitarbeitenden mit Personalprofil geändert werden."
        : raw;
      setError(message);
      toast.error(message);
      cancelConfirm();
    } finally {
      setIsSubmitting(false);
    }
  };

  const footer = (
    <>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onClose}
        disabled={isSubmitting}
      >
        Abbrechen
      </Button>
      <Button
        type="submit"
        size="md"
        form="care-plan-editor-form"
        className="gap-2"
        disabled={isSubmitting}
      >
        {isSubmitting ? (
          <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
        ) : null}
        Speichern
      </Button>
    </>
  );

  return (
    <>
      <FormModal
        isOpen={formVisible}
        onClose={onClose}
        title={title}
        footer={footer}
        size={isException ? "lg" : "xl"}
        mobilePosition="bottom"
      >
        {/* noValidate: a half-cleared <input type="time"> (e.g. backspacing the
            hour of "15:00") reports validity.badInput, which makes the browser
            refuse the submit — the Save button then does nothing beyond a native
            bubble on an off-screen field. Such a field reads back as "", which
            is the removal the warning below already announces. */}
        <form
          id="care-plan-editor-form"
          noValidate
          onSubmit={handleSubmit}
          className="space-y-5"
        >
          {error ? (
            <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-xl border px-4 py-3 text-sm">
              {error}
            </div>
          ) : null}

          {isException ? (
            <>
              <p className="text-sm leading-6 text-gray-600">
                Gilt nur an diesem Tag. Die festen Zeiten der Woche bleiben
                unverändert.
              </p>

              {parentAuthored ? (
                <div className="border-moto-blue/20 bg-moto-blue/10 text-moto-blue-hover flex items-start gap-2.5 rounded-xl border px-4 py-3 text-sm">
                  <MotoConceptIcon
                    concept="parents"
                    size={18}
                    className="mt-0.5"
                  />
                  <span>
                    Diese Zeiten wurden von den Eltern über das Elternportal
                    gesetzt. Wenn du sie änderst, ersetzt deine Eingabe die
                    Angabe der Eltern.
                  </span>
                </div>
              ) : null}

              <div className="grid gap-3 sm:grid-cols-2">
                <LegSection
                  label="Ankunft"
                  icon={<Clock className="h-4 w-4" aria-hidden="true" />}
                  regularLabel={`Regulär: ${formatRegularArrival(arrivalDay)}`}
                  mode={arrivalMode}
                  onModeChange={(mode) => setArrivalMode(mode as ArrivalMode)}
                  options={[
                    ["regular", "Regulär"],
                    ["time", "Andere Zeit"],
                    ["absent", "Kommt nicht"],
                  ]}
                  time={arrivalTime}
                  onTimeChange={setArrivalTime}
                  reason={arrivalReason}
                  onReasonChange={setArrivalReason}
                  showTime={arrivalMode === "time"}
                  showReason={arrivalMode !== "regular"}
                />
                <LegSection
                  label="Abholung"
                  icon={<MotoConceptIcon concept="pickup" size={18} />}
                  regularLabel={`Regulär: ${formatRegularPickup(pickupDay)}`}
                  mode={pickupMode}
                  onModeChange={(mode) => setPickupMode(mode as PickupMode)}
                  options={[
                    ["regular", "Regulär"],
                    ["time", "Andere Zeit"],
                    ["none", "Keine Abholung"],
                  ]}
                  time={pickupTime}
                  onTimeChange={setPickupTime}
                  reason={pickupReason}
                  onReasonChange={setPickupReason}
                  showTime={pickupMode === "time"}
                  showReason={pickupMode !== "regular"}
                />
              </div>
              {pickupPulledForward(pickupMode, pickupTime, pickupDay) ? (
                <div className="border-moto-amber/40 bg-moto-amber-soft text-moto-amber-strong flex items-start gap-2.5 rounded-xl border px-4 py-3 text-sm">
                  <Clock
                    className="mt-0.5 h-4 w-4 shrink-0"
                    aria-hidden="true"
                  />
                  <span>
                    Abholung vorverlegt: Betreuungsblöcke, die um {pickupTime}{" "}
                    Uhr oder später beginnen, werden für diesen Tag automatisch
                    abgemeldet (entschuldigt). Wird die Zeit wieder geändert
                    oder die Ausnahme entfernt, gilt der reguläre Plan.
                  </span>
                </div>
              ) : null}
              {onResetPickupToOffering &&
              pickupDay?.baseSchedule &&
              pickupDay.offeringSchedule &&
              pickupDay.baseSchedule.source !== "care_offering" ? (
                <div className="flex justify-end">
                  <Button
                    type="button"
                    variant="ghost"
                    size="compact"
                    disabled={isResettingPickup}
                    onClick={() =>
                      void handleResetPickupToOffering(
                        pickupDay.weekday,
                        toDayISO(pickupDay.date),
                      )
                    }
                  >
                    {isResettingPickup
                      ? "Setzt zurück…"
                      : "Abholung auf Angebots-Gehzeit zurücksetzen"}
                  </Button>
                </div>
              ) : null}
              <DayNotesEditor
                key={toDayISO(arrivalDay.date)}
                date={toDayISO(arrivalDay.date)}
                arrivalNotes={arrivalDay.notes}
                pickupNotes={pickupDay.notes}
                onCreateArrival={onCreateArrivalNote}
                onUpdateArrival={onUpdateArrivalNote}
                onDeleteArrival={onDeleteArrivalNote}
                onCreatePickup={onCreatePickupNote}
                onUpdatePickup={onUpdatePickupNote}
                onDeletePickup={onDeletePickupNote}
                onError={handleNoteError}
              />
            </>
          ) : (
            <WeeklySection
              rows={weeklyRows}
              expandedWeekdays={expandedWeekdays}
              removals={weeklyRemovals}
              onToggleNotes={(weekday) =>
                setExpandedWeekdays((current) => {
                  const next = new Set(current);
                  if (next.has(weekday)) {
                    next.delete(weekday);
                  } else {
                    next.add(weekday);
                  }
                  return next;
                })
              }
              onChange={(weekday, field, value) =>
                setWeeklyRows((rows) =>
                  rows.map((row) =>
                    row.weekday === weekday ? { ...row, [field]: value } : row,
                  ),
                )
              }
            />
          )}
        </form>
      </FormModal>

      <ConfirmationModal
        isOpen={showParentConfirm}
        onClose={cancelConfirm}
        onConfirm={() => void performSave()}
        title="Eltern-Angabe überschreiben?"
        confirmText="Trotzdem überschreiben"
        cancelText="Abbrechen"
        isConfirmLoading={isSubmitting}
        confirmButtonClass="bg-moto-red hover:bg-moto-red-hover"
      >
        <p className="text-sm leading-6 text-gray-600">
          Du überschreibst eine von den Eltern gesetzte Zeit. Die ursprüngliche
          Angabe wird ersetzt und der Tag gilt anschließend als von der
          Einrichtung geändert.
        </p>
      </ConfirmationModal>

      <ConfirmationModal
        isOpen={showRemovalConfirm}
        onClose={cancelConfirm}
        onConfirm={() => void performSave()}
        title="Zeiten entfernen?"
        confirmText="Trotzdem speichern"
        cancelText="Zurück"
        isConfirmLoading={isSubmitting}
        confirmButtonClass="bg-moto-red hover:bg-moto-red-hover"
      >
        <div className="space-y-2 text-sm leading-6 text-gray-600">
          <p>
            Diese Zeiten sind im Wochenplan hinterlegt und werden durch das
            Speichern entfernt:
          </p>
          <ul className="list-inside list-disc font-semibold text-gray-800">
            {weeklyRemovals.map((removal) => (
              <li key={removal}>{removal}</li>
            ))}
          </ul>
        </div>
      </ConfirmationModal>
    </>
  );
}

function LegSection({
  label,
  icon,
  regularLabel,
  mode,
  onModeChange,
  options,
  time,
  onTimeChange,
  reason,
  onReasonChange,
  showTime,
  showReason,
}: {
  readonly label: string;
  readonly icon: React.ReactNode;
  readonly regularLabel: string;
  readonly mode: string;
  readonly onModeChange: (mode: string) => void;
  readonly options: ReadonlyArray<readonly [string, string]>;
  readonly time: string;
  readonly onTimeChange: (value: string) => void;
  readonly reason: string;
  readonly onReasonChange: (value: string) => void;
  readonly showTime: boolean;
  readonly showReason: boolean;
}) {
  const timeId = `exception-${label.toLowerCase()}-time`;
  const reasonId = `exception-${label.toLowerCase()}-reason`;
  return (
    <section className="rounded-xl border border-gray-200 bg-white p-3 shadow-sm sm:rounded-2xl sm:p-4">
      <div className="mb-3 flex items-start gap-3">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-gray-100">
          {icon}
        </span>
        <div>
          <h3 className="text-sm font-semibold text-gray-900">{label}</h3>
          <p className="text-xs text-gray-500">{regularLabel}</p>
        </div>
      </div>

      <div className="grid gap-2">
        {options.map(([value, text]) => (
          <Button
            key={value}
            type="button"
            size="md"
            variant={mode === value ? "primary" : "outline"}
            className="w-full justify-start"
            onClick={() => onModeChange(value)}
          >
            {text}
          </Button>
        ))}
      </div>

      {showTime ? (
        <label className="mt-3 block" htmlFor={timeId}>
          <span className="mb-1 block text-xs font-medium text-gray-500">
            Uhrzeit
          </span>
          <input
            id={timeId}
            type="time"
            value={time}
            onChange={(event) => onTimeChange(event.target.value)}
            className="h-11 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none sm:h-10"
          />
        </label>
      ) : null}

      {/* "Grund", not "Notiz": on a day the free text explains the deviation,
          and calling it a note was half of what made #893 confusing. Notes live
          in the weekly plan and mean "every week". */}
      {showReason ? (
        <label className="mt-3 block" htmlFor={reasonId}>
          <span className="mb-1 block text-xs font-medium text-gray-500">
            Grund
          </span>
          <input
            id={reasonId}
            type="text"
            value={reason}
            onChange={(event) => onReasonChange(event.target.value)}
            maxLength={255}
            className="h-11 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none sm:h-10"
            placeholder="Optional"
          />
        </label>
      ) : null}
    </section>
  );
}

function DayNotesEditor({
  date,
  arrivalNotes,
  pickupNotes,
  onCreateArrival,
  onUpdateArrival,
  onDeleteArrival,
  onCreatePickup,
  onUpdatePickup,
  onDeletePickup,
  onError,
}: {
  readonly date: string;
  readonly arrivalNotes: readonly { id: number; content: string }[];
  readonly pickupNotes: readonly { id: string; content: string }[];
  readonly onCreateArrival: (date: string, content: string) => Promise<void>;
  readonly onUpdateArrival: (
    date: string,
    id: number,
    content: string,
  ) => Promise<void>;
  readonly onDeleteArrival: (id: number) => Promise<void>;
  readonly onCreatePickup: (date: string, content: string) => Promise<void>;
  readonly onUpdatePickup: (
    date: string,
    id: string,
    content: string,
  ) => Promise<void>;
  readonly onDeletePickup: (id: string) => Promise<void>;
  readonly onError: (err: unknown) => void;
}) {
  const [arrivalDraft, setArrivalDraft] = useState("");
  const [pickupDraft, setPickupDraft] = useState("");
  return (
    <div className="space-y-3 rounded-xl border border-gray-200 p-4">
      <p className="text-sm font-semibold text-gray-900">
        Hinweise nur für diesen Tag
      </p>
      <NoteList
        label="Ankunft"
        notes={arrivalNotes}
        draft={arrivalDraft}
        setDraft={setArrivalDraft}
        onCreate={() => onCreateArrival(date, arrivalDraft)}
        onUpdate={(id, content) => onUpdateArrival(date, Number(id), content)}
        onDelete={(id) => onDeleteArrival(Number(id))}
        onError={onError}
      />
      <NoteList
        label="Abholung"
        notes={pickupNotes}
        draft={pickupDraft}
        setDraft={setPickupDraft}
        onCreate={() => onCreatePickup(date, pickupDraft)}
        onUpdate={(id, content) => onUpdatePickup(date, id, content)}
        onDelete={onDeletePickup}
        onError={onError}
      />
    </div>
  );
}

function NoteList({
  label,
  notes,
  draft,
  setDraft,
  onCreate,
  onUpdate,
  onDelete,
  onError,
}: {
  readonly label: string;
  readonly notes: readonly { id: string | number; content: string }[];
  readonly draft: string;
  readonly setDraft: (value: string) => void;
  readonly onCreate: () => Promise<void>;
  readonly onUpdate: (id: string, content: string) => Promise<void>;
  readonly onDelete: (id: string) => Promise<void>;
  readonly onError: (err: unknown) => void;
}) {
  const mutationInFlight = useRef(false);
  const [isMutationPending, setIsMutationPending] = useState(false);

  const runMutation = async (
    mutation: () => Promise<void>,
    onSuccess?: () => void,
  ) => {
    if (mutationInFlight.current) return;

    mutationInFlight.current = true;
    setIsMutationPending(true);
    try {
      await mutation();
      onSuccess?.();
    } catch (err) {
      onError(err);
    } finally {
      mutationInFlight.current = false;
      setIsMutationPending(false);
    }
  };

  return (
    <div className="space-y-2">
      <p className="text-xs font-medium text-gray-500">{label}</p>
      {notes.map((note) => (
        <NoteEditor
          key={note.id}
          label={label}
          note={note}
          onUpdate={onUpdate}
          onDelete={onDelete}
          runMutation={runMutation}
          isMutationPending={isMutationPending}
        />
      ))}
      <div className="flex gap-2">
        <input
          aria-label={`${label} Hinweis hinzufügen`}
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          maxLength={500}
          className="min-w-0 flex-1 rounded border px-2 py-1 text-sm"
        />
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={!draft.trim() || isMutationPending}
          onClick={() => void runMutation(onCreate, () => setDraft(""))}
        >
          Hinzufügen
        </Button>
      </div>
    </div>
  );
}

function NoteEditor({
  label,
  note,
  onUpdate,
  onDelete,
  runMutation,
  isMutationPending,
}: {
  readonly label: string;
  readonly note: { id: string | number; content: string };
  readonly onUpdate: (id: string, content: string) => Promise<void>;
  readonly onDelete: (id: string) => Promise<void>;
  readonly runMutation: (
    mutation: () => Promise<void>,
    onSuccess?: () => void,
  ) => Promise<void>;
  readonly isMutationPending: boolean;
}) {
  const [content, setContent] = useState(note.content);
  const trimmedContent = content.trim();
  const hasChanges = trimmedContent !== note.content;

  return (
    <div className="flex gap-2">
      <input
        aria-label={`${label} Hinweis`}
        value={content}
        onChange={(event) => setContent(event.target.value)}
        maxLength={500}
        className="min-w-0 flex-1 rounded border px-2 py-1 text-sm"
      />
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={!trimmedContent || !hasChanges || isMutationPending}
        onClick={() =>
          void runMutation(
            () => onUpdate(String(note.id), trimmedContent),
            () => setContent(trimmedContent),
          )
        }
      >
        Änderung speichern
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        disabled={isMutationPending}
        onClick={() => void runMutation(() => onDelete(String(note.id)))}
      >
        Löschen
      </Button>
    </div>
  );
}

function WeeklySection({
  rows,
  expandedWeekdays,
  removals,
  onToggleNotes,
  onChange,
}: {
  readonly rows: WeeklyRow[];
  readonly expandedWeekdays: Set<number>;
  readonly removals: string[];
  readonly onToggleNotes: (weekday: number) => void;
  readonly onChange: (
    weekday: number,
    field: "arrivalTime" | "pickupTime" | "arrivalNotes" | "pickupNotes",
    value: string,
  ) => void;
}) {
  return (
    <div className="space-y-3">
      <p className="text-sm leading-6 text-gray-600">
        Gilt ab sofort für alle kommenden Wochen. Bereits eingetragene Ausnahmen
        bleiben bestehen.
      </p>

      <div className="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm sm:rounded-2xl">
        <div className="hidden grid-cols-[minmax(100px,0.7fr)_minmax(140px,1fr)_minmax(140px,1fr)] gap-3 border-b border-gray-100 bg-gray-50 px-4 py-3 text-xs font-semibold tracking-wide text-gray-500 uppercase sm:grid">
          <span>Tag</span>
          <span>Ankunft</span>
          <span>Abholung</span>
        </div>
        <div className="divide-y divide-gray-100">
          {WEEKDAYS.map((day) => {
            const row = rows.find((entry) => entry.weekday === day.value);
            return (
              <div
                key={day.value}
                className="grid gap-3 px-3 py-4 sm:grid-cols-[minmax(100px,0.7fr)_minmax(140px,1fr)_minmax(140px,1fr)] sm:items-center sm:px-4"
              >
                <div>
                  <div className="text-sm font-semibold text-gray-900">
                    {day.label}
                  </div>
                  <button
                    type="button"
                    onClick={() => onToggleNotes(day.value)}
                    className="mt-1 inline-flex items-center gap-1 text-xs font-medium text-gray-500 transition-colors hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                  >
                    <StickyNote className="h-3.5 w-3.5" aria-hidden="true" />
                    Notizen
                    <ChevronDown
                      className={`h-3.5 w-3.5 transition-transform ${
                        expandedWeekdays.has(day.value) ? "rotate-180" : ""
                      }`}
                      aria-hidden="true"
                    />
                  </button>
                </div>
                <WeeklyTimeField
                  id={`weekly-arrival-${day.value}`}
                  label="Ankunft"
                  value={row?.arrivalTime ?? ""}
                  onChange={(value) =>
                    onChange(day.value, "arrivalTime", value)
                  }
                />
                <WeeklyTimeField
                  id={`weekly-pickup-${day.value}`}
                  label="Abholung"
                  value={row?.pickupTime ?? ""}
                  onChange={(value) => onChange(day.value, "pickupTime", value)}
                />
                {expandedWeekdays.has(day.value) ? (
                  <div className="grid gap-3 sm:col-span-3 sm:grid-cols-[minmax(100px,0.7fr)_minmax(140px,1fr)_minmax(140px,1fr)]">
                    <div className="hidden sm:block" />
                    <WeeklyNoteField
                      id={`weekly-arrival-notes-${day.value}`}
                      label="Ankunftsnotiz (jede Woche)"
                      value={row?.arrivalNotes ?? ""}
                      onChange={(value) =>
                        onChange(day.value, "arrivalNotes", value)
                      }
                    />
                    <WeeklyNoteField
                      id={`weekly-pickup-notes-${day.value}`}
                      label="Abholnotiz (jede Woche)"
                      value={row?.pickupNotes ?? ""}
                      onChange={(value) =>
                        onChange(day.value, "pickupNotes", value)
                      }
                    />
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      </div>

      {removals.length > 0 ? (
        <div className="border-moto-orange/25 bg-moto-orange/10 text-moto-orange-strong rounded-xl border px-4 py-3 text-sm">
          Wird beim Speichern entfernt: {removals.join(", ")}.
        </div>
      ) : null}
    </div>
  );
}

function WeeklyNoteField({
  id,
  label,
  value,
  onChange,
}: {
  readonly id: string;
  readonly label: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
}) {
  return (
    <label className="block" htmlFor={id}>
      <span className="mb-1 block text-xs font-medium text-gray-500">
        {label}
      </span>
      <input
        id={id}
        type="text"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        maxLength={500}
        className="h-11 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-900 shadow-sm transition-colors hover:border-gray-300 focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none sm:h-10"
        placeholder="Optional"
      />
    </label>
  );
}

function WeeklyTimeField({
  id,
  label,
  value,
  onChange,
}: {
  readonly id: string;
  readonly label: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
}) {
  return (
    <label className="block" htmlFor={id}>
      <span className="mb-1 block text-xs font-medium text-gray-500 sm:hidden">
        {label}
      </span>
      <input
        id={id}
        type="time"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="h-11 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-900 shadow-sm transition-colors hover:border-gray-300 focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none sm:h-10"
      />
    </label>
  );
}

// --- pure helpers -----------------------------------------------------------

interface LegState {
  readonly mode: string;
  readonly time: string;
  readonly reason: string;
}

function legChanged(current: LegState, initial: LegState): boolean {
  if (current.mode !== initial.mode) return true;
  if (current.mode === "time" && current.time !== initial.time) return true;
  return (
    current.mode !== "regular" &&
    current.reason.trim() !== initial.reason.trim()
  );
}

function toLegSubmit(
  mode: string,
  time: string,
  reason: string,
): CareLegSubmit {
  const trimmed = reason.trim() ? reason.trim() : null;
  if (mode === "regular") return { kind: "regular" };
  if (mode === "time") return { kind: "time", time, reason: trimmed };
  return { kind: "none", reason: trimmed };
}

/** Local calendar day of a Date, never via toISOString (UTC shifts a day). */
function toDayISO(value: Date): string {
  const month = `${value.getMonth() + 1}`.padStart(2, "0");
  const day = `${value.getDate()}`.padStart(2, "0");
  return `${value.getFullYear()}-${month}-${day}`;
}

function arrivalInitialState(day: ArrivalDayData | null): {
  mode: ArrivalMode;
  time: string;
  reason: string;
} {
  if (day?.exception) {
    return {
      mode: day.exception.expected_arrival ? "time" : "absent",
      time: day.exception.expected_arrival?.slice(0, 5) ?? "",
      reason: day.exception.reason ?? "",
    };
  }
  return { mode: "regular", time: day?.effectiveTime ?? "", reason: "" };
}

function pickupInitialState(day: PickupDayData | null): {
  mode: PickupMode;
  time: string;
  reason: string;
} {
  if (day?.exception) {
    return {
      mode: day.exception.pickupTime ? "time" : "none",
      time: day.exception.pickupTime
        ? formatPickupTime(day.exception.pickupTime)
        : "",
      reason: day.exception.reason ?? "",
    };
  }
  return {
    mode: "regular",
    time: day?.effectiveTime ? formatPickupTime(day.effectiveTime) : "",
    reason: "",
  };
}

function formatRegularArrival(day: ArrivalDayData | null): string {
  return day?.baseSchedule?.expected_arrival
    ? day.baseSchedule.expected_arrival.slice(0, 5)
    : "nicht geplant";
}

/**
 * True when the entered day pickup time is EARLIER than the weekly baseline —
 * the backend then derives an automatic partial absence for the blocks after
 * the new time (#2360). Mirrors the backend trigger: no baseline, equal, or
 * later times never couple, so no hint is shown for them.
 */
function pickupPulledForward(
  mode: PickupMode,
  time: string,
  day: PickupDayData | null,
): boolean {
  if (mode !== "time" || !TIME_PATTERN.test(time)) return false;
  if (!day?.baseSchedule?.pickupTime) return false;
  const baseline = formatPickupTime(day.baseSchedule.pickupTime);
  if (!TIME_PATTERN.test(baseline)) return false;
  return time.padStart(5, "0") < baseline.padStart(5, "0");
}

function formatRegularPickup(day: PickupDayData | null): string {
  if (!day?.baseSchedule?.pickupTime) return "nicht geplant";
  const time = formatPickupTime(day.baseSchedule.pickupTime);
  return `${time} (${pickupScheduleSourceLabel(day.baseSchedule)})`;
}

function buildWeeklyRows(
  arrival: readonly ArrivalScheduleFormEntry[],
  pickup: readonly PickupScheduleFormData[],
): WeeklyRow[] {
  return WEEKDAYS.map((day) => {
    const arrivalEntry = arrival.find(
      (schedule) => schedule.weekday === day.value,
    );
    const pickupEntry = pickup.find(
      (schedule) => schedule.weekday === day.value,
    );
    return {
      weekday: day.value,
      arrivalTime: arrivalEntry?.expected_arrival ?? "",
      arrivalNotes: arrivalEntry?.notes ?? "",
      pickupTime: pickupEntry?.pickupTime ?? "",
      pickupNotes: pickupEntry?.notes ?? "",
    };
  });
}

function validateWeeklyRows(rows: readonly WeeklyRow[]): string | null {
  for (const row of rows) {
    const day = WEEKDAYS.find((weekday) => weekday.value === row.weekday);
    if (row.arrivalTime && !TIME_PATTERN.test(row.arrivalTime)) {
      return `Ungültige Ankunftszeit für ${day?.label ?? "diesen Tag"}.`;
    }
    if (row.arrivalNotes.trim() && !row.arrivalTime.trim()) {
      return `Eine Ankunftsnotiz für ${day?.label ?? "diesen Tag"} benötigt eine Ankunftszeit.`;
    }
    if (row.pickupTime && !TIME_PATTERN.test(row.pickupTime)) {
      return `Ungültige Abholzeit für ${day?.label ?? "diesen Tag"}.`;
    }
    if (row.pickupNotes.trim() && !row.pickupTime.trim()) {
      return `Eine Abholnotiz für ${day?.label ?? "diesen Tag"} benötigt eine Abholzeit.`;
    }
  }
  return null;
}

/**
 * Which previously scheduled times an emptied field would delete. The weekly
 * write REPLACES the plan, so clearing a field really does remove that day —
 * this is what the warning and the confirmation are built from, so a mistyped
 * Friday cannot vanish silently.
 */
function collectWeeklyRemovals(
  rows: readonly WeeklyRow[],
  arrival: readonly ArrivalScheduleFormEntry[],
  pickup: readonly PickupScheduleFormData[],
): string[] {
  const removals: string[] = [];
  for (const row of rows) {
    const label =
      WEEKDAYS.find((day) => day.value === row.weekday)?.label ?? "Tag";
    const hadArrival = arrival.some(
      (entry) => entry.weekday === row.weekday && entry.expected_arrival,
    );
    const hadPickup = pickup.some(
      (entry) => entry.weekday === row.weekday && entry.pickupTime,
    );
    if (hadArrival && !row.arrivalTime.trim()) {
      removals.push(`${label} Ankunft`);
    }
    if (hadPickup && !row.pickupTime.trim()) {
      removals.push(`${label} Abholung`);
    }
  }
  return removals;
}

function toWeeklySubmit(rows: readonly WeeklyRow[]): CarePlanWeeklySubmit {
  return {
    arrivalSchedules: rows
      .filter((row) => row.arrivalTime.trim() !== "")
      .map((row) => ({
        weekday: row.weekday,
        inCare: true,
        expected_arrival: row.arrivalTime,
        notes: row.arrivalNotes.trim() ? row.arrivalNotes : null,
      })),
    pickupSchedules: rows
      .filter((row) => row.pickupTime.trim() !== "")
      .map((row) => ({
        weekday: row.weekday,
        pickupTime: row.pickupTime,
        notes: row.pickupNotes.trim() ? row.pickupNotes : undefined,
      })),
  };
}
