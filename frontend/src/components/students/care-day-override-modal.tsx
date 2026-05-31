"use client";

import { useEffect, useState } from "react";
import { CalendarDays, Clock, Loader2, StickyNote, Trash2 } from "lucide-react";
import { FormModal } from "~/components/ui/form-modal";
import { useToast } from "~/contexts/ToastContext";
import {
  type ArrivalDayData,
  formatShortDate,
  getWeekdayLabel,
} from "~/lib/arrival-schedule-helpers";
import type { ArrivalNote } from "~/lib/student-arrival-api";
import type {
  DayData as PickupDayData,
  PickupNote,
} from "~/lib/pickup-schedule-helpers";
import { formatPickupTime } from "~/lib/pickup-schedule-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";

type ArrivalMode = "regular" | "time" | "absent";
type PickupMode = "regular" | "time" | "none";
type NoteTarget = "arrival" | "pickup";

interface CareDayOverrideModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly arrivalDay: ArrivalDayData | null;
  readonly pickupDay: PickupDayData | null;
  readonly onSaveArrivalException: (params: {
    expectedArrival: string | null;
    reason?: string | null;
  }) => Promise<void>;
  readonly onDeleteArrivalException: () => Promise<void>;
  readonly onSavePickupException: (params: {
    pickupTime?: string;
    reason?: string;
  }) => Promise<void>;
  readonly onDeletePickupException: () => Promise<void>;
  readonly onCreateArrivalNote: (content: string) => Promise<void>;
  readonly onDeleteArrivalNote: (noteId: number) => Promise<void>;
  readonly onCreatePickupNote: (content: string) => Promise<void>;
  readonly onDeletePickupNote: (noteId: string) => Promise<void>;
}

const TIME_PATTERN = /^([01]?\d|2[0-3]):[0-5]\d$/;

