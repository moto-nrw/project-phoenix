"use client";

import { useMemo, useState } from "react";
import type { DateRange } from "react-day-picker";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { RangeCalendarInline } from "~/components/ui/date-range-picker";
import { Modal } from "~/components/ui/modal";
import { Textarea } from "~/components/ui/textarea";
import { BooleanField } from "~/components/settings/fields/boolean-field";
import { useToast } from "~/contexts/ToastContext";
import { dispatchAbsencesRefresh } from "~/lib/absence-helpers";
import { createLogger } from "~/lib/logger";
import { timeTrackingService } from "~/lib/time-tracking-api";
import type { StaffAbsence } from "~/lib/time-tracking-helpers";

// Forward-looking presets specific to vacation requests. Covers ~90% of
// real-world cases (single Brückentag, current/next week, current/next
// month) so the OGS staff don't have to navigate the calendar for the
// common scenarios.
function buildVacationPresets(today: Date) {
  const startOfWeek = (d: Date) => {
    const x = new Date(d);
    const dow = (x.getDay() + 6) % 7; // Mon = 0
    x.setDate(x.getDate() - dow);
    x.setHours(0, 0, 0, 0);
    return x;
  };
  const endOfFridayOfWeek = (mondayDate: Date) => {
    const x = new Date(mondayDate);
    x.setDate(x.getDate() + 4);
    return x;
  };
  const thisWeekMon = startOfWeek(today);
  const nextWeekMon = new Date(thisWeekMon);
  nextWeekMon.setDate(nextWeekMon.getDate() + 7);
  const tomorrow = new Date(today);
  tomorrow.setDate(tomorrow.getDate() + 1);
  const monthEnd = new Date(today.getFullYear(), today.getMonth() + 1, 0);
  const nextMonthStart = new Date(today.getFullYear(), today.getMonth() + 1, 1);
  const nextMonthEnd = new Date(today.getFullYear(), today.getMonth() + 2, 0);

  return [
    { label: "Heute", range: () => ({ from: today, to: today }) },
    { label: "Morgen", range: () => ({ from: tomorrow, to: tomorrow }) },
    {
      label: "Diese Woche",
      range: () => ({ from: thisWeekMon, to: endOfFridayOfWeek(thisWeekMon) }),
    },
    {
      label: "Nächste Woche",
      range: () => ({ from: nextWeekMon, to: endOfFridayOfWeek(nextWeekMon) }),
    },
    {
      label: "Diesen Monat",
      range: () => ({ from: today, to: monthEnd }),
    },
    {
      label: "Nächsten Monat",
      range: () => ({ from: nextMonthStart, to: nextMonthEnd }),
    },
  ];
}

const logger = createLogger({ component: "VacationRequestModal" });

const BLOCKING_VACATION_STATUSES = new Set([
  "requested",
  "question",
  "approved",
  "reported",
]);

function toIsoDate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function parseLocalDate(iso: string): Date {
  return new Date(`${iso}T00:00:00`);
}

