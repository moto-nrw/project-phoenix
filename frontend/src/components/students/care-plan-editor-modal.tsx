"use client";

import { useEffect, useRef, useState } from "react";
import { useFormError } from "~/components/ui/form-error";
import { Clock, Loader2 } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import {
  SlideOver,
  SlideOverBody,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import { useToast } from "~/contexts/ToastContext";
import {
  type ArrivalDayData,
  formatShortDate,
  getWeekdayLabel,
} from "~/lib/arrival-schedule-helpers";
import type { DayData as PickupDayData } from "~/lib/pickup-schedule-helpers";
import {
  formatPickupTime,
  pickupScheduleSourceLabel,
} from "~/lib/pickup-schedule-helpers";

/**
 * The care plan is edited through exactly two doors, and each one owns one kind
 * of thing (issue #893):
 *
 *   day card                  -> an EXCEPTION for that date, with a Grund
 *   "Wochenplan bearbeiten"   -> the recurring times of the week, with Notizen
 *
 * That split is the whole fix: before, two identical-looking pencils both led
 * to dialogs that mixed exceptions and notes, and a note gave no hint whether
 * it applied once or every week. Since #3119 only the exception lives in this
 * slide-over; the weekly plan is edited in place in the section
 * (`care-weekly-plan-editor.tsx`, Bauart 2 Regel 3).
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

interface NoteDeletionTarget {
  readonly content: string;
  readonly deleteNote: () => Promise<void>;
}

interface CarePlanEditorModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  /** The day the editor was opened from; both null while closed. */
  readonly arrivalDay: ArrivalDayData | null;
  readonly pickupDay: PickupDayData | null;
  readonly onSubmitException: (payload: CareExceptionSubmit) => Promise<void>;
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

export function CarePlanEditorModal({
  isOpen,
  onClose,
  arrivalDay,
  pickupDay,
  onSubmitException,
  onResetPickupToOffering,
  onCreateArrivalNote = noopNoteAction,
  onUpdateArrivalNote = noopNoteAction,
  onDeleteArrivalNote = noopNoteAction,
  onCreatePickupNote = noopNoteAction,
  onUpdatePickupNote = noopNoteAction,
  onDeletePickupNote = noopNoteAction,
}: CarePlanEditorModalProps) {
  const toast = useToast();
  const isException = arrivalDay !== null && pickupDay !== null;

  const [arrivalMode, setArrivalMode] = useState<ArrivalMode>("regular");
  const [arrivalTime, setArrivalTime] = useState("");
  const [arrivalReason, setArrivalReason] = useState("");
  const [pickupMode, setPickupMode] = useState<PickupMode>("regular");
  const [pickupTime, setPickupTime] = useState("");
  const [isResettingPickup, setIsResettingPickup] = useState(false);
  const [pickupReason, setPickupReason] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useFormError();
  const [showParentConfirm, setShowParentConfirm] = useState(false);
  const [noteDeletionTarget, setNoteDeletionTarget] =
    useState<NoteDeletionTarget | null>(null);
  const [isDeletingNote, setIsDeletingNote] = useState(false);
  const [noteDrafts, setNoteDrafts] = useState<Record<string, string>>({});
  const initializedExceptionKey = useRef<string | null>(null);
  // The form steps aside while the confirmation is up so the two dialogs never
  // stack. Kept out of `isOpen` on purpose: the reset effect keys on `isOpen`,
  // so toggling this preserves what the user typed.
  const [formVisible, setFormVisible] = useState(true);

  useEffect(() => {
    if (!isOpen || !isException) {
      initializedExceptionKey.current = null;
      setNoteDrafts({});
      return;
    }

    const exceptionKey = toDayISO(arrivalDay.date);
    if (initializedExceptionKey.current === exceptionKey) return;
    initializedExceptionKey.current = exceptionKey;

    setError(null);
    setShowParentConfirm(false);
    setNoteDeletionTarget(null);
    setIsDeletingNote(false);
    setFormVisible(true);

    const arrivalInit = arrivalInitialState(arrivalDay);
    setArrivalMode(arrivalInit.mode);
    setArrivalTime(arrivalInit.time);
    setArrivalReason(arrivalInit.reason);

    const pickupInit = pickupInitialState(pickupDay);
    setPickupMode(pickupInit.mode);
    setPickupTime(pickupInit.time);
    setPickupReason(pickupInit.reason);
  }, [isOpen, isException, arrivalDay, pickupDay, setError]);

  if (!isOpen || !arrivalDay || !pickupDay) return null;

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

  const parentAuthored =
    arrivalDay.exception?.source === "guardian" ||
    pickupDay.exception?.source === "guardian";
  const overwritesParent =
    (arrivalDay.exception?.source === "guardian" && arrivalChanged) ||
    (pickupDay.exception?.source === "guardian" && pickupChanged);

  const title = `Ausnahme für ${getWeekdayLabel(arrivalDay.weekday)}, ${formatShortDate(arrivalDay.date)}`;

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);

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
      setShowParentConfirm(true);
      return;
    }

    void performSave();
  };

  const cancelConfirm = () => {
    setShowParentConfirm(false);
    setFormVisible(true);
  };

  const handleNoteError = (err: unknown) => {
    const message =
      err instanceof Error
        ? err.message
        : "Hinweis konnte nicht gespeichert werden";
    setError(message);
  };

  const closeNoteDeleteConfirmation = () => {
    setNoteDeletionTarget(null);
    setFormVisible(true);
  };

  const handleNoteDelete = async () => {
    if (!noteDeletionTarget) return;
    setIsDeletingNote(true);
    try {
      await noteDeletionTarget.deleteNote();
    } catch (err) {
      handleNoteError(err);
    } finally {
      setIsDeletingNote(false);
      closeNoteDeleteConfirmation();
    }
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
    } finally {
      setIsResettingPickup(false);
    }
  };

  const performSave = async () => {
    setError(null);
    setIsSubmitting(true);
    try {
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
      <SlideOver
        open={formVisible}
        onOpenChange={(open) => {
          if (!open) onClose();
        }}
      >
        <SlideOverContent widthClass="sm:w-[640px]">
          <SlideOverHeader className="flex-row items-start justify-between gap-3">
            <div className="min-w-0">
              <SlideOverTitle>{title}</SlideOverTitle>
            </div>
            <SlideOverCloseButton aria-label="Fenster schließen" />
          </SlideOverHeader>
          {/* Speicher- und Hinweisfehler stehen im Fehler-Slot oben im Rumpf
              (Bauart 2 Regel 5), nicht als Toast. */}
          <SlideOverBody error={error}>
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
              pickupDay.baseSchedule &&
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
                noteDrafts={noteDrafts}
                onNoteDraftChange={(key, content) =>
                  setNoteDrafts((drafts) => ({ ...drafts, [key]: content }))
                }
                onRequestDelete={(target) => {
                  setFormVisible(false);
                  setNoteDeletionTarget(target);
                }}
              />
            </form>
          </SlideOverBody>
          <SlideOverFooter className="flex-row justify-end gap-2">
            {footer}
          </SlideOverFooter>
        </SlideOverContent>
      </SlideOver>

      <ConfirmationModal
        isOpen={showParentConfirm}
        onClose={cancelConfirm}
        onConfirm={() => void performSave()}
        title="Eltern-Angabe überschreiben?"
        confirmText="Trotzdem überschreiben"
        cancelText="Abbrechen"
        isConfirmLoading={isSubmitting}
        confirmVariant="danger"
      >
        <p className="text-sm leading-6 text-gray-600">
          Du überschreibst eine von den Eltern gesetzte Zeit. Die ursprüngliche
          Angabe wird ersetzt und der Tag gilt anschließend als von der
          Einrichtung geändert.
        </p>
      </ConfirmationModal>

      <ConfirmDeleteModal
        isOpen={noteDeletionTarget !== null}
        title="Hinweis löschen?"
        description={
          <p>
            Der Hinweis{" "}
            <span className="font-medium text-gray-900">
              „{noteDeletionTarget?.content}“
            </span>{" "}
            wird gelöscht.
          </p>
        }
        gate={{ mode: "twoStep" }}
        loading={isDeletingNote}
        error=""
        onConfirm={() => void handleNoteDelete()}
        onClose={closeNoteDeleteConfirmation}
      />
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
    <section className="moto-content-surface rounded-xl border p-3 shadow-sm sm:rounded-2xl sm:p-4">
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
  noteDrafts,
  onNoteDraftChange,
  onRequestDelete,
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
  readonly noteDrafts: Readonly<Record<string, string>>;
  readonly onNoteDraftChange: (key: string, content: string) => void;
  readonly onRequestDelete: (target: NoteDeletionTarget) => void;
}) {
  return (
    <div className="space-y-3 rounded-xl border border-gray-200 p-4">
      <p className="text-sm font-semibold text-gray-900">
        Hinweise nur für diesen Tag
      </p>
      <NoteList
        label="Ankunft"
        notes={arrivalNotes}
        draft={noteDrafts[`${date}:new:Ankunft`] ?? ""}
        setDraft={(content) =>
          onNoteDraftChange(`${date}:new:Ankunft`, content)
        }
        onCreate={() =>
          onCreateArrival(date, noteDrafts[`${date}:new:Ankunft`] ?? "")
        }
        onUpdate={(id, content) => onUpdateArrival(date, Number(id), content)}
        onDelete={(id) => onDeleteArrival(Number(id))}
        onError={onError}
        noteDrafts={noteDrafts}
        onNoteDraftChange={onNoteDraftChange}
        onRequestDelete={onRequestDelete}
      />
      <NoteList
        label="Abholung"
        notes={pickupNotes}
        draft={noteDrafts[`${date}:new:Abholung`] ?? ""}
        setDraft={(content) =>
          onNoteDraftChange(`${date}:new:Abholung`, content)
        }
        onCreate={() =>
          onCreatePickup(date, noteDrafts[`${date}:new:Abholung`] ?? "")
        }
        onUpdate={(id, content) => onUpdatePickup(date, id, content)}
        onDelete={onDeletePickup}
        onError={onError}
        noteDrafts={noteDrafts}
        onNoteDraftChange={onNoteDraftChange}
        onRequestDelete={onRequestDelete}
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
  noteDrafts,
  onNoteDraftChange,
  onRequestDelete,
}: {
  readonly label: string;
  readonly notes: readonly { id: string | number; content: string }[];
  readonly draft: string;
  readonly setDraft: (value: string) => void;
  readonly onCreate: () => Promise<void>;
  readonly onUpdate: (id: string, content: string) => Promise<void>;
  readonly onDelete: (id: string) => Promise<void>;
  readonly onError: (err: unknown) => void;
  readonly noteDrafts: Readonly<Record<string, string>>;
  readonly onNoteDraftChange: (key: string, content: string) => void;
  readonly onRequestDelete: (target: NoteDeletionTarget) => void;
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
          content={noteDrafts[`${label}:${note.id}`] ?? note.content}
          onContentChange={(content) =>
            onNoteDraftChange(`${label}:${note.id}`, content)
          }
          onUpdate={onUpdate}
          onDelete={onDelete}
          runMutation={runMutation}
          isMutationPending={isMutationPending}
          onRequestDelete={onRequestDelete}
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
  content,
  onContentChange,
  onUpdate,
  onDelete,
  runMutation,
  isMutationPending,
  onRequestDelete,
}: {
  readonly label: string;
  readonly note: { id: string | number; content: string };
  readonly content: string;
  readonly onContentChange: (content: string) => void;
  readonly onUpdate: (id: string, content: string) => Promise<void>;
  readonly onDelete: (id: string) => Promise<void>;
  readonly runMutation: (
    mutation: () => Promise<void>,
    onSuccess?: () => void,
  ) => Promise<void>;
  readonly isMutationPending: boolean;
  readonly onRequestDelete: (target: NoteDeletionTarget) => void;
}) {
  const trimmedContent = content.trim();
  const hasChanges = trimmedContent !== note.content;

  return (
    <div className="flex gap-2">
      <input
        aria-label={`${label} Hinweis`}
        value={content}
        onChange={(event) => onContentChange(event.target.value)}
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
            () => onContentChange(trimmedContent),
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
        onClick={() =>
          onRequestDelete({
            content: note.content,
            deleteNote: () => onDelete(String(note.id)),
          })
        }
      >
        Löschen
      </Button>
    </div>
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
