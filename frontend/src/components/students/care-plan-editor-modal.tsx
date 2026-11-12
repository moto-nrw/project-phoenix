"use client";

import { useEffect, useRef, useState } from "react";
import { Clock } from "lucide-react";
import { Alert } from "~/components/ui/alert";
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
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import {
  type ArrivalDayData,
  formatShortDate,
  getWeekdayLabel,
} from "~/lib/arrival-schedule-helpers";
import type { DayData as PickupDayData } from "~/lib/pickup-schedule-helpers";
import {
  type AllowedDepartureModes,
  type DepartureMode,
  DEPARTURE_MODE_LABELS,
} from "~/lib/departure-plan";
import {
  formatPickupTime,
  pickupScheduleSourceLabel,
} from "~/lib/pickup-schedule-helpers";
import { parentsConcept } from "~/lib/concepts/parents";
import { pickupConcept } from "~/lib/concepts/pickup";
import { IconTile } from "~/components/ui/icon-tile";
import { InsetPanel } from "~/components/ui/inset-panel";

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
type CareLegSubmit =
  | { kind: "regular" }
  | { kind: "time"; time: string; reason: string | null }
  | { kind: "none"; reason: string | null };

/** Die Abholart des Tages (#3945): wie im Wochenplan oder eine eigene. */
type DepartureLegSubmit =
  | { kind: "regular" }
  | { kind: "mode"; mode: DepartureMode; reason: string | null };

export interface CareExceptionSubmit {
  /** The single ISO day this exception applies to. */
  readonly date: string;
  /** null = this leg was not touched and must stay exactly as it is. */
  readonly arrival: CareLegSubmit | null;
  readonly pickup: CareLegSubmit | null;
  readonly departure?: DepartureLegSubmit | null;
}

const DEPARTURE_OPTION_ORDER: readonly DepartureMode[] = [
  "alone",
  "bus",
  "pickup",
  "accompanied",
];
const WEEKDAY_KEYS = ["", "mon", "tue", "wed", "thu", "fri"] as const;

