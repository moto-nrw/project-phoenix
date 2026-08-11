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
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { DatePicker, ISODatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
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
import type { CarePlanDay } from "~/lib/student-care-plan-api";
import type { StudentPartialAbsence } from "~/lib/student-partial-absences-api";

interface PlannedStatusDaysModalProps {
  readonly isOpen: boolean;
  readonly status: StudentStatusKind;
  readonly studentName: string;
  readonly isSubmitting: boolean;
  readonly existingDays?: StudentStatusDay[];
  readonly deletingStatusDayId?: string | null;
  readonly existingPartialAbsences?: StudentPartialAbsence[];
  readonly onClose: () => void;
  readonly loadExistingDays: (
    from: string,
    to: string,
  ) => Promise<StudentStatusDay[]>;
  readonly loadPartialAbsences?: (
    from: string,
    to: string,
  ) => Promise<StudentPartialAbsence[]>;
  readonly onSubmit: (dates: string[], reason?: string) => Promise<void>;
  readonly onDeleteStatusDay?: (statusDayId: string) => Promise<void>;
  readonly loadCarePlanDay?: (date: string) => Promise<CarePlanDay>;
  readonly onSubmitPartialAbsence?: (
    partialAbsenceId: string | null,
    date: string,
    fromTime: string,
    reason?: string,
  ) => Promise<void>;
  readonly onDeletePartialAbsence?: (partialAbsenceId: string) => Promise<void>;
}

const WEEKDAY_LABELS = ["Mo", "Di", "Mi", "Do", "Fr"];
const EMPTY_STATUS_DAYS: StudentStatusDay[] = [];
const EMPTY_PARTIAL_ABSENCES: StudentPartialAbsence[] = [];
const MAX_SELECTION_SPAN_DAYS = 366;
type SelectionMode = "individual" | "range";
const SELECTION_MODE_ITEMS: readonly SegmentedControlItem<SelectionMode>[] = [
  { value: "individual", label: "Einzelne Tage" },
  { value: "range", label: "Zeitraum" },
];
type ExcusalScope = "full_day" | "from_time";
const EXCUSAL_SCOPE_ITEMS: readonly SegmentedControlItem<ExcusalScope>[] = [
  { value: "full_day", label: "Ganzer Tag" },
  { value: "from_time", label: "Ab Uhrzeit" },
];

export function PlannedStatusDaysModal({
  isOpen,
  status,
  studentName,
  isSubmitting,
  existingDays = EMPTY_STATUS_DAYS,
  deletingStatusDayId = null,
  existingPartialAbsences = EMPTY_PARTIAL_ABSENCES,
  onClose,
  loadExistingDays,
  loadPartialAbsences,
  onSubmit,
  onDeleteStatusDay,
  loadCarePlanDay,
  onSubmitPartialAbsence,
  onDeletePartialAbsence,
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
  const [excusalScope, setExcusalScope] = useState<ExcusalScope>("full_day");
  const [fromTime, setFromTime] = useState("");
  const [editingPartialAbsenceId, setEditingPartialAbsenceId] = useState<
    string | null
  >(null);
  const [partialAbsencePendingDeletion, setPartialAbsencePendingDeletion] =
    useState<StudentPartialAbsence | null>(null);
  const [partialAbsenceDeleteError, setPartialAbsenceDeleteError] =
    useState("");
  const [carePlanDay, setCarePlanDay] = useState<CarePlanDay | null>(null);
  const [isLoadingCarePlan, setIsLoadingCarePlan] = useState(false);
  const [carePlanError, setCarePlanError] = useState<string | null>(null);
  const isSick = status === "sick";
  const isClassTrip = status === "class_trip";
  const isPartialExcusal = status === "excused" && excusalScope === "from_time";
  const usesRangeSelection =
    !isPartialExcusal && (isClassTrip || selectionMode === "range");
  const title = isSick
    ? "Krankmeldung planen"
    : isClassTrip
      ? "Klassenfahrt planen"
      : "Entschuldigung planen";
  const submitLabel = isPartialExcusal
    ? editingPartialAbsenceId
      ? "Entschuldigung aktualisieren"
      : "Entschuldigen"
    : isSick
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
  const partialAbsenceByDate = useMemo(
    () =>
      new Map(
        existingPartialAbsences.map((absence) => [absence.date, absence]),
      ),
    [existingPartialAbsences],
  );

  // The detail page expands its SWR range when a distant date is selected.
  // Hydrate edit mode when that asynchronous partial-absence response arrives.
  useEffect(() => {
    if (!isOpen || !isPartialExcusal || selectedDates.length !== 1) return;
    const existing = partialAbsenceByDate.get(toISODate(selectedDates[0]!));
    if (!existing || existing.id === editingPartialAbsenceId) return;
    setEditingPartialAbsenceId(existing.id);
    setFromTime(existing.fromTime);
    setReason(existing.reason ?? "");
  }, [
    editingPartialAbsenceId,
    isOpen,
    isPartialExcusal,
    partialAbsenceByDate,
    selectedDates,
  ]);
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
  const candidateDateKeySet = useMemo(
    () => new Set(candidateDateKeys),
    [candidateDateKeys],
  );
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
      // Only re-apply rows that sit on the actual selection. The range lookup
      // may return intervening days for non-contiguous individual picks.
      for (const day of checkedExistingDays) {
        if (!day.cleared_at && candidateDateKeySet.has(day.date)) {
          days.set(day.date, day);
        }
      }
    }
    return days;
  }, [
    activeExistingDayByDate,
    candidateDateKeySet,
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
  // Rows the user can clear: same-status days from the parent list plus
  // selection conflicts (including out-of-window dates). Fetched rows that
  // only fall between non-contiguous picks are excluded.
  const removableExistingDays = useMemo(() => {
    const byId = new Map<string, StudentStatusDay>();
    for (const day of activeExistingDays) {
      byId.set(day.id, day);
    }
    if (checkedCurrentSelection) {
      for (const day of checkedExistingDays) {
        if (
          day.cleared_at ||
          byId.has(day.id) ||
          !candidateDateKeySet.has(day.date)
        ) {
          continue;
        }
        byId.set(day.id, day);
      }
    }
    return Array.from(byId.values()).sort((a, b) =>
      a.date.localeCompare(b.date),
    );
  }, [
    activeExistingDays,
    candidateDateKeySet,
    checkedCurrentSelection,
    checkedExistingDays,
  ]);
  const removableListTitle = useMemo(() => {
    if (
      removableExistingDays.length > 0 &&
      removableExistingDays.every((day) => day.status === status)
    ) {
      return getExistingListTitle(status);
    }
    return "Bereits vorhanden";
  }, [removableExistingDays, status]);
  const selectableDateKeys = useMemo(
    () =>
      candidateDateKeys.filter((date) => !candidateExistingDayByDate.has(date)),
    [candidateDateKeys, candidateExistingDayByDate],
  );
  const hasInvalidRangeOrder =
    rangeStart !== "" && rangeEnd !== "" && rangeEnd < rangeStart;
  const overlappingInstances = useMemo(() => {
    if (!carePlanDay || !fromTime) return [];
    return carePlanDay.instances.filter(
      (instance) =>
        instance.startTime < fromTime && instance.endTime > fromTime,
    );
  }, [carePlanDay, fromTime]);

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
      setExcusalScope("full_day");
      setFromTime("");
      setEditingPartialAbsenceId(null);
      setPartialAbsencePendingDeletion(null);
      setPartialAbsenceDeleteError("");
      setCarePlanDay(null);
      setCarePlanError(null);
    },
    [isClassTrip],
  );

  useEffect(() => {
    if (isOpen) {
      resetForm(true);
    }
  }, [isOpen, resetForm]);

  useEffect(() => {
    const date = isPartialExcusal ? candidateDateKeys[0] : undefined;
    if (!date || candidateDateKeys.length !== 1 || !loadCarePlanDay) {
      setCarePlanDay(null);
      setCarePlanError(null);
      setIsLoadingCarePlan(false);
      return;
    }
    let current = true;
    setIsLoadingCarePlan(true);
    setCarePlanError(null);
    void loadCarePlanDay(date)
      .then((day) => {
        if (current) setCarePlanDay(day);
      })
      .catch(() => {
        if (current) {
          setCarePlanDay(null);
          setCarePlanError(
            "Der Betreuungsplan konnte nicht geprüft werden. Speichern ist derzeit nicht möglich.",
          );
        }
      })
      .finally(() => {
        if (current) setIsLoadingCarePlan(false);
      });
    return () => {
      current = false;
    };
  }, [candidateDateKeys, isPartialExcusal, loadCarePlanDay]);

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

    const partialLookup =
      isPartialExcusal && loadPartialAbsences
        ? loadPartialAbsences(firstCandidate, lastCandidate)
        : Promise.resolve(EMPTY_PARTIAL_ABSENCES);
    void Promise.all([
      loadExistingDays(firstCandidate, lastCandidate),
      partialLookup,
    ])
      .then(([days, partials]) => {
        if (!isCurrent) return;
        setCheckedExistingDays(days);
        if (isPartialExcusal && candidateDateKeys.length === 1) {
          const existing = partials.find(
            (absence) => absence.date === firstCandidate,
          );
          if (existing) {
            setEditingPartialAbsenceId(existing.id);
            setFromTime(existing.fromTime);
            setReason(existing.reason ?? "");
          }
        }
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
    isPartialExcusal,
    loadExistingDays,
    loadPartialAbsences,
    selectionKey,
  ]);

  const setSortedDates = (dates: Date[]) => {
    const sourceDates = isPartialExcusal ? dates.slice(-1) : dates;
    const unique = new Map<string, Date>();
    for (const date of sourceDates) {
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
    if (unique.size === sourceDates.length) {
      setSelectionHint(null);
    }
    if (isPartialExcusal) {
      const selectedDate = Array.from(unique.keys())[0];
      const existing = selectedDate
        ? partialAbsenceByDate.get(selectedDate)
        : undefined;
      setEditingPartialAbsenceId(existing?.id ?? null);
      setFromTime(existing?.fromTime ?? "");
      setReason(existing?.reason ?? "");
    }
    setSelectedDates(
      Array.from(unique.values()).sort((a, b) => a.getTime() - b.getTime()),
    );
  };

  const handleExcusalScopeChange = (next: ExcusalScope) => {
    setExcusalScope(next);
    setSelectionMode("individual");
    setSelectedDates([]);
    setRangeStart("");
    setRangeEnd("");
    setFromTime("");
    setReason("");
    setEditingPartialAbsenceId(null);
    setPartialAbsencePendingDeletion(null);
    setPartialAbsenceDeleteError("");
    setCarePlanDay(null);
    setCarePlanError(null);
  };

  const handleEditPartialAbsence = (absence: StudentPartialAbsence) => {
    setExcusalScope("from_time");
    setSelectionMode("individual");
    setSelectedDates([parseISODate(absence.date)]);
    setEditingPartialAbsenceId(absence.id);
    setFromTime(absence.fromTime);
    setReason(absence.reason ?? "");
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

  // Drop the row from the local conflict check only after the caller confirms
  // deletion. Out-of-window deletes never touch the parent SWR range, so a
  // local clear is required on success; a failed delete must keep the row.
  // The caller owns the user-facing error toast, so failures are swallowed here.
  const handleDeleteStatusDay = async (statusDayId: string) => {
    if (!onDeleteStatusDay) return;
    try {
      await onDeleteStatusDay(statusDayId);
    } catch {
      return;
    }
    setCheckedExistingDays((current) =>
      current.filter((day) => day.id !== statusDayId),
    );
  };

  const handleDeletePartialAbsence = async () => {
    if (!onDeletePartialAbsence || !partialAbsencePendingDeletion) return;
    const partialAbsenceId = partialAbsencePendingDeletion.id;
    setPartialAbsenceDeleteError("");
    try {
      await onDeletePartialAbsence(partialAbsenceId);
    } catch {
      setPartialAbsenceDeleteError(
        "Die Teilentschuldigung konnte nicht entfernt werden. Bitte erneut versuchen.",
      );
      return;
    }
    setPartialAbsencePendingDeletion(null);
    if (editingPartialAbsenceId === partialAbsenceId) {
      setSelectedDates([]);
      setEditingPartialAbsenceId(null);
      setFromTime("");
      setReason("");
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
    if (isPartialExcusal) {
      if (
        dateKeys.length !== 1 ||
        !fromTime ||
        isLoadingCarePlan ||
        carePlanError ||
        !carePlanDay ||
        !onSubmitPartialAbsence
      ) {
        return;
      }
      try {
        await onSubmitPartialAbsence(
          editingPartialAbsenceId,
          dateKeys[0]!,
          fromTime,
          trimmedReason || undefined,
        );
      } catch {
        setConflictCheckRevision((current) => current + 1);
        return;
      }
      resetForm(false);
      return;
    }
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
              hasSelectionTooWide ||
              (isPartialExcusal &&
                (selectableDateKeys.length !== 1 ||
                  !fromTime ||
                  isLoadingCarePlan ||
                  carePlanError !== null ||
                  carePlanDay === null ||
                  !onSubmitPartialAbsence))
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
              {isPartialExcusal
                ? "Wähle genau einen Tag und die Uhrzeit, ab der die Entschuldigung gilt."
                : isClassTrip
                  ? "Wähle den Zeitraum der Klassenfahrt aus."
                  : selectionMode === "range"
                    ? "Wähle den ersten und letzten Tag des Zeitraums aus."
                    : "Wähle einen oder mehrere konkrete Tage aus."}
            </p>
          </div>
        </div>

        {status === "excused" ? (
          <SegmentedControl
            items={EXCUSAL_SCOPE_ITEMS}
            value={excusalScope}
            onChange={handleExcusalScopeChange}
            fullWidth
            ariaLabel="Umfang der Entschuldigung"
          />
        ) : null}

        {!isClassTrip && !isPartialExcusal ? (
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
              {isPartialExcusal ? (
                <DatePicker
                  value={selectedDates[0] ?? null}
                  onChange={(date) => setSortedDates(date ? [date] : [])}
                  disabledDay={(date) =>
                    activeExistingDayByDate.has(toISODate(date))
                  }
                  placeholder="Tag auswählen"
                  calendarLayout="inline"
                  required
                  hideClearButton
                />
              ) : (
                <DatePicker
                  mode="multiple"
                  values={selectedDates}
                  onChangeDates={setSortedDates}
                  disabledDates={disabledDates}
                  placeholder="Tage auswählen"
                  dropdownPlacement="down"
                  calendarLayout="inline"
                />
              )}
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

        {isPartialExcusal ? (
          <div className="space-y-3">
            <Input
              id="planned-excusal-from-time"
              type="time"
              label="Entschuldigt ab"
              value={fromTime}
              onChange={(event) => setFromTime(event.target.value)}
              required
              controlSize="compact"
            />
            <Textarea
              id="planned-partial-excusal-reason"
              label="Grund (optional)"
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              rows={2}
              maxLength={255}
              placeholder="z. B. Arzttermin"
            />
            {isLoadingCarePlan ? (
              <Alert type="info" message="Betreuungsplan wird geprüft…" />
            ) : null}
            {carePlanError ? (
              <Alert type="error" message={carePlanError} />
            ) : null}
            {overlappingInstances.map((instance) => (
              <Alert
                key={instance.id}
                type="warning"
                message={`Der Block ${instance.title} (${instance.startTime}–${instance.endTime}) überschneidet die gewählte Uhrzeit. Er bleibt erwartet und muss bei Bedarf separat entschieden werden.`}
              />
            ))}
          </div>
        ) : null}

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

        {removableExistingDays.length > 0 ? (
          <div>
            <p className="mb-2 text-sm font-medium text-gray-900">
              {removableListTitle}
            </p>
            <div className="space-y-2">
              {removableExistingDays.map((day) => (
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
                      onClick={() => {
                        void handleDeleteStatusDay(day.id);
                      }}
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

        {isPartialExcusal && existingPartialAbsences.length > 0 ? (
          <div>
            <p className="mb-2 text-sm font-medium text-gray-900">
              Bereits ab Uhrzeit entschuldigt
            </p>
            <div className="space-y-2">
              {existingPartialAbsences.map((absence) => (
                <div
                  key={absence.id}
                  className="moto-content-surface flex flex-col gap-3 rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-700 shadow-sm sm:flex-row sm:items-center sm:justify-between"
                >
                  <span className="min-w-0 flex-1 break-words">
                    <span className="block">
                      {formatDateLabel(absence.date)}
                    </span>
                    <span className="block text-xs text-gray-500">
                      Ab {absence.fromTime} Uhr
                      {absence.pickupTime &&
                      absence.pickupTime !== absence.fromTime
                        ? ` · Abholung ${absence.pickupTime} Uhr`
                        : ""}
                    </span>
                    {absence.reason ? (
                      <span className="mt-0.5 block text-xs text-gray-600 italic">
                        {absence.reason}
                      </span>
                    ) : null}
                  </span>
                  <span className="flex flex-wrap gap-2 sm:shrink-0">
                    <Button
                      type="button"
                      size="compact"
                      variant="outline"
                      onClick={() => handleEditPartialAbsence(absence)}
                      disabled={isSubmitting}
                      aria-label={`Teilentschuldigung vom ${formatDateLabel(absence.date)} bearbeiten`}
                    >
                      Bearbeiten
                    </Button>
                    {onDeletePartialAbsence ? (
                      <Button
                        type="button"
                        size="compact"
                        variant="outline_danger"
                        onClick={() => {
                          setPartialAbsenceDeleteError("");
                          setPartialAbsencePendingDeletion(absence);
                        }}
                        disabled={isSubmitting}
                        aria-label={`Teilentschuldigung vom ${formatDateLabel(absence.date)} entfernen`}
                      >
                        Entfernen
                      </Button>
                    ) : null}
                  </span>
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
      <ConfirmDeleteModal
        isOpen={partialAbsencePendingDeletion !== null}
        title="Teilentschuldigung entfernen?"
        description={
          partialAbsencePendingDeletion ? (
            <p>
              Die Teilentschuldigung vom{" "}
              {formatDateLabel(partialAbsencePendingDeletion.date)} wird
              entfernt. Nur die von ihr geänderten Betreuungsblöcke und die
              zugehörige Abholzeit werden wiederhergestellt.
            </p>
          ) : null
        }
        gate={{
          mode: "twoStep",
          firstStepLabel: "Entfernen bestätigen",
        }}
        onConfirm={handleDeletePartialAbsence}
        onClose={() => {
          setPartialAbsencePendingDeletion(null);
          setPartialAbsenceDeleteError("");
        }}
        loading={isSubmitting}
        error={partialAbsenceDeleteError}
        confirmLabel="Teilentschuldigung entfernen"
        loadingLabel="Wird entfernt…"
      />
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