function formatDateLabel(iso: string): string {
  return parseLocalDate(iso).toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

function formatRangeLabel(start: string, end: string): string {
  return start === end
    ? formatDateLabel(start)
    : `${formatDateLabel(start)} bis ${formatDateLabel(end)}`;
}

function vacationStatusLabel(status: string): string {
  switch (status) {
    case "requested":
      return "beantragt";
    case "question":
      return "Rückfrage";
    case "approved":
      return "genehmigt";
    case "reported":
      return "eingetragen";
    default:
      return status;
  }
}

function isBlockingVacation(absence: StaffAbsence): boolean {
  return (
    absence.absenceType === "vacation" &&
    BLOCKING_VACATION_STATUSES.has(absence.status)
  );
}

function rangesOverlap(
  startA: Date,
  endA: Date,
  startB: Date,
  endB: Date,
): boolean {
  return (
    startA.getTime() <= endB.getTime() && startB.getTime() <= endA.getTime()
  );
}

function normalizeVacationRequestError(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  if (
    message.includes("dates overlap with an existing absence") ||
    message.includes("absence overlaps")
  ) {
    return "Der gewählte Zeitraum überschneidet sich mit einem bestehenden Urlaubsantrag.";
  }
  if (message.includes("vacation range contains no working days")) {
    return "Der gewählte Zeitraum enthält keine Werktage.";
  }
  if (message.includes("vacation request must start today or in the future")) {
    return "Urlaub kann nur für heute oder zukünftige Tage beantragt werden.";
  }
  return message || "Antrag konnte nicht gesendet werden.";
}

// Mirrors backend countWorkingDays: full Mon-Fri count minus 0.5 per
// boundary half-day flag. Single-day range collapses both flags to one.
function countWorkingDays(
  from: Date,
  to: Date,
  startHalf: boolean,
  endHalf: boolean,
): number {
  if (to.getTime() < from.getTime()) return 0;
  let count = 0;
  const cursor = new Date(from);
  while (cursor.getTime() <= to.getTime()) {
    const dow = cursor.getDay();
    if (dow !== 0 && dow !== 6) count += 1;
    cursor.setDate(cursor.getDate() + 1);
  }
  let result = count;
  const isSingleDay = from.getTime() === to.getTime();
  if (isSingleDay) {
    if (startHalf || endHalf) result -= 0.5;
    return result;
  }
  if (startHalf) result -= 0.5;
  if (endHalf) result -= 0.5;
  return result;
}

export function VacationRequestModal({
  isOpen,
  onClose,
  onSubmitted,
  remainingDays,
  existingVacations,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSubmitted: () => void;
  readonly remainingDays: number;
  readonly existingVacations: readonly StaffAbsence[];
}) {
  const today = useMemo(() => new Date(), []);
  const vacationPresets = useMemo(() => buildVacationPresets(today), [today]);
  const [range, setRange] = useState<DateRange | undefined>(undefined);
  const [startHalf, setStartHalf] = useState(false);
  const [endHalf, setEndHalf] = useState(false);
  const [note, setNote] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const [confirmedOverBalance, setConfirmedOverBalance] = useState(false);
  const toast = useToast();

  const blockingVacations = useMemo(
    () => existingVacations.filter(isBlockingVacation),
    [existingVacations],
  );

  const calendarModifiers = useMemo(
    () => ({
      requestedVacation: blockingVacations
        .filter((v) => v.status === "requested")
        .map((v) => ({
          from: parseLocalDate(v.dateStart),
          to: parseLocalDate(v.dateEnd),
        })),
      questionVacation: blockingVacations
        .filter((v) => v.status === "question")
        .map((v) => ({
          from: parseLocalDate(v.dateStart),
          to: parseLocalDate(v.dateEnd),
        })),
      approvedVacation: blockingVacations
        .filter((v) => v.status === "approved" || v.status === "reported")
        .map((v) => ({
          from: parseLocalDate(v.dateStart),
          to: parseLocalDate(v.dateEnd),
        })),
    }),
    [blockingVacations],
  );

  const isSingleDay = useMemo(
    () =>
      Boolean(
        range?.from && range?.to && range.from.getTime() === range.to.getTime(),
      ),
    [range],
  );

  const workingDays = useMemo(() => {
    if (!range?.from || !range.to) return 0;
    return countWorkingDays(range.from, range.to, startHalf, endHalf);
  }, [range, startHalf, endHalf]);

  const exceedsBalance = workingDays > remainingDays;
  const overlap = useMemo(() => {
    if (!range?.from || !range.to) return null;
    return (
      blockingVacations.find((absence) =>
        rangesOverlap(
          range.from!,
          range.to!,
          parseLocalDate(absence.dateStart),
          parseLocalDate(absence.dateEnd),
        ),
      ) ?? null
    );
  }, [blockingVacations, range]);

  const overlapMessage = overlap
    ? `Dieser Zeitraum überschneidet sich mit Urlaub vom ${formatRangeLabel(
        overlap.dateStart,
        overlap.dateEnd,
      )} (${vacationStatusLabel(overlap.status)}).`
    : null;
  const overBalanceDays = Math.max(0, workingDays - remainingDays);

  const handleSubmit = async () => {
    if (!range?.from || !range.to) {
      toast.error("Bitte Zeitraum auswählen.");
      return;
    }
    if (workingDays === 0) {
      toast.error("Der gewählte Zeitraum enthält keine Werktage.");
      return;
    }
    if (overlapMessage) {
      setServerError(null);
      return;
    }
    if (exceedsBalance && !confirmedOverBalance) {
      setConfirmedOverBalance(true);
      setServerError(null);
      return;
    }
    setSubmitting(true);
    try {
      await timeTrackingService.requestVacation({
        date_start: toIsoDate(range.from),
        date_end: toIsoDate(range.to),
        start_half_day: startHalf,
        end_half_day: endHalf,
        note: note.trim() || undefined,
      });
      toast.success("Urlaubsantrag gesendet.");
      dispatchAbsencesRefresh();
      onSubmitted();
      handleReset();
      onClose();
    } catch (err) {
      const message = normalizeVacationRequestError(err);
      logger.error("vacation_request_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setServerError(message);
      toast.error(message);
    } finally {
      setSubmitting(false);
    }
  };

  const handleReset = () => {
    setRange(undefined);
    setStartHalf(false);
    setEndHalf(false);
    setNote("");
    setServerError(null);
    setConfirmedOverBalance(false);
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={() => {
        if (!submitting) {
          handleReset();
          onClose();
        }
      }}
      title="Urlaub beantragen"
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-2xl"
      footer={
        <div className="flex w-full flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="text-xs text-gray-500">
            {range?.from && range?.to ? (
              <>
                <span className="font-medium text-gray-700">
                  {workingDays} {workingDays === 1 ? "Tag" : "Tage"}
                </span>
                {" beantragt · "}
                <span
                  className={exceedsBalance ? "text-red-600" : "text-gray-500"}
                >
                  {remainingDays} Tage verfügbar
                </span>
              </>
            ) : (
              <span>Wähle einen Zeitraum</span>
            )}
          </div>
          <div className="flex flex-col-reverse gap-2 sm:flex-row">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => {
                handleReset();
                onClose();
              }}
              disabled={submitting}
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              variant="primary"
              size="md"
              onClick={handleSubmit}
              disabled={
                submitting ||
                !range?.from ||
                !range.to ||
                workingDays === 0 ||
                Boolean(overlapMessage)
              }
            >
              {submitting
                ? "Wird gesendet…"
                : exceedsBalance && confirmedOverBalance
                  ? "Trotzdem anfragen"
                  : "Antrag senden"}
            </Button>
          </div>
        </div>
      }
    >
      <div className="space-y-5">
        <div>
          <p className="mb-2 text-xs font-semibold tracking-wider text-gray-500 uppercase">
            Zeitraum
          </p>
          <div className="overflow-hidden rounded-xl border border-gray-200 bg-white">
            <RangeCalendarInline
              value={range}
              onChange={(nextRange) => {
                setRange(nextRange);
                setServerError(null);
                setConfirmedOverBalance(false);
              }}
              fromMin={today}
              presets={vacationPresets}
              modifiers={calendarModifiers}
              modifiersClassNames={{
                requestedVacation:
                  "[&>button]:!bg-[#F78C10]/10 [&>button]:!text-[#8A5600] [&>button]:!ring-1 [&>button]:!ring-[#F78C10]/30",
                questionVacation:
                  "[&>button]:!bg-[#7C3AED]/15 [&>button]:!text-[#7C3AED] [&>button]:!ring-1 [&>button]:!ring-[#7C3AED]/40",
                approvedVacation:
                  "[&>button]:!bg-[#83CD2D]/15 [&>button]:!text-[#4a7a15] [&>button]:!ring-1 [&>button]:!ring-[#83CD2D]/40",
              }}
            />
          </div>
          {blockingVacations.length > 0 && (
            <div className="mt-2 flex flex-wrap gap-3 text-[11px] text-gray-500">
              <span className="inline-flex items-center gap-1">
                <span className="h-2 w-2 rounded-full bg-[#F78C10]/50" />
                Beantragt
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="h-2 w-2 rounded-full bg-[#7C3AED]/50" />
                {"Rückfrage"}
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="h-2 w-2 rounded-full bg-[#83CD2D]/50" />
                Genehmigt oder eingetragen
              </span>
            </div>
          )}
        </div>

        {range?.from &&
          range?.to &&
          (isSingleDay ? (
            <div className="flex items-center justify-between rounded-xl border border-gray-200 bg-white px-4 py-3">
              <div>
                <p className="text-sm font-medium text-gray-800">Halber Tag</p>
                <p className="text-xs text-gray-500">
                  Aktivieren, wenn nur ein halber Arbeitstag beansprucht wird.
                </p>
              </div>
              <BooleanField
                value={startHalf || endHalf}
                onChange={(v) => {
                  setStartHalf(v);
                  setEndHalf(v);
                  setConfirmedOverBalance(false);
                }}
              />
            </div>
          ) : (
            <div className="space-y-2">
              <p className="text-xs font-semibold tracking-wider text-gray-500 uppercase">
                Halbe Tage an den Rändern
              </p>
              <div className="flex items-center justify-between rounded-xl border border-gray-200 bg-white px-4 py-3">
                <div>
                  <p className="text-sm font-medium text-gray-800">
                    Erster Tag halbtags
                  </p>
                  <p className="text-xs text-gray-500">
                    Nur Vor- oder Nachmittag am Startdatum.
                  </p>
                </div>
                <BooleanField
                  value={startHalf}
                  onChange={(v) => {
                    setStartHalf(v);
                    setConfirmedOverBalance(false);
                  }}
                />
              </div>
              <div className="flex items-center justify-between rounded-xl border border-gray-200 bg-white px-4 py-3">
                <div>
                  <p className="text-sm font-medium text-gray-800">
                    Letzter Tag halbtags
                  </p>
                  <p className="text-xs text-gray-500">
                    Nur Vor- oder Nachmittag am Enddatum.
                  </p>
                </div>
                <BooleanField
                  value={endHalf}
                  onChange={(v) => {
                    setEndHalf(v);
                    setConfirmedOverBalance(false);
                  }}
                />
              </div>
            </div>
          ))}

        {exceedsBalance && (
          <div className="rounded-xl border border-[#F78C10]/20 bg-[#F78C10]/10 px-4 py-3 text-xs text-[#8A5600]">
            <p className="font-medium">
              Dieser Antrag liegt {overBalanceDays}{" "}
              {overBalanceDays === 1 ? "Tag" : "Tage"} über deinem Resturlaub.
            </p>
            <p className="mt-1">
              {confirmedOverBalance
                ? "Du hast die Warnung bestätigt. Mit Trotzdem anfragen wird der Antrag gesendet."
                : "Die OGS-Leitung kann ihn trotzdem genehmigen. Klicke erst auf Antrag senden, um die Warnung zu bestätigen."}
            </p>
          </div>
        )}

        <div>
          <label
            htmlFor="vacation-note"
            className="mb-2 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
          >
            Notiz (optional)
          </label>
          <Textarea
            id="vacation-note"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            rows={3}
            maxLength={500}
            placeholder="Zum Beispiel: Vertretung mit Kollegin XY abgestimmt"
          />
          <p className="mt-1 text-right text-xs text-gray-400">
            {note.length}/500
          </p>
        </div>

        {(overlapMessage || serverError) && (
          <Alert type="error" message={overlapMessage ?? serverError ?? ""} />
        )}
      </div>
    </Modal>
  );
}