/** „jeden Montag“ für das Übernehmen der gebuchten Abholzeit. */
function weeklyAdverb(weekday: number): string {
  const names = ["", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag"];
  const name = names[weekday];
  return name ? `jeden ${name}` : "in jeder Woche";
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
  /** Abholarten des Wochenplans; nennen „Regulär“ der Abholart (#3945). */
  readonly weeklyDepartureModes?: AllowedDepartureModes;
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
    noteId: string,
    content: string,
  ) => Promise<void>;
  readonly onDeleteArrivalNote?: (noteId: string) => Promise<void>;
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
  weeklyDepartureModes,
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
  const [pickupReason, setPickupReason] = useState("");
  const [departureMode, setDepartureMode] = useState<string>("regular");
  const [departureReason, setDepartureReason] = useState("");
  // Welche wartende Aktion das Panel sperrt; nur für die Ladetexte.
  const [runningAction, setRunningAction] = useState<
    "save" | "resetPickup" | "deleteNote" | null
  >(null);
  const formRef = useRef<HTMLFormElement>(null);
  const errors = useApiFormError(formRef);
  const clearErrors = errors.clear;
  const [showParentConfirm, setShowParentConfirm] = useState(false);
  const [noteDeletionTarget, setNoteDeletionTarget] =
    useState<NoteDeletionTarget | null>(null);
  const [noteDrafts, setNoteDrafts] = useState<Record<string, string>>({});
  const initializedExceptionKey = useRef<string | null>(null);
  // The form steps aside while the confirmation is up so the two dialogs never
  // stack. Kept out of `isOpen` on purpose: the reset effect keys on `isOpen`,
  // so toggling this preserves what the user typed.
  const [formVisible, setFormVisible] = useState(true);
  // Eine Sitzung je geöffnetem Tag. Bewusst `isOpen`, nicht `formVisible`:
  // Bestätigungen über dem Panel blenden es aus, ihr Speichern gehört aber
  // zu derselben Sitzung und hält das Panel gesperrt.
  const save = useFormModalPending(
    isOpen,
    arrivalDay ? toDayISO(arrivalDay.date) : null,
  );
  const isSubmitting = save.pending && runningAction === "save";
  const isResettingPickup = save.pending && runningAction === "resetPickup";
  const isDeletingNote = save.pending && runningAction === "deleteNote";

  useEffect(() => {
    if (!isOpen || !isException) {
      initializedExceptionKey.current = null;
      setNoteDrafts({});
      return;
    }

    const exceptionKey = toDayISO(arrivalDay.date);
    if (initializedExceptionKey.current === exceptionKey) return;
    initializedExceptionKey.current = exceptionKey;

    clearErrors();
    setShowParentConfirm(false);
    setNoteDeletionTarget(null);
    setFormVisible(true);

    const arrivalInit = arrivalInitialState(arrivalDay);
    setArrivalMode(arrivalInit.mode);
    setArrivalTime(arrivalInit.time);
    setArrivalReason(arrivalInit.reason);

    const pickupInit = pickupInitialState(pickupDay);
    setPickupMode(pickupInit.mode);
    setPickupTime(pickupInit.time);
    setPickupReason(pickupInit.reason);

    const departureInit = departureInitialState(pickupDay);
    setDepartureMode(departureInit.mode);
    setDepartureReason(departureInit.reason);
  }, [isOpen, isException, arrivalDay, pickupDay, clearErrors]);

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
  const departureInit = departureInitialState(pickupDay);
  const departureChanged = legChanged(
    { mode: departureMode, time: "", reason: departureReason },
    departureInit,
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
    errors.clear();

    if (arrivalMode === "time" && !TIME_PATTERN.test(arrivalTime)) {
      errors.invalid("Bitte geben Sie eine gültige Ankunftszeit ein.");
      return;
    }
    if (pickupMode === "time" && !TIME_PATTERN.test(pickupTime)) {
      errors.invalid("Bitte geben Sie eine gültige Abholzeit ein.");
      return;
    }
    if (!arrivalChanged && !pickupChanged && !departureChanged) {
      errors.invalid("Bitte ändern Sie zuerst eine Zeit oder die Abholart.");
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
    void errors.show(err, { object: "die Notiz" });
  };

  const runNoteMutation = (
    mutation: () => Promise<void>,
    onSuccess?: () => void,
  ): Promise<void> =>
    save
      .run(async (op) => {
        setRunningAction(null);
        try {
          await mutation();
          if (op.isCurrent()) onSuccess?.();
        } catch (err) {
          if (op.isCurrent()) handleNoteError(err);
        }
      })
      .then(() => undefined);

  const closeNoteDeleteConfirmation = () => {
    setNoteDeletionTarget(null);
    setFormVisible(true);
  };

  const handleNoteDelete = async () => {
    if (!noteDeletionTarget) return;
    const target = noteDeletionTarget;
    await save.run(async (op) => {
      setRunningAction("deleteNote");
      try {
        await target.deleteNote();
      } catch (err) {
        if (op.isCurrent()) handleNoteError(err);
      } finally {
        if (op.isCurrent()) closeNoteDeleteConfirmation();
      }
    });
  };

  const handleResetPickupToOffering = async (weekday: number, date: string) => {
    if (!onResetPickupToOffering) return;
    await save.run(async (op) => {
      setRunningAction("resetPickup");
      errors.clear();
      try {
        await onResetPickupToOffering(weekday, date);
      } catch (err) {
        // Protokolliert; eine späte Ablehnung zeigt kein neuerer Vorgang.
        if (!op.isCurrent()) return;
        await errors.show(err, {
          object: "die Abholung",
          retry: () => void handleResetPickupToOffering(weekday, date),
        });
      }
    });
  };

  const performSave = async () => {
    await save.run(async (op) => {
      setRunningAction("save");
      errors.clear();
      try {
        await onSubmitException({
          date: toDayISO(arrivalDay.date),
          arrival: arrivalChanged
            ? toLegSubmit(arrivalMode, arrivalTime, arrivalReason)
            : null,
          pickup: pickupChanged
            ? toLegSubmit(pickupMode, pickupTime, pickupReason)
            : null,
          ...(departureChanged
            ? { departure: toDepartureSubmit(departureMode, departureReason) }
            : {}),
        });
      } catch (err) {
        // Eine späte Ablehnung gehört nicht in einen neueren Vorgang.
        if (!op.isCurrent()) return;
        cancelConfirm();
        // Ohne „Wiederholen“: „Speichern“ wiederholt mit dem aktuellen
        // Entwurf. Eine Ref dafür bräuchte einen Hook nach dem return null.
        await errors.show(err, {
          object: "die Ausnahme",
        });
        return;
      }
      toast.success("Die Ausnahme ist gespeichert.");
      if (!op.isCurrent()) return;
      setShowParentConfirm(false);
      onClose();
    });
  };

  const footer = (
    <>
      <Button
        type="button"
        variant="outline"
        size="md"
        noTooltip="Schließt nur das Fenster, nichts wird gespeichert."
        onClick={onClose}
        disabled={save.pending}
      >
        Abbrechen
      </Button>
      <Button
        type="submit"
        size="md"
        form="care-plan-editor-form"
        isLoading={isSubmitting}
        loadingText="Wird gespeichert …"
        tooltip="Speichert die Abweichung nur für diesen Tag, bei Eltern-Angaben fragt moto nach."
      >
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
        pending={save}
      >
        <SlideOverContent size="md">
          <SlideOverHeader
            close={<SlideOverCloseButton aria-label="Fenster schließen" />}
          >
            <SlideOverTitle>{title}</SlideOverTitle>
          </SlideOverHeader>
          {/* Speicher- und Hinweisfehler stehen im Fehler-Slot oben im Rumpf
              (Bauart 2 Regel 5), nicht als Toast. */}
          <SlideOverBody error={errors.error}>
            {/* noValidate: a half-cleared <input type="time"> (e.g. backspacing the
            hour of "15:00") reports validity.badInput, which makes the browser
            refuse the submit — the Save button then does nothing beyond a native
            bubble on an off-screen field. Such a field reads back as "", which
            is the removal the warning below already announces. */}
            <form
              ref={formRef}
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
                <Alert
                  type="info"
                  announce="off"
                  icon={<MotoConceptIcon concept={parentsConcept} size={18} />}
                  message="Diese Zeiten wurden von den Eltern über das Elternportal gesetzt. Wenn du sie änderst, ersetzt deine Eingabe die Angabe der Eltern."
                />
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
                  icon={<MotoConceptIcon concept={pickupConcept} size={18} />}
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
              <LegSection
                label="Abholart"
                icon={<MotoConceptIcon concept={pickupConcept} size={18} />}
                regularLabel={`Regulär: ${formatRegularDeparture(pickupDay, weeklyDepartureModes)}`}
                mode={departureMode}
                onModeChange={setDepartureMode}
                options={[
                  ["regular", "Wie im Wochenplan"],
                  ...DEPARTURE_OPTION_ORDER.map(
                    (mode) =>
                      [mode, DEPARTURE_MODE_LABELS[mode]] as readonly [
                        string,
                        string,
                      ],
                  ),
                ]}
                optionsClassName="sm:grid-cols-2"
                time=""
                onTimeChange={() => undefined}
                reason={departureReason}
                onReasonChange={setDepartureReason}
                showTime={false}
                showReason={departureMode !== "regular"}
              />
              {pickupPulledForward(pickupMode, pickupTime, pickupDay) ? (
                <Alert
                  type="warning"
                  announce="off"
                  icon={<Clock className="h-4 w-4" aria-hidden="true" />}
                  message={
                    <>
                      Abholung vorverlegt: Betreuungsblöcke, die um {pickupTime}{" "}
                      Uhr oder später beginnen, werden für diesen Tag
                      automatisch abgemeldet (entschuldigt). Wird die Zeit
                      wieder geändert oder die Ausnahme entfernt, gilt der
                      reguläre Plan.
                    </>
                  }
                />
              ) : null}
              {onResetPickupToOffering &&
              pickupDay.baseSchedule &&
              pickupDay.offeringSchedule &&
              pickupDay.baseSchedule.source !== "care_offering" ? (
                // Der Dialog gilt sonst nur für diesen Tag; das Übernehmen
                // ändert aber den Wochenplan ab sofort, also auch vor diesem
                // Tag. Der Satz sagt das und nennt die gebuchte Zeit (#5618).
                // Braucht ein früherer Tag die Zeit noch, lehnt moto ab und
                // nennt den Tag, ab dem es geht (#5643).
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <p className="text-sm text-gray-600">
                    Gebucht:{" "}
                    {formatPickupTime(pickupDay.offeringSchedule.pickupTime)}{" "}
                    Uhr. Übernehmen gilt ab sofort{" "}
                    {weeklyAdverb(pickupDay.weekday)}, nicht nur an diesem Tag.
                  </p>
                  <Button
                    type="button"
                    variant="surface"
                    size="compact"
                    className="ml-auto"
                    tooltip="Setzt die Abholzeit ab sofort jede Woche auf die gebuchte Zeit."
                    disabled={save.pending}
                    onClick={() =>
                      void handleResetPickupToOffering(
                        pickupDay.weekday,
                        toDayISO(pickupDay.date),
                      )
                    }
                  >
                    {isResettingPickup
                      ? "Wird übernommen …"
                      : "Gebuchte Abholzeit übernehmen"}
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
                runMutation={runNoteMutation}
                isMutationPending={save.pending}
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
          <SlideOverFooter>{footer}</SlideOverFooter>
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
        loadingText="Wird gespeichert …"
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
  optionsClassName = "",
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
  /** Zusätzliche Rasterklassen für viele Optionen (Abholart, #3945). */
  readonly optionsClassName?: string;
}) {
  const timeId = `exception-${label.toLowerCase()}-time`;
  const reasonId = `exception-${label.toLowerCase()}-reason`;
  return (
    <InsetPanel as="section" tone="white" padding="md">
      <div className="mb-3 flex items-start gap-3">
        <IconTile size="sm" icon={icon} />
        <div>
          <h3 className="text-sm font-semibold text-gray-900">{label}</h3>
          <p className="text-xs text-gray-500">{regularLabel}</p>
        </div>
      </div>

      <div className={`grid gap-2 ${optionsClassName}`}>
        {options.map(([value, text]) => (
          <Button
            key={value}
            type="button"
            size="md"
            variant={mode === value ? "primary" : "outline"}
            className="w-full justify-start"
            noTooltip="Wählt nur eine Möglichkeit, gespeichert wird erst mit „Speichern“."
            onClick={() => onModeChange(value)}
          >
            {text}
          </Button>
        ))}
      </div>

      {showTime ? (
        <div className="mt-3">
          <Input
            id={timeId}
            label="Uhrzeit"
            controlSize="compact"
            type="time"
            value={time}
            onChange={(event) => onTimeChange(event.target.value)}
          />
        </div>
      ) : null}

      {/* "Grund", not "Notiz": on a day the free text explains the deviation,
          and calling it a note was half of what made #893 confusing. Notes live
          in the weekly plan and mean "every week". */}
      {showReason ? (
        <div className="mt-3">
          <Input
            id={reasonId}
            label="Grund"
            controlSize="compact"
            type="text"
            value={reason}
            onChange={(event) => onReasonChange(event.target.value)}
            maxLength={255}
            placeholder="Optional"
          />
        </div>
      ) : null}
    </InsetPanel>
  );
}

/**
 * Führt eine Hinweis-Änderung über die Sperre des Panels aus (#6593);
 * `onSuccess` läuft nur, solange das Panel noch denselben Tag zeigt.
 */
type NoteMutationRunner = (
  mutation: () => Promise<void>,
  onSuccess?: () => void,
) => Promise<void>;

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
  runMutation,
  isMutationPending,
  noteDrafts,
  onNoteDraftChange,
  onRequestDelete,
}: {
  readonly date: string;
  readonly arrivalNotes: readonly { id: string; content: string }[];
  readonly pickupNotes: readonly { id: string; content: string }[];
  readonly onCreateArrival: (date: string, content: string) => Promise<void>;
  readonly onUpdateArrival: (
    date: string,
    id: string,
    content: string,
  ) => Promise<void>;
  readonly onDeleteArrival: (id: string) => Promise<void>;
  readonly onCreatePickup: (date: string, content: string) => Promise<void>;
  readonly onUpdatePickup: (
    date: string,
    id: string,
    content: string,
  ) => Promise<void>;
  readonly onDeletePickup: (id: string) => Promise<void>;
  readonly runMutation: NoteMutationRunner;
  readonly isMutationPending: boolean;
  readonly noteDrafts: Readonly<Record<string, string>>;
  readonly onNoteDraftChange: (key: string, content: string) => void;
  readonly onRequestDelete: (target: NoteDeletionTarget) => void;
}) {
  return (
    <InsetPanel tone="white" padding="md" className="space-y-3">
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
        onUpdate={(id, content) => onUpdateArrival(date, id, content)}
        onDelete={onDeleteArrival}
        runMutation={runMutation}
        isMutationPending={isMutationPending}
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
        runMutation={runMutation}
        isMutationPending={isMutationPending}
        noteDrafts={noteDrafts}
        onNoteDraftChange={onNoteDraftChange}
        onRequestDelete={onRequestDelete}
      />
    </InsetPanel>
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
  runMutation,
  isMutationPending,
  noteDrafts,
  onNoteDraftChange,
  onRequestDelete,
}: {
  readonly label: string;
  readonly notes: readonly { id: string; content: string }[];
  readonly draft: string;
  readonly setDraft: (value: string) => void;
  readonly onCreate: () => Promise<void>;
  readonly onUpdate: (id: string, content: string) => Promise<void>;
  readonly onDelete: (id: string) => Promise<void>;
  readonly runMutation: NoteMutationRunner;
  readonly isMutationPending: boolean;
  readonly noteDrafts: Readonly<Record<string, string>>;
  readonly onNoteDraftChange: (key: string, content: string) => void;
  readonly onRequestDelete: (target: NoteDeletionTarget) => void;
}) {
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
        <div className="min-w-0 flex-1">
          <Input
            aria-label={`${label} Hinweis hinzufügen`}
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            maxLength={500}
            controlSize="compact"
          />
        </div>
        <Button
          type="button"
          variant="outline"
          size="md"
          tooltip="Speichert den Hinweis sofort für diesen Tag."
          disabled={!draft.trim() || isMutationPending}
          disabledReason={
            draft.trim()
              ? undefined
              : "Schreiben Sie zuerst einen Hinweis in das Feld."
          }
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
  readonly note: { id: string; content: string };
  readonly content: string;
  readonly onContentChange: (content: string) => void;
  readonly onUpdate: (id: string, content: string) => Promise<void>;
  readonly onDelete: (id: string) => Promise<void>;
  readonly runMutation: NoteMutationRunner;
  readonly isMutationPending: boolean;
  readonly onRequestDelete: (target: NoteDeletionTarget) => void;
}) {
  const trimmedContent = content.trim();
  const hasChanges = trimmedContent !== note.content;

  return (
    // Two actions leave the field no room on a phone; it wraps to its own line
    // instead of shrinking to a sliver.
    <div className="flex flex-wrap gap-2">
      <div className="min-w-48 flex-1">
        <Input
          aria-label={`${label} Hinweis`}
          value={content}
          onChange={(event) => onContentChange(event.target.value)}
          maxLength={500}
          controlSize="compact"
        />
      </div>
      <Button
        type="button"
        variant="outline"
        size="md"
        tooltip="Speichert den geänderten Hinweis sofort."
        disabled={!trimmedContent || !hasChanges || isMutationPending}
        disabledReason={
          !trimmedContent
            ? "Schreiben Sie zuerst einen Text in das Feld."
            : !hasChanges
              ? "Ändern Sie zuerst den Text des Hinweises."
              : undefined
        }
        onClick={() =>
          void runMutation(
            () => onUpdate(note.id, trimmedContent),
            () => onContentChange(trimmedContent),
          )
        }
      >
        Änderung speichern
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="md"
        tooltip="Löscht den Hinweis, moto fragt vorher nach."
        disabled={isMutationPending}
        onClick={() =>
          onRequestDelete({
            content: note.content,
            deleteNote: () => onDelete(note.id),
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
      mode: day.exception.expectedArrival ? "time" : "absent",
      time: day.exception.expectedArrival?.slice(0, 5) ?? "",
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

function departureInitialState(day: PickupDayData | null): {
  mode: string;
  time: string;
  reason: string;
} {
  const current = day?.departureException;
  if (current) {
    return {
      mode: current.departureMode,
      time: "",
      reason: current.reason ?? "",
    };
  }
  return { mode: "regular", time: "", reason: "" };
}

function toDepartureSubmit(mode: string, reason: string): DepartureLegSubmit {
  if (mode === "regular") return { kind: "regular" };
  return {
    kind: "mode",
    mode: mode as DepartureMode,
    reason: reason.trim() ? reason.trim() : null,
  };
}

/** Die Abholarten des Wochentags, wie der Wochenplan sie führt. */
function formatRegularDeparture(
  day: PickupDayData | null,
  weekly: AllowedDepartureModes | undefined,
): string {
  const key = WEEKDAY_KEYS[day?.weekday ?? 0];
  const modes = key && weekly ? (weekly[key] ?? []) : [];
  if (modes.length === 0) return "nicht festgelegt";
  return modes.map((mode) => DEPARTURE_MODE_LABELS[mode]).join(", ");
}

function formatRegularArrival(day: ArrivalDayData | null): string {
  return day?.baseSchedule?.expectedArrival
    ? day.baseSchedule.expectedArrival.slice(0, 5)
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
