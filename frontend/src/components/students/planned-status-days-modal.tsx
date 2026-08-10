"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  addDays,
  differenceInCalendarDays,
  format,
  isSameDay,
  startOfWeek,
} from "date-fns";
import { de } from "date-fns/locale";
import { X } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { DatePicker, ISODatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import {
  SegmentedControl,
  type SegmentedControlItem,
} from "~/components/ui/segmented-control";
import {
  formatDate as formatCalendarDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import type {
  StudentStatusDay,
  StudentStatusKind,
} from "~/lib/student-status-days-api";

interface PlannedStatusDaysModalProps {
  readonly isOpen: boolean;
  readonly status: StudentStatusKind;
  readonly studentName: string;
  readonly isSubmitting: boolean;
  readonly existingDays?: StudentStatusDay[];
  readonly deletingStatusDayId?: string | null;
  readonly onClose: () => void;
  readonly loadExistingDays: (
    from: string,
    to: string,
  ) => Promise<StudentStatusDay[]>;
  readonly onSubmit: (dates: string[], reason?: string) => Promise<void>;
  readonly onDeleteStatusDay?: (statusDayId: string) => Promise<void>;
}

const WEEKDAY_LABELS = ["Mo", "Di", "Mi", "Do", "Fr"];
const EMPTY_STATUS_DAYS: StudentStatusDay[] = [];
const MAX_SELECTION_SPAN_DAYS = 366;
type SelectionMode = "individual" | "range";
const SELECTION_MODE_ITEMS: readonly SegmentedControlItem<SelectionMode>[] = [
  { value: "individual", label: "Einzelne Tage" },
  { value: "range", label: "Zeitraum" },
];

export function PlannedStatusDaysModal({
  isOpen,
  status,
  studentName,
  isSubmitting,
  existingDays = EMPTY_STATUS_DAYS,
  deletingStatusDayId = null,
  onClose,
  loadExistingDays,
  onSubmit,
  onDeleteStatusDay,
}: PlannedStatusDaysModalProps) {
  const [selectionMode, setSelectionMode] =
    useState<SelectionMode>("individual");
  const [selectedDates, setSelectedDates] = useState<Date[]>([]);
  const [rangeStart, setRangeStart] = useState("");
  const [rangeEnd, setRangeEnd] = useState("");
  const [selectionHint, setSelectionHint] = useState<string | null>(null);
  const [checkedExistingDays, setCheckedExistingDays] = useState<
    StudentStatusDay[]
  >([]);
  const [checkedSelectionKey, setCheckedSelectionKey] = useState("");
  const [conflictCheckRevision, setConflictCheckRevision] = useState(0);
  const [isCheckingConflicts, setIsCheckingConflicts] = useState(false);
  const [conflictCheckError, setConflictCheckError] = useState<string | null>(
    null,
  );
  const [reason, setReason] = useState("");
  const isSick = status === "sick";
  const isClassTrip = status === "class_trip";
  const usesRangeSelection = isClassTrip || selectionMode === "range";
  const title = isSick
    ? "Krankmeldung planen"
    : isClassTrip
      ? "Klassenfahrt planen"
      : "Entschuldigung planen";
  const submitLabel = isSick
    ? "Krankmelden"
    : isClassTrip
      ? "Klassenfahrt speichern"
      : "Entschuldigen";
  const currentWeekDates = useMemo(() => {
    const monday = startOfWeek(new Date(), { weekStartsOn: 1 });
    return Array.from({ length: 5 }, (_, index) => addDays(monday, index));
  }, []);
  const activeExistingDays = useMemo(
    () =>
      existingDays
        .filter((day) => day.status === status && !day.cleared_at)
        .sort((a, b) => a.date.localeCompare(b.date)),
    [existingDays, status],
  );
  const activeExistingDayByDate = useMemo(() => {
    const days = new Map<string, StudentStatusDay>();
    for (const day of existingDays) {
      if (!day.cleared_at) {
        days.set(day.date, day);
      }
    }
    return days;
  }, [existingDays]);
  // Fingerprint active rows so a delete (or external refresh) re-runs the
  // selection conflict check instead of leaving checkedExistingDays stale.
  const existingDaysFingerprint = useMemo(
    () =>
      existingDays
        .map(
          (day) =>
            `${day.id}:${day.date}:${day.status}:${day.cleared_at ?? ""}`,
        )
        .sort((a, b) => a.localeCompare(b))
        .join("|"),
    [existingDays],
  );
  const disabledDates = useMemo(
    () =>
      Array.from(activeExistingDayByDate.keys()).map((date) =>
        parseISODate(date),
      ),
    [activeExistingDayByDate],
  );

  const selectedKeys = useMemo(
    () => new Set(selectedDates.map(toISODate)),
    [selectedDates],
  );
  const rangeDateKeys = useMemo(
    () => buildDateRangeKeys(rangeStart, rangeEnd),
    [rangeEnd, rangeStart],
  );
  const hasRangeTooLong = isDateSpanTooLong(rangeStart, rangeEnd);
  const candidateDateKeys = useMemo(
    () =>
      usesRangeSelection
        ? rangeDateKeys
        : selectedDates.map(toISODate).sort((a, b) => a.localeCompare(b)),
    [rangeDateKeys, selectedDates, usesRangeSelection],
  );
  const selectionKey = candidateDateKeys.join(",");
  const hasIndividualSelectionTooWide =
    !usesRangeSelection &&
    candidateDateKeys.length > 1 &&
    isDateSpanTooLong(
      candidateDateKeys[0] ?? "",
      candidateDateKeys[candidateDateKeys.length - 1] ?? "",
    );
  const hasSelectionTooWide = hasRangeTooLong || hasIndividualSelectionTooWide;
  const checkedCurrentSelection =
    selectionKey !== "" && checkedSelectionKey === selectionKey;
  const candidateExistingDayByDate = useMemo(() => {
    const days = new Map(activeExistingDayByDate);
    if (checkedCurrentSelection) {
      for (const date of candidateDateKeys) {
        days.delete(date);
      }
      for (const day of checkedExistingDays) {
        if (!day.cleared_at) days.set(day.date, day);
      }
    }
    return days;
  }, [
    activeExistingDayByDate,
    candidateDateKeys,
    checkedCurrentSelection,
    checkedExistingDays,
  ]);
  const conflictingDays = useMemo(
    () =>
      candidateDateKeys
        .map((date) => candidateExistingDayByDate.get(date))
        .filter((day): day is StudentStatusDay => day !== undefined),
    [candidateDateKeys, candidateExistingDayByDate],
  );
  const selectableDateKeys = useMemo(
    () =>
      candidateDateKeys.filter((date) => !candidateExistingDayByDate.has(date)),
    [candidateDateKeys, candidateExistingDayByDate],
  );
  const hasInvalidRangeOrder =
    rangeStart !== "" && rangeEnd !== "" && rangeEnd < rangeStart;

  const resetForm = useCallback(
    (prefillClassTrip: boolean) => {
      const today = new Date();
      const todayKey = toISODate(today);
      setSelectionMode("individual");
      setSelectionHint(null);
      setRangeStart(prefillClassTrip && isClassTrip ? todayKey : "");
      setRangeEnd(prefillClassTrip && isClassTrip ? todayKey : "");
      setSelectedDates([]);
      setCheckedExistingDays([]);
      setCheckedSelectionKey("");
      setConflictCheckError(null);
      setReason("");
    },
    [isClassTrip],
  );

  useEffect(() => {
    if (isOpen) {
      resetForm(true);
    }
  }, [isOpen, resetForm]);

  useEffect(() => {
    const firstCandidate = candidateDateKeys[0];
    const lastCandidate = candidateDateKeys[candidateDateKeys.length - 1];
    if (
      !isOpen ||
      !firstCandidate ||
      !lastCandidate ||
      hasInvalidRangeOrder ||
      hasSelectionTooWide
    ) {
      setCheckedExistingDays([]);
      setCheckedSelectionKey("");
      setConflictCheckError(null);
      setIsCheckingConflicts(false);
      return;
    }

    let isCurrent = true;
    setCheckedSelectionKey("");
    setConflictCheckError(null);
    setIsCheckingConflicts(true);

    void loadExistingDays(firstCandidate, lastCandidate)
      .then((days) => {
        if (!isCurrent) return;
        setCheckedExistingDays(days);
        setCheckedSelectionKey(selectionKey);
      })
      .catch(() => {
        if (!isCurrent) return;
        setCheckedExistingDays([]);
        setConflictCheckError(
          "Vorhandene Status-Tage konnten nicht geprüft werden. Speichern ist derzeit nicht möglich.",
        );
      })
      .finally(() => {
        if (isCurrent) setIsCheckingConflicts(false);
      });

    return () => {
      isCurrent = false;
    };
  }, [
    candidateDateKeys,
    conflictCheckRevision,
    existingDaysFingerprint,
    hasInvalidRangeOrder,
    hasSelectionTooWide,
    isOpen,
    loadExistingDays,
    selectionKey,
  ]);

  const setSortedDates = (dates: Date[]) => {
    const unique = new Map<string, Date>();
    for (const date of dates) {
      const key = toISODate(date);
      const existingDay = activeExistingDayByDate.get(key);
      if (existingDay) {
        setSelectionHint(
          `${formatDateLabel(existingDay.date)} ist ${getExistingStatusLabel(
            existingDay.status,
          )}.`,
        );
      } else {
        unique.set(key, date);
      }
    }
    if (unique.size === dates.length) {
      setSelectionHint(null);
    }
    setSelectedDates(
      Array.from(unique.values()).sort((a, b) => a.getTime() - b.getTime()),
    );
  };

  const toggleDate = (date: Date) => {
    const exists = selectedDates.some((selected) => isSameDay(selected, date));
    setSortedDates(
      exists
        ? selectedDates.filter((selected) => !isSameDay(selected, date))
        : [...selectedDates, date],
    );
  };

  const handleSelectionModeChange = (next: SelectionMode) => {
    setSelectionMode(next);
    setSelectionHint(null);
    setCheckedExistingDays([]);
    setCheckedSelectionKey("");
    setConflictCheckError(null);
    if (next === "range") {
      setSelectedDates([]);
    } else {
      setRangeStart("");
      setRangeEnd("");
    }
  };

  const handleClose = () => {
    if (!isSubmitting) {
      resetForm(false);
      onClose();
    }
  };

  const handleSubmit = async () => {
    if (!checkedCurrentSelection || conflictCheckError) return;
    const dateKeys = selectableDateKeys;
    if (dateKeys.length === 0) {
      setSelectionHint(
        usesRangeSelection
          ? "Wähle einen Zeitraum ohne bestehenden Status aus."
          : "Wähle mindestens einen Tag ohne Krankmeldung oder Entschuldigung aus.",
      );
      return;
    }
    const trimmedReason = reason.trim();
    // Pass the reason as a second arg only when present, so callers/tests
    // that expect the single-arg shape keep matching.
    try {
      if (trimmedReason) {
        await onSubmit(dateKeys, trimmedReason);
      } else {
        await onSubmit(dateKeys);
      }
    } catch {
      // The caller owns the user-facing error toast. Keep every input intact
      // and refresh conflicts in case the write lost a concurrent race.
      setConflictCheckRevision((current) => current + 1);
      return;
    }
    resetForm(false);
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={handleClose}
      title={title}
      size="md"
      footer={
        <>
          <Button
            type="button"
            onClick={handleClose}
            disabled={isSubmitting}
            variant="outline"
            size="md"
            className="w-full sm:w-auto"
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            onClick={handleSubmit}
            disabled={
              isSubmitting ||
              isCheckingConflicts ||
              !checkedCurrentSelection ||
              conflictCheckError !== null ||
              selectableDateKeys.length === 0 ||
              hasInvalidRangeOrder ||
              hasSelectionTooWide
            }
            size="md"
            className="w-full disabled:cursor-not-allowed sm:w-auto"
          >
            {isSubmitting ? "Speichert…" : submitLabel}
          </Button>
        </>
      }
    >
      <div className="space-y-5">
        <div className="flex items-start gap-3">
          <div className="mt-0.5 flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-gray-100">
            <MotoConceptIcon
              concept={isClassTrip ? "classTrip" : isSick ? "sick" : "excused"}
              size={22}
            />
          </div>
          <div>
            <p className="text-sm font-medium text-gray-900">{studentName}</p>
            <p className="mt-1 text-sm text-gray-500">
              {isClassTrip
                ? "Wähle den Zeitraum der Klassenfahrt aus."
                : selectionMode === "range"
                  ? "Wähle den ersten und letzten Tag des Zeitraums aus."
                  : "Wähle einen oder mehrere konkrete Tage aus."}
            </p>
          </div>
        </div>

        {!isClassTrip ? (
          <SegmentedControl
            items={SELECTION_MODE_ITEMS}
            value={selectionMode}
            onChange={handleSelectionModeChange}
            fullWidth
            ariaLabel="Art der Datumsauswahl"
          />
        ) : null}

        {usesRangeSelection ? (
          <div>
            <p className="mb-2 text-sm font-medium text-gray-900">Zeitraum</p>
            <div className="grid gap-3 sm:grid-cols-2">
              <div>
                <label
                  htmlFor="planned-status-range-start"
                  className="text-sm font-medium text-gray-700"
                >
                  Von
                </label>
                <ISODatePicker
                  id="planned-status-range-start"
                  value={rangeStart}
                  onChange={setRangeStart}
                  calendarLayout="popover"
                  className="mt-1"
                />
              </div>
              <div>
                <label
                  htmlFor="planned-status-range-end"
                  className="text-sm font-medium text-gray-700"
                >
                  Bis
                </label>
                <ISODatePicker
                  id="planned-status-range-end"
                  value={rangeEnd}
                  min={rangeStart || undefined}
                  onChange={setRangeEnd}
                  calendarLayout="popover"
                  className="mt-1"
                  error={
                    hasInvalidRangeOrder
                      ? "Das Bis-Datum darf nicht vor dem Von-Datum liegen."
                      : hasRangeTooLong
                        ? "Ein Zeitraum darf höchstens 366 Tage umfassen."
                        : undefined
                  }
                />
              </div>
            </div>
            {rangeDateKeys.length > 0 ? (
              <p className="mt-2 text-sm text-gray-500">
                {rangeDateKeys.length === 1
                  ? `${formatCalendarDate(rangeStart)} · 1 Tag`
                  : `${formatCalendarDate(rangeStart)} bis ${formatCalendarDate(rangeEnd)} · ${rangeDateKeys.length} Tage`}
              </p>
            ) : null}
            {selectionHint ? (
              <p className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong mt-2 rounded-lg border px-3 py-2 text-sm">
                {selectionHint}
              </p>
            ) : null}
          </div>
        ) : (
          <>
            <div>
              <p className="mb-2 text-sm font-medium text-gray-900">
                Kalenderauswahl
              </p>
              <DatePicker
                mode="multiple"
                values={selectedDates}
                onChangeDates={setSortedDates}
                disabledDates={disabledDates}
                placeholder="Tage auswählen"
                dropdownPlacement="down"
                calendarLayout="inline"
              />
              {selectionHint ? (
                <p className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong mt-2 rounded-lg border px-3 py-2 text-sm">
                  {selectionHint}
                </p>
              ) : null}
            </div>

            <div>
              <p className="mb-2 text-sm font-medium text-gray-900">
                Aktuelle Woche
              </p>
              <div className="grid grid-cols-5 gap-1.5 sm:gap-2">
                {currentWeekDates.map((date, index) => {
                  const key = toISODate(date);
                  const isSelected = selectedKeys.has(key);
                  const existingDay = activeExistingDayByDate.get(key);
                  return (
                    <button
                      key={key}
                      type="button"
                      onClick={() => toggleDate(date)}
                      disabled={existingDay !== undefined}
                      className={`min-h-16 rounded-lg border px-1.5 py-2 text-center text-sm shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:px-2 ${
                        isSelected
                          ? "border-gray-900 bg-gray-900 text-white"
                          : existingDay
                            ? "border-gray-100 bg-gray-50 text-gray-400"
                            : "moto-content-surface border-gray-200 text-gray-700 hover:border-gray-300 hover:bg-gray-50"
                      } disabled:cursor-not-allowed`}
                      title={
                        existingDay
                          ? capitalizeFirst(
                              getExistingStatusLabel(existingDay.status),
                            )
                          : undefined
                      }
                    >
                      <span className="block font-semibold">
                        {WEEKDAY_LABELS[index]}
                      </span>
                      <span className="text-xs">
                        {format(date, "dd.MM.", { locale: de })}
                      </span>
                      {existingDay ? (
                        <span className="mt-1 block text-[10px] leading-tight font-medium sm:text-[11px]">
                          <span className="block">bereits</span>
                          <span className="block">
                            {getStatusLabel(existingDay.status).toLowerCase()}
                          </span>
                        </span>
                      ) : null}
                    </button>
                  );
                })}
              </div>
            </div>
          </>
        )}

        {isCheckingConflicts && candidateDateKeys.length > 0 ? (
          <Alert type="info" message="Vorhandene Status-Tage werden geprüft…" />
        ) : null}
        {conflictCheckError ? (
          <Alert type="error" message={conflictCheckError} />
        ) : null}
        {hasIndividualSelectionTooWide ? (
          <Alert
            type="error"
            message="Die ausgewählten Einzeltage dürfen höchstens 366 Tage auseinanderliegen."
          />
        ) : null}
        {conflictingDays.length > 0 ? (
          <Alert
            type="warning"
            message={getConflictMessage(
              conflictingDays,
              candidateDateKeys.length,
              selectableDateKeys.length,
            )}
          />
        ) : null}

        {!isClassTrip && selectedDates.length > 0 && (
          <div>
            <p className="mb-2 text-sm font-medium text-gray-900">
              {conflictingDays.length > 0
                ? `${selectableDateKeys.length} von ${candidateDateKeys.length} Tagen werden gespeichert`
                : selectedDates.length === 1
                  ? "1 Tag ausgewählt"
                  : `${selectedDates.length} Tage ausgewählt`}
            </p>
            <div className="flex flex-wrap gap-2">
              {selectedDates.map((date) => {
                const key = toISODate(date);
                return (
                  <button
                    key={key}
                    type="button"
                    onClick={() =>
                      setSortedDates(
                        selectedDates.filter(
                          (selected) => !isSameDay(selected, date),
                        ),
                      )
                    }
                    className="moto-content-surface inline-flex items-center gap-1 rounded-lg border border-gray-200 px-3 py-1.5 text-sm text-gray-700 shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                  >
                    {format(date, "dd.MM.yyyy", { locale: de })}
                    <X className="h-3.5 w-3.5 text-gray-400" />
                  </button>
                );
              })}
            </div>
          </div>
        )}

        {activeExistingDays.length > 0 ? (
          <div>
            <p className="mb-2 text-sm font-medium text-gray-900">
              {getExistingListTitle(status)}
            </p>
            <div className="space-y-2">
              {activeExistingDays.map((day) => (
                <div
                  key={day.id}
                  className="moto-content-surface flex items-center justify-between gap-3 rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-700 shadow-sm"
                >
                  <span>
                    <span className="block">{formatDateLabel(day.date)}</span>
                    <span className="block text-xs text-gray-500">
                      {capitalizeFirst(getExistingStatusLabel(day.status))}
                    </span>
                    {day.note ? (
                      <span className="mt-0.5 block text-xs text-gray-600 italic">
                        {day.note}
                      </span>
                    ) : null}
                  </span>
                  {onDeleteStatusDay ? (
                    <button
                      type="button"
                      onClick={() => onDeleteStatusDay(day.id)}
                      disabled={isSubmitting || deletingStatusDayId === day.id}
                      className="border-moto-red/20 text-moto-red-strong hover:bg-moto-red/10 focus-visible:ring-moto-red/30 inline-flex h-8 items-center justify-center rounded-lg border bg-white px-2.5 text-xs font-semibold shadow-sm transition-colors focus-visible:ring-2 focus-visible:outline-none disabled:opacity-50"
                    >
                      {deletingStatusDayId === day.id
                        ? "Wird entfernt..."
                        : "Entfernen"}
                    </button>
                  ) : null}
                </div>
              ))}
            </div>
          </div>
        ) : null}

        {(isSick || isClassTrip) && (
          <div>
            <label
              htmlFor="planned-sick-reason"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              {isSick ? "Grund (optional)" : "Hinweis (optional)"}
            </label>
            <textarea
              id="planned-sick-reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              rows={2}
              maxLength={2000}
              placeholder={
                isSick
                  ? "z. B. Fieber, beim Arzt"
                  : "z. B. Ziel oder Kontakt vor Ort"
              }
              className="w-full resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-gray-500 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none"
            />
          </div>
        )}
      </div>
    </FormModal>
  );
}

function formatDateLabel(date: string): string {
  return formatCalendarDate(date, true);
}

function getStatusLabel(status: StudentStatusKind): string {
  if (status === "sick") return "Krank";
  if (status === "class_trip") return "Klassenfahrt";
  return "Entschuldigt";
}

function getExistingStatusLabel(status: StudentStatusKind): string {
  if (status === "sick") return "bereits krank";
  if (status === "class_trip") return "bereits Klassenfahrt";
  return "bereits entschuldigt";
}

function capitalizeFirst(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function getExistingListTitle(status: StudentStatusKind): string {
  if (status === "sick") return "Bereits krank";
  if (status === "class_trip") return "Bereits Klassenfahrt";
  return "Bereits entschuldigt";
}

function buildDateRangeKeys(start: string, end: string): string[] {
  if (!start || !end || end < start || isDateSpanTooLong(start, end)) {
    return [];
  }
  const result: string[] = [];
  const endDate = parseISODate(end);
  for (
    let cursor = parseISODate(start);
    cursor <= endDate;
    cursor = addDays(cursor, 1)
  ) {
    result.push(toISODate(cursor));
  }
  return result;
}

function isDateSpanTooLong(start: string, end: string): boolean {
  if (!start || !end || end < start) return false;
  return (
    differenceInCalendarDays(parseISODate(end), parseISODate(start)) >=
    MAX_SELECTION_SPAN_DAYS
  );
}

function getConflictMessage(
  conflicts: StudentStatusDay[],
  totalCount: number,
  selectableCount: number,
): string {
  const conflictCount = conflicts.length;
  const details = conflicts
    .map(
      (day) =>
        `${formatCalendarDate(day.date)} (${getStatusLabel(day.status).toLowerCase()})`,
    )
    .join(", ");
  if (conflictCount === totalCount) {
    return `${details}: ${totalCount === 1 ? "Dieser Tag hat" : "Diese Tage haben"} bereits einen Status und ${totalCount === 1 ? "wird" : "werden"} nicht überschrieben. Es wird nichts gespeichert.`;
  }

  return `${details}: ${conflictCount} von ${totalCount} Tagen ${conflictCount === 1 ? "hat" : "haben"} bereits einen Status und ${conflictCount === 1 ? "wird" : "werden"} nicht überschrieben. ${selectableCount} ${selectableCount === 1 ? "Tag wird" : "Tage werden"} gespeichert.`;
}