export function CareDayOverrideModal({
  isOpen,
  onClose,
  arrivalDay,
  pickupDay,
  onSaveArrivalException,
  onDeleteArrivalException,
  onSavePickupException,
  onDeletePickupException,
  onCreateArrivalNote,
  onDeleteArrivalNote,
  onCreatePickupNote,
  onDeletePickupNote,
}: CareDayOverrideModalProps) {
  const toast = useToast();
  const [arrivalMode, setArrivalMode] = useState<ArrivalMode>("regular");
  const [arrivalTime, setArrivalTime] = useState("");
  const [arrivalReason, setArrivalReason] = useState("");
  const [pickupMode, setPickupMode] = useState<PickupMode>("regular");
  const [pickupTime, setPickupTime] = useState("");
  const [pickupReason, setPickupReason] = useState("");
  const [noteTarget, setNoteTarget] = useState<NoteTarget>("pickup");
  const [noteContent, setNoteContent] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [deletingNoteKey, setDeletingNoteKey] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isOpen) return;
    if (arrivalDay?.exception) {
      setArrivalMode(arrivalDay.exception.expected_arrival ? "time" : "absent");
      setArrivalTime(arrivalDay.exception.expected_arrival?.slice(0, 5) ?? "");
      setArrivalReason(arrivalDay.exception.reason ?? "");
    } else {
      setArrivalMode("regular");
      setArrivalTime(arrivalDay?.effectiveTime ?? "");
      setArrivalReason("");
    }

    if (pickupDay?.exception) {
      setPickupMode(pickupDay.exception.pickupTime ? "time" : "none");
      setPickupTime(
        pickupDay.exception.pickupTime
          ? formatPickupTime(pickupDay.exception.pickupTime)
          : "",
      );
      setPickupReason(pickupDay.exception.reason ?? "");
    } else {
      setPickupMode("regular");
      setPickupTime(
        pickupDay?.effectiveTime
          ? formatPickupTime(pickupDay.effectiveTime)
          : "",
      );
      setPickupReason("");
    }

    setNoteTarget("pickup");
    setNoteContent("");
    setError(null);
  }, [isOpen, arrivalDay, pickupDay]);

  if (!arrivalDay || !pickupDay) return null;

  const title = `${getWeekdayLabel(arrivalDay.weekday)}, ${formatShortDate(arrivalDay.date)}`;
  const regularArrival = arrivalDay.baseSchedule?.expected_arrival
    ? arrivalDay.baseSchedule.expected_arrival.slice(0, 5)
    : "nicht geplant";
  const regularPickup = pickupDay.baseSchedule?.pickupTime
    ? formatPickupTime(pickupDay.baseSchedule.pickupTime)
    : "nicht geplant";
  const notes = getDayNotes(arrivalDay.notes, pickupDay.notes);

  const handleSubmit = async (event: React.FormEvent) => {
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

    setIsSubmitting(true);
    try {
      if (arrivalMode === "regular") {
        if (arrivalDay.exception) await onDeleteArrivalException();
      } else {
        await onSaveArrivalException({
          expectedArrival: arrivalMode === "absent" ? null : arrivalTime,
          reason: arrivalReason.trim() ? arrivalReason.trim() : null,
        });
      }

      if (pickupMode === "regular") {
        if (pickupDay.exception) await onDeletePickupException();
      } else {
        await onSavePickupException({
          pickupTime: pickupMode === "none" ? undefined : pickupTime,
          reason: pickupReason.trim() ? pickupReason.trim() : undefined,
        });
      }

      if (noteContent.trim()) {
        if (noteTarget === "arrival") {
          await onCreateArrivalNote(noteContent.trim());
        } else {
          await onCreatePickupNote(noteContent.trim());
        }
      }

      toast.success("Tagesänderung wurde gespeichert");
      onClose();
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Tagesänderung konnte nicht gespeichert werden";
      setError(message);
      toast.error(message);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDeleteNote = async (note: CareDayNote) => {
    setDeletingNoteKey(note.key);
    setError(null);
    try {
      if (note.kind === "arrival") {
        await onDeleteArrivalNote(Number(note.id));
      } else {
        await onDeletePickupNote(String(note.id));
      }
      toast.success("Notiz wurde gelöscht");
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Notiz konnte nicht gelöscht werden";
      setError(message);
      toast.error(message);
    } finally {
      setDeletingNoteKey(null);
    }
  };

  const footer = (
    <>
      <button
        type="button"
        onClick={onClose}
        disabled={isSubmitting}
        className="h-10 w-full rounded-lg border border-gray-200 bg-white px-4 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50 sm:w-auto"
      >
        Abbrechen
      </button>
      <button
        type="submit"
        form="care-day-override-form"
        disabled={isSubmitting}
        className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 text-sm font-semibold text-white shadow-sm hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50 sm:w-auto"
      >
        {isSubmitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
        Tagesänderung speichern
      </button>
    </>
  );

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      footer={footer}
      size="lg"
      mobilePosition="bottom"
    >
      <form
        id="care-day-override-form"
        onSubmit={handleSubmit}
        className="space-y-5"
      >
        {error ? (
          <div className="rounded-xl border border-[#FF3130]/20 bg-[#FF3130]/10 px-4 py-3 text-sm text-[#CC2626]">
            {error}
          </div>
        ) : null}

        <div className="grid gap-3 sm:grid-cols-2">
          <DayOverrideSection
            label="Ankunft"
            icon={<Clock className="h-4 w-4" aria-hidden="true" />}
            color={LOCATION_COLORS.GROUP_ROOM}
            regularLabel={`Regulär: ${regularArrival}`}
            mode={arrivalMode}
            onModeChange={(mode) => setArrivalMode(mode as ArrivalMode)}
            options={[
              ["regular", "Regulär"],
              ["time", "Andere Zeit"],
            ]}
            time={arrivalTime}
            onTimeChange={setArrivalTime}
            reason={arrivalReason}
            onReasonChange={setArrivalReason}
            showTime={arrivalMode === "time"}
            showReason={arrivalMode !== "regular"}
          />
          <DayOverrideSection
            label="Abholung"
            icon={<CalendarDays className="h-4 w-4" aria-hidden="true" />}
            color={LOCATION_COLORS.SCHOOLYARD}
            regularLabel={`Regulär: ${regularPickup}`}
            mode={pickupMode}
            onModeChange={(mode) => setPickupMode(mode as PickupMode)}
            options={[
              ["regular", "Regulär"],
              ["time", "Andere Zeit"],
            ]}
            time={pickupTime}
            onTimeChange={setPickupTime}
            reason={pickupReason}
            onReasonChange={setPickupReason}
            showTime={pickupMode === "time"}
            showReason={pickupMode !== "regular"}
          />
        </div>

        <section className="rounded-xl border border-gray-200 bg-white p-3 shadow-sm sm:rounded-2xl sm:p-4">
          <div className="mb-3 flex items-center gap-2">
            <StickyNote className="h-4 w-4 text-gray-400" aria-hidden="true" />
            <h3 className="text-sm font-semibold text-gray-900">Notizen</h3>
          </div>

          {notes.length > 0 ? (
            <div className="mb-4 space-y-2">
              {notes.map((note) => (
                <div
                  key={note.key}
                  className="flex items-start gap-2 rounded-xl bg-gray-50 px-3 py-2 text-sm text-gray-700"
                >
                  <span className="mt-0.5 shrink-0 rounded-full bg-white px-2 py-0.5 text-[11px] font-semibold text-gray-500 shadow-sm">
                    {note.kind === "arrival" ? "Ankunft" : "Abholung"}
                  </span>
                  <span className="min-w-0 flex-1 break-words">
                    {note.content}
                  </span>
                  <button
                    type="button"
                    onClick={() => void handleDeleteNote(note)}
                    disabled={deletingNoteKey === note.key}
                    className="rounded-md p-1.5 text-gray-400 hover:bg-white hover:text-[#CC2626] focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
                    aria-label="Notiz löschen"
                  >
                    {deletingNoteKey === note.key ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Trash2 className="h-3.5 w-3.5" />
                    )}
                  </button>
                </div>
              ))}
            </div>
          ) : null}

          <div className="grid gap-3 sm:grid-cols-[160px_1fr]">
            <label className="block">
              <span className="mb-1 block text-xs font-medium text-gray-500">
                Bereich
              </span>
              <select
                value={noteTarget}
                onChange={(event) =>
                  setNoteTarget(event.target.value as NoteTarget)
                }
                className="moto-select h-11 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none sm:h-10"
              >
                <option value="arrival">Ankunft</option>
                <option value="pickup">Abholung</option>
              </select>
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-medium text-gray-500">
                Neue Notiz
              </span>
              <input
                type="text"
                value={noteContent}
                onChange={(event) => setNoteContent(event.target.value)}
                maxLength={500}
                className="h-11 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none sm:h-10"
                placeholder="Optional"
              />
            </label>
          </div>
        </section>
      </form>
    </FormModal>
  );
}

