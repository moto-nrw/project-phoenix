"use client";

import { useEffect, useMemo, useState } from "react";
import { addDays, format, isSameDay, startOfWeek } from "date-fns";
import { de } from "date-fns/locale";
import { CalendarCheck, CalendarX, X } from "lucide-react";
import { DatePicker, ISODatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
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
  readonly onSubmit: (dates: string[], reason?: string) => Promise<void>;
  readonly onDeleteStatusDay?: (statusDayId: string) => Promise<void>;
}

const WEEKDAY_LABELS = ["Mo", "Di", "Mi", "Do", "Fr"];
const EMPTY_STATUS_DAYS: StudentStatusDay[] = [];

export function PlannedStatusDaysModal({
  isOpen,
  status,
  studentName,
  isSubmitting,
  existingDays = EMPTY_STATUS_DAYS,
  deletingStatusDayId = null,
  onClose,
  onSubmit,
  onDeleteStatusDay,
}: PlannedStatusDaysModalProps) {
  const [selectedDates, setSelectedDates] = useState<Date[]>([]);
  const [rangeStart, setRangeStart] = useState("");
  const [rangeEnd, setRangeEnd] = useState("");
  const [selectionHint, setSelectionHint] = useState<string | null>(null);
  const [reason, setReason] = useState("");
  const isSick = status === "sick";
  const isClassTrip = status === "class_trip";
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
  const disabledDates = useMemo(
    () =>
      Array.from(activeExistingDayByDate.keys()).map(
        (date) => new Date(`${date}T00:00:00`),
      ),
    [activeExistingDayByDate],
  );

  const selectedKeys = useMemo(
    () => new Set(selectedDates.map(formatDateKey)),
    [selectedDates],
  );

  useEffect(() => {
    if (isOpen) {
      const today = new Date();
      const todayKey = formatDateKey(today);
      setSelectionHint(null);
      setRangeStart(todayKey);
      setRangeEnd(todayKey);
      setSelectedDates(activeExistingDayByDate.has(todayKey) ? [] : [today]);
    }
  }, [activeExistingDayByDate, isOpen]);

  const setSortedDates = (dates: Date[]) => {
    const unique = new Map<string, Date>();
    for (const date of dates) {
      const key = formatDateKey(date);
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

  const handleClose = () => {
    if (!isSubmitting) {
      setSelectedDates([]);
      setRangeStart("");
      setRangeEnd("");
      setSelectionHint(null);
      setReason("");
      onClose();
    }
  };

  const handleSubmit = async () => {
    const dateKeys = isClassTrip
      ? buildDateRangeKeys(rangeStart, rangeEnd).filter(
          (date) => !activeExistingDayByDate.has(date),
        )
      : selectedDates
          .map(formatDateKey)
          .filter((date) => !activeExistingDayByDate.has(date));
    if (dateKeys.length === 0) {
      setSelectionHint(
        isClassTrip
          ? "Wähle einen Zeitraum ohne bestehenden Status aus."
          : "Wähle mindestens einen Tag ohne Krankmeldung oder Entschuldigung aus.",
      );
      return;
    }
    const trimmedReason = reason.trim();
    // Pass the reason as a second arg only when present, so callers/tests
    // that expect the single-arg shape keep matching.
    if (trimmedReason) {
      await onSubmit(dateKeys, trimmedReason);
    } else {
      await onSubmit(dateKeys);
    }
    setSelectedDates([]);
    setRangeStart("");
    setRangeEnd("");
    setSelectionHint(null);
    setReason("");
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={handleClose}
      title={title}
      size="md"
      footer={
        <>
          <button
            type="button"
            onClick={handleClose}
            disabled={isSubmitting}
            className="inline-flex h-10 w-full items-center justify-center rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50 sm:w-auto"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={handleSubmit}
            disabled={
              isSubmitting ||
              (isClassTrip
                ? !rangeStart || !rangeEnd || rangeEnd < rangeStart
                : selectedDates.length === 0)
            }
            className="inline-flex h-10 w-full items-center justify-center rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 sm:w-auto"
          >
            {isSubmitting ? "Speichert..." : submitLabel}
          </button>
        </>
      }
    >
      <div className="space-y-5">
        <div className="flex items-start gap-3">
          <div className="moto-content-surface mt-0.5 flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg border border-gray-200 text-gray-600 shadow-sm">
            {isSick ? (
              <CalendarX className="h-5 w-5" />
            ) : (
              <CalendarCheck className="h-5 w-5" />
            )}
          </div>
          <div>
            <p className="text-sm font-medium text-gray-900">{studentName}</p>
            <p className="mt-1 text-sm text-gray-500">
              {isClassTrip
                ? "Wähle den Zeitraum der Klassenfahrt aus."
                : "Heute ist vorausgewählt. Wähle bei Bedarf weitere konkrete Tage aus."}
            </p>
          </div>
        </div>

        {isClassTrip ? (
          <div>
            <p className="mb-2 text-sm font-medium text-gray-900">Zeitraum</p>
            <div className="grid gap-3 sm:grid-cols-2">
              <div>
                <label
                  htmlFor="class-trip-range-start"
                  className="text-sm font-medium text-gray-700"
                >
                  Von
                </label>
                <ISODatePicker
                  id="class-trip-range-start"
                  value={rangeStart}
                  onChange={setRangeStart}
                  calendarLayout="popover"
                  className="mt-1"
                />
              </div>
              <div>
                <label
                  htmlFor="class-trip-range-end"
                  className="text-sm font-medium text-gray-700"
                >
                  Bis
                </label>
                <ISODatePicker
                  id="class-trip-range-end"
                  value={rangeEnd}
                  min={rangeStart || undefined}
                  onChange={setRangeEnd}
                  calendarLayout="popover"
                  className="mt-1"
                />
              </div>
            </div>
            {selectionHint ? (
              <p className="mt-2 rounded-lg border border-[#FF3130]/20 bg-[#FF3130]/10 px-3 py-2 text-sm text-[#CC2626]">
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
                <p className="mt-2 rounded-lg border border-[#FF3130]/20 bg-[#FF3130]/10 px-3 py-2 text-sm text-[#CC2626]">
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
                  const key = formatDateKey(date);
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

        {!isClassTrip && selectedDates.length > 0 && (
          <div>
            <p className="mb-2 text-sm font-medium text-gray-900">
              Ausgewählte Tage
            </p>
            <div className="flex flex-wrap gap-2">
              {selectedDates.map((date) => {
                const key = formatDateKey(date);
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
                      className="inline-flex h-8 items-center justify-center rounded-lg border border-[#FF3130]/20 bg-white px-2.5 text-xs font-semibold text-[#CC2626] shadow-sm transition-colors hover:bg-[#FF3130]/10 focus-visible:ring-2 focus-visible:ring-[#FF3130]/30 focus-visible:outline-none disabled:opacity-50"
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

function formatDateKey(date: Date): string {
  return format(date, "yyyy-MM-dd");
}

function formatDateLabel(date: string): string {
  return format(new Date(`${date}T00:00:00`), "EEEE, dd.MM.yyyy", {
    locale: de,
  });
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
  if (!start || !end || end < start) {
    return [];
  }
  const result: string[] = [];
  const endDate = new Date(`${end}T00:00:00`);
  for (
    let cursor = new Date(`${start}T00:00:00`);
    cursor <= endDate;
    cursor = addDays(cursor, 1)
  ) {
    result.push(formatDateKey(cursor));
  }
  return result;
}