function DayOverrideSection({
  label,
  icon,
  color,
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
  readonly color: string;
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
  return (
    <section className="rounded-xl border border-gray-200 bg-white p-3 shadow-sm sm:rounded-2xl sm:p-4">
      <div className="mb-3 flex items-start gap-3">
        <span
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg"
          style={{ backgroundColor: `${color}14`, color }}
        >
          {icon}
        </span>
        <div>
          <h3 className="text-sm font-semibold text-gray-900">{label}</h3>
          <p className="text-xs text-gray-500">{regularLabel}</p>
        </div>
      </div>

      <div className="grid gap-2">
        {options.map(([value, text]) => (
          <button
            key={value}
            type="button"
            onClick={() => onModeChange(value)}
            className={`h-10 rounded-lg border px-3 text-left text-sm font-semibold transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:h-9 ${
              mode === value
                ? "border-gray-900 bg-gray-900 text-white"
                : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
            }`}
          >
            {text}
          </button>
        ))}
      </div>

      {showTime ? (
        <label className="mt-3 block">
          <span className="mb-1 block text-xs font-medium text-gray-500">
            Uhrzeit
          </span>
          <input
            type="time"
            value={time}
            onChange={(event) => onTimeChange(event.target.value)}
            className="h-11 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none sm:h-10"
          />
        </label>
      ) : null}

      {showReason ? (
        <label className="mt-3 block">
          <span className="mb-1 block text-xs font-medium text-gray-500">
            Grund
          </span>
          <input
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

interface CareDayNote {
  readonly key: string;
  readonly id: number | string;
  readonly kind: NoteTarget;
  readonly content: string;
}

function getDayNotes(
  arrivalNotes: ArrivalNote[],
  pickupNotes: PickupNote[],
): CareDayNote[] {
  return [
    ...arrivalNotes.map((note) => ({
      key: `arrival-${note.id}`,
      id: note.id,
      kind: "arrival" as const,
      content: note.content,
    })),
    ...pickupNotes.map((note) => ({
      key: `pickup-${note.id}`,
      id: note.id,
      kind: "pickup" as const,
      content: note.content,
    })),
  ];
}
