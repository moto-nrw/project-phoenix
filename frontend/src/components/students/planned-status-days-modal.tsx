"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { addDays } from "date-fns/addDays";
import { differenceInCalendarDays } from "date-fns/differenceInCalendarDays";
import { format } from "date-fns/format";
import { isSameDay } from "date-fns/isSameDay";
import { startOfWeek } from "date-fns/startOfWeek";
import { de } from "date-fns/locale/de";
import { Pencil, Trash2 } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { ConsentCheckbox } from "~/components/ui/consent-checkbox";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { StatusBadge } from "~/components/ui/status-badge";
import { DatePicker, ISODatePicker } from "~/components/ui/date-picker";
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
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import {
  SegmentedControl,
  type SegmentedControlItem,
} from "~/components/ui/segmented-control";
import {
  formatDate as formatCalendarDate,
  parseISODate,
  toISODate,
  berlinTodayISO,
} from "~/lib/date-helpers";
import {
  StudentStatusDayConflictError,
  type StudentStatusDay,
  type StudentStatusKind,
} from "~/lib/student-status-days-api";
import type { CarePlanDay } from "~/lib/student-care-plan-api";
import type { StudentPartialAbsence } from "~/lib/student-partial-absences-api";
import { excusedConcept } from "~/lib/concepts/excused";
import { classTripConcept } from "~/lib/concepts/classTrip";
import { sickConcept } from "~/lib/concepts/sick";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { RemovableChip } from "~/components/ui/removable-chip";

/** Extra choices of a save; only present when the person made one. */
export interface PlannedStatusSubmitOptions {
  /** Days whose partial excusal the person chose to replace (#5698). */
  readonly replacePartialAbsenceDates: readonly string[];
}

interface PlannedStatusDaysModalProps {
  readonly isOpen: boolean;
  readonly status: StudentStatusKind;
  readonly studentName: string;
  readonly isSubmitting: boolean;
  readonly existingDays?: StudentStatusDay[];
  readonly deletingStatusDayId?: string | null;
  readonly existingPartialAbsences?: StudentPartialAbsence[];
  /** Whether the caller may plan a partial excusal ("Ab Uhrzeit"). Those rows
   *  are pickup exceptions: they need users:update plus full care access to
   *  the child, which the absence permission of a school ohne feste Gruppen
   *  deliberately does not grant (#2232). False hides the scope switch, so an
   *  absence-only staffer is not offered a save that would 403. */
  readonly canPlanPartialExcusal?: boolean;
  readonly onClose: () => void;
  readonly loadExistingDays: (
    from: string,
    to: string,
  ) => Promise<StudentStatusDay[]>;
  readonly loadPartialAbsences?: (
    from: string,
    to: string,
  ) => Promise<StudentPartialAbsence[]>;
  /**
   * Saves the days and resolves once they are saved; the panel then closes
   * itself through `onClose`, only while it is still the same session.
   */
  readonly onSubmit: (
    dates: string[],
    reason?: string,
    options?: PlannedStatusSubmitOptions,
  ) => Promise<void>;
  readonly onDeleteStatusDay?: (statusDayId: string) => Promise<void>;
  readonly canDeleteStatusDay?: (day: StudentStatusDay) => boolean;
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
  canPlanPartialExcusal = true,
  onClose,
  loadExistingDays,
  loadPartialAbsences,
  onSubmit,
  onDeleteStatusDay,
  canDeleteStatusDay,
  loadCarePlanDay,
  onSubmitPartialAbsence,
  onDeletePartialAbsence,
}: PlannedStatusDaysModalProps) {
  const [selectionMode, setSelectionMode] =
    useState<SelectionMode>("individual");
  const [selectedDates, setSelectedDates] = useState<Date[]>([]);
  // Eine Sitzung je Öffnen und Statusart. Bewusst `isOpen`: die
  // Lösch-Bestätigungen blenden das Panel aus, ihr Entfernen gehört aber zu
  // derselben Sitzung.
  const save = useFormModalPending(isOpen, status);
  const [rangeStart, setRangeStart] = useState("");
  const [rangeEnd, setRangeEnd] = useState("");
  // Save error of the form (Bauart 2 Regel 5): shown in the body's error
  // slot, not as a toast by the caller.
  const submitErrors = useApiFormError();
  const clearSubmitError = submitErrors.clear;
  const reportInvalid = submitErrors.invalid;
  // „Wiederholen“ sendet den aktuellen Entwurf, nicht den vom Fehler.
  const latestSubmitRef = useRef<() => Promise<void>>(async () => undefined);
  // Failed check of existing entries: shown in place, saving stays blocked.
  const conflictLoad = useApiLoadError();
  const showConflictLoadError = conflictLoad.show;
  const clearConflictLoadError = conflictLoad.clear;
  const conflictCheckFailed = conflictLoad.error !== null;
  // Failed removal of a partial excusal: shown in its confirmation dialog.
  const partialDeleteErrors = useApiFormError();
  // Failed removal of a planned status day: shown in its confirmation dialog.
  const statusDayDeleteErrors = useApiFormError();
  const clearStatusDayDeleteError = statusDayDeleteErrors.clear;
  const clearPartialDeleteError = partialDeleteErrors.clear;
  const [checkedExistingDays, setCheckedExistingDays] = useState<
    StudentStatusDay[]
  >([]);
  // Partial excusals found by the same check. A manual one blocks a full-day
  // status on its day, so those days are named and left out (#5698).
  const [checkedPartialAbsences, setCheckedPartialAbsences] = useState<
    StudentPartialAbsence[]
  >([]);
  // The person chose to replace the partial excusals on the selection with
  // this full-day status (#5698). Nothing is replaced without this choice.
  const [replacePartial, setReplacePartial] = useState(false);
  const [checkedSelectionKey, setCheckedSelectionKey] = useState("");
  const [conflictCheckRevision, setConflictCheckRevision] = useState(0);
  const [isCheckingConflicts, setIsCheckingConflicts] = useState(false);
  const [reason, setReason] = useState("");
  const [excusalScope, setExcusalScope] = useState<ExcusalScope>("full_day");
  const [fromTime, setFromTime] = useState("");
  const [editingPartialAbsenceId, setEditingPartialAbsenceId] = useState<
    string | null
  >(null);
  const [partialAbsencePendingDeletion, setPartialAbsencePendingDeletion] =
    useState<StudentPartialAbsence | null>(null);
  const [statusDayPendingDeletion, setStatusDayPendingDeletion] = useState<{
    readonly id: string;
    readonly date: string;
    readonly status: StudentStatusKind;
  } | null>(null);
  const [carePlanDay, setCarePlanDay] = useState<CarePlanDay | null>(null);
  const [isLoadingCarePlan, setIsLoadingCarePlan] = useState(false);
  const isSick = status === "sick";
  const isClassTrip = status === "class_trip";
  const statusNoun = isSick
    ? "Krankmeldung"
    : isClassTrip
      ? "Klassenfahrt"
      : "Entschuldigung";
  const isPartialExcusal =
    canPlanPartialExcusal &&
    status === "excused" &&
    excusalScope === "from_time";
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
    const monday = startOfWeek(parseISODate(berlinTodayISO()), {
      weekStartsOn: 1,
    });
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
  // Manual partial excusals on the selection, from the check of exactly
  // these days plus the detail page's own window. Automatic ones (from a
  // pulled-forward pickup time) coexist with a full-day status.
  const partialConflicts = useMemo(() => {
    if (isPartialExcusal) return EMPTY_PARTIAL_ABSENCES;
    const byDate = new Map<string, StudentPartialAbsence>();
    const sources = checkedCurrentSelection
      ? [...existingPartialAbsences, ...checkedPartialAbsences]
      : existingPartialAbsences;
    for (const absence of sources) {
      if (!absence.auto && candidateDateKeySet.has(absence.date)) {
        byDate.set(absence.date, absence);
      }
    }
    return Array.from(byDate.values()).sort((a, b) =>
      a.date.localeCompare(b.date),
    );
  }, [
    candidateDateKeySet,
    checkedCurrentSelection,
    checkedPartialAbsences,
    existingPartialAbsences,
    isPartialExcusal,
  ]);
  const partialConflictDates = useMemo(
    () => new Set(partialConflicts.map((absence) => absence.date)),
    [partialConflicts],
  );
  // Replacing needs the right to change partial excusals: the same right the
  // „Entfernen“ in the list below needs.
  const canReplacePartial =
    !isPartialExcusal && Boolean(onDeletePartialAbsence);
  const replacingPartial =
    canReplacePartial && replacePartial && partialConflicts.length > 0;
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
      candidateDateKeys.filter(
        (date) =>
          !candidateExistingDayByDate.has(date) &&
          (replacingPartial || !partialConflictDates.has(date)),
      ),
    [
      candidateDateKeys,
      candidateExistingDayByDate,
      partialConflictDates,
      replacingPartial,
    ],
  );
  const blockedDayCount = candidateDateKeys.length - selectableDateKeys.length;
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
      const today = parseISODate(berlinTodayISO());
      const todayKey = toISODate(today);
      setSelectionMode("individual");
      clearSubmitError();
      setRangeStart(prefillClassTrip && isClassTrip ? todayKey : "");
      setRangeEnd(prefillClassTrip && isClassTrip ? todayKey : "");
      setSelectedDates([]);
      setCheckedExistingDays([]);
      setReplacePartial(false);
      setCheckedSelectionKey("");
      clearConflictLoadError();
      setReason("");
      setExcusalScope("full_day");
      setFromTime("");
      setEditingPartialAbsenceId(null);
      setPartialAbsencePendingDeletion(null);
      clearPartialDeleteError();
      setStatusDayPendingDeletion(null);
      clearStatusDayDeleteError();
      setCarePlanDay(null);
    },
    [
      clearConflictLoadError,
      clearPartialDeleteError,
      clearStatusDayDeleteError,
      clearSubmitError,
      isClassTrip,
    ],
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
      setIsLoadingCarePlan(false);
      return;
    }
    let current = true;
    setIsLoadingCarePlan(true);
    void loadCarePlanDay(date)
      .then((day) => {
        if (current) setCarePlanDay(day);
      })
      .catch(() => {
        // Best-effort only: the timetable care-plan endpoint needs
        // schedules:read. Staff who may edit the student but lack that
        // permission must still save a partial excusal; overlapping-block
        // warnings are skipped when the plan cannot be loaded.
        if (current) setCarePlanDay(null);
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
      setCheckedPartialAbsences([]);
      setCheckedSelectionKey("");
      clearConflictLoadError();
      setIsCheckingConflicts(false);
      return;
    }

    let isCurrent = true;
    setCheckedSelectionKey("");
    clearConflictLoadError();
    setIsCheckingConflicts(true);

    // The partial-excusal read needs users:read. For a full-day status it
    // only names a blocking day early: without it the save still answers
    // with the partial-absence conflict, so a failed read must not block.
    const partialLookup = !loadPartialAbsences
      ? Promise.resolve(EMPTY_PARTIAL_ABSENCES)
      : isPartialExcusal
        ? loadPartialAbsences(firstCandidate, lastCandidate)
        : loadPartialAbsences(firstCandidate, lastCandidate).catch(
            () => EMPTY_PARTIAL_ABSENCES,
          );
    void Promise.all([
      loadExistingDays(firstCandidate, lastCandidate),
      partialLookup,
    ])
      .then(([days, partials]) => {
        if (!isCurrent) return;
        setCheckedExistingDays(days);
        setCheckedPartialAbsences(partials);
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
      .catch((error: unknown) => {
        if (!isCurrent) return;
        setCheckedExistingDays([]);
        setCheckedPartialAbsences([]);
        void showConflictLoadError(error, {
          object: "die Prüfung der vorhandenen Einträge",
          retry: () => setConflictCheckRevision((current) => current + 1),
        });
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
    clearConflictLoadError,
    showConflictLoadError,
  ]);

  const setSortedDates = (dates: Date[]) => {
    const sourceDates = isPartialExcusal ? dates.slice(-1) : dates;
    const unique = new Map<string, Date>();
    let containsExistingDay = false;
    for (const date of sourceDates) {
      const key = toISODate(date);
      const existingDay = activeExistingDayByDate.get(key);
      if (existingDay) {
        containsExistingDay = true;
        reportInvalid(
          `${formatDateLabel(existingDay.date)} ist ${getExistingStatusLabel(
            existingDay.status,
          )}.`,
        );
      } else {
        unique.set(key, date);
      }
    }
    if (!containsExistingDay) {
      clearSubmitError();
    }
    if (isPartialExcusal) {
      // While editing an existing partial excusal the date is fixed: clearing
      // editingPartialAbsenceId on another day would create a new record and
      // leave the original unchanged.
      if (editingPartialAbsenceId) {
        const editing =
          existingPartialAbsences.find(
            (absence) => absence.id === editingPartialAbsenceId,
          ) ?? null;
        if (editing) {
          const editDate = parseISODate(editing.date);
          setSelectedDates([editDate]);
          reportInvalid(
            "Beim Bearbeiten bleibt das Datum fest. Abbrechen, um einen anderen Tag zu wählen.",
          );
          return;
        }
      }
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
    clearPartialDeleteError();
    setCarePlanDay(null);
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
    clearSubmitError();
    setCheckedExistingDays([]);
    setCheckedPartialAbsences([]);
    setCheckedSelectionKey("");
    clearConflictLoadError();
    if (next === "range") {
      setSelectedDates([]);
    } else {
      setRangeStart("");
      setRangeEnd("");
    }
  };

  // `isSubmitting` meldet ein Speichern des Aufrufers; die eigene Sperre
  // des Panels (`save`) deckt nur Läufe ab, die hier starten.
  const handleClose = () => {
    if (!isSubmitting) {
      resetForm(false);
      onClose();
    }
  };

  // Drop the row from the local conflict check only after the caller confirms
  // deletion. Out-of-window deletes never touch the parent SWR range, so a
  // local clear is required on success; a failed delete must keep the row.
  // Entfernen läuft erst nach der Rückfrage (Bauart 2 Regel 6, #3109).
  const handleDeleteStatusDay = async (statusDayId: string) => {
    if (!onDeleteStatusDay) return;
    await save.run(async (op) => {
      statusDayDeleteErrors.clear();
      try {
        await onDeleteStatusDay(statusDayId);
      } catch (error) {
        if (!op.isCurrent()) return;
        // The dialog stays open and says why; no „Wiederholen“, removing runs
        // only through the confirmation.
        await statusDayDeleteErrors.show(error, {
          object: "die geplante Abwesenheit",
        });
        return;
      }
      if (!op.isCurrent()) return;
      setStatusDayPendingDeletion(null);
      setCheckedExistingDays((current) =>
        current.filter((day) => day.id !== statusDayId),
      );
    });
  };

  const handleDeletePartialAbsence = async () => {
    if (!onDeletePartialAbsence || !partialAbsencePendingDeletion) return;
    const partialAbsenceId = partialAbsencePendingDeletion.id;
    await save.run(async (op) => {
      clearPartialDeleteError();
      try {
        await onDeletePartialAbsence(partialAbsenceId);
      } catch (error) {
        if (!op.isCurrent()) return;
        // Kein „Wiederholen“: Entfernen läuft nur über die Rückfrage.
        await partialDeleteErrors.show(error, {
          object: "die Teilentschuldigung",
        });
        return;
      }
      if (!op.isCurrent()) return;
      setPartialAbsencePendingDeletion(null);
      setCheckedPartialAbsences((current) =>
        current.filter((absence) => absence.id !== partialAbsenceId),
      );
      if (editingPartialAbsenceId === partialAbsenceId) {
        setSelectedDates([]);
        setEditingPartialAbsenceId(null);
        setFromTime("");
        setReason("");
      }
    });
  };

  const handleSubmit = async () => {
    if (!checkedCurrentSelection || conflictCheckFailed) return;
    clearSubmitError();
    const dateKeys = selectableDateKeys;
    if (dateKeys.length === 0) {
      submitErrors.invalid(
        usesRangeSelection
          ? "Bitte wählen Sie einen Zeitraum ohne bestehenden Status."
          : "Bitte wählen Sie mindestens einen Tag ohne Krankmeldung oder Entschuldigung.",
      );
      return;
    }
    const trimmedReason = reason.trim();
    if (isPartialExcusal) {
      if (
        dateKeys.length !== 1 ||
        !fromTime ||
        isLoadingCarePlan ||
        !onSubmitPartialAbsence
      ) {
        return;
      }
      const submitPartialAbsence = onSubmitPartialAbsence;
      await save.run(async (op) => {
        try {
          await submitPartialAbsence(
            editingPartialAbsenceId,
            dateKeys[0]!,
            fromTime,
            trimmedReason || undefined,
          );
        } catch (error) {
          // Eine späte Ablehnung gehört nicht in einen neueren Vorgang.
          if (!op.isCurrent()) return;
          // The caller logs the failure; the person reads it here, at the form.
          await submitErrors.show(error, {
            object: "die Entschuldigung",
            retry: () => void latestSubmitRef.current(),
          });
          setConflictCheckRevision((current) => current + 1);
          return;
        }
        // Closing belongs to this session: a late answer must not close a
        // panel opened in between (#6593).
        if (!op.isCurrent()) return;
        resetForm(false);
        onClose();
      });
      return;
    }
    // Pass the reason as a second arg only when present, so callers/tests
    // that expect the single-arg shape keep matching.
    const replaceDates = replacingPartial
      ? partialConflicts
          .map((absence) => absence.date)
          .filter((date) => dateKeys.includes(date))
      : [];
    await save.run(async (op) => {
      try {
        if (replaceDates.length > 0) {
          await onSubmit(dateKeys, trimmedReason || undefined, {
            replacePartialAbsenceDates: replaceDates,
          });
        } else if (trimmedReason) {
          await onSubmit(dateKeys, trimmedReason);
        } else {
          await onSubmit(dateKeys);
        }
      } catch (err) {
        // Eine späte Ablehnung gehört nicht in einen neueren Vorgang.
        if (!op.isCurrent()) return;
        if (err instanceof StudentStatusDayConflictError) {
          // Names the days from the structured conflict list of the answer.
          submitErrors.invalid(getSaveConflictMessage(err.conflicts));
        } else {
          await submitErrors.show(err, {
            object: isSick
              ? "die Krankmeldung"
              : isClassTrip
                ? "die Klassenfahrt"
                : "die Entschuldigung",
            retry: () => void latestSubmitRef.current(),
          });
        }
        // Keep every input intact and refresh conflicts in case the write lost
        // a concurrent race.
        setConflictCheckRevision((current) => current + 1);
        return;
      }
      if (!op.isCurrent()) return;
      resetForm(false);
      onClose();
    });
  };
  useLayoutEffect(() => {
    latestSubmitRef.current = handleSubmit;
  });

  const listedPartialAbsences = isPartialExcusal
    ? existingPartialAbsences
    : partialConflicts;

  const footer = (
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
          conflictCheckFailed ||
          hasInvalidRangeOrder ||
          hasSelectionTooWide ||
          (partialConflicts.length > 0 && selectableDateKeys.length === 0) ||
          (isPartialExcusal &&
            (selectableDateKeys.length !== 1 ||
              !fromTime ||
              isLoadingCarePlan ||
              !onSubmitPartialAbsence))
        }
        size="md"
        className="w-full disabled:cursor-not-allowed sm:w-auto"
      >
        {isSubmitting ? "Speichert…" : submitLabel}
      </Button>
    </>
  );

  return (
    <>
      <SlideOver
        // Ausgeblendet, solange die Lösch-Bestätigung offen ist — nie zwei
        // eigenständige Dialoge übereinander (#2774). Die Bestätigung steht
        // deshalb als Geschwister NEBEN der Schublade: ein Kind der Schublade
        // würde beim Ausblenden mit verschwinden.
        open={
          isOpen &&
          partialAbsencePendingDeletion === null &&
          statusDayPendingDeletion === null
        }
        onOpenChange={(open) => {
          if (!open) handleClose();
        }}
        pending={save}
      >
        <SlideOverContent size="md">
          <SlideOverHeader
            close={<SlideOverCloseButton aria-label="Fenster schließen" />}
          >
            <SlideOverTitle>{title}</SlideOverTitle>
          </SlideOverHeader>
          <SlideOverBody error={submitErrors.error}>
            <div className="space-y-5">
              <div className="flex items-start gap-3">
                <ConceptIconTile
                  concept={
                    isClassTrip
                      ? classTripConcept
                      : isSick
                        ? sickConcept
                        : excusedConcept
                  }
                  size="section"
                  className="mt-0.5"
                />
                <div>
                  <p className="text-sm font-medium text-gray-900">
                    {studentName}
                  </p>
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

              {status === "excused" && canPlanPartialExcusal ? (
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
                  <p className="mb-2 text-sm font-medium text-gray-900">
                    Zeitraum
                  </p>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <ISODatePicker
                      label="Von"
                      id="planned-status-range-start"
                      value={rangeStart}
                      onChange={setRangeStart}
                      calendarLayout="popover"
                    />
                    <ISODatePicker
                      label="Bis"
                      id="planned-status-range-end"
                      value={rangeEnd}
                      min={rangeStart || undefined}
                      onChange={setRangeEnd}
                      calendarLayout="popover"
                      error={
                        hasInvalidRangeOrder
                          ? "Das Bis-Datum darf nicht vor dem Von-Datum liegen."
                          : hasRangeTooLong
                            ? "Ein Zeitraum darf höchstens 366 Tage umfassen."
                            : undefined
                      }
                    />
                  </div>
                  {rangeDateKeys.length > 0 ? (
                    <p className="mt-2 text-sm text-gray-500">
                      {rangeDateKeys.length === 1
                        ? `${formatCalendarDate(rangeStart)} · 1 Tag`
                        : `${formatCalendarDate(rangeStart)} bis ${formatCalendarDate(rangeEnd)} · ${rangeDateKeys.length} Tage`}
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
                        onChange={(date) => {
                          if (editingPartialAbsenceId) return;
                          setSortedDates(date ? [date] : []);
                        }}
                        disabledDay={(date) =>
                          Boolean(editingPartialAbsenceId) ||
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
                        // A manual partial excusal stays selectable here: the
                        // day is named and skipped, or its excusal removed.
                        const partialDay = isPartialExcusal
                          ? undefined
                          : partialAbsenceByDate.get(key);
                        const partialFrom =
                          !existingDay && partialDay && !partialDay.auto
                            ? partialDay.fromTime
                            : null;
                        return (
                          <button
                            key={key}
                            type="button"
                            onClick={() => toggleDate(date)}
                            disabled={existingDay !== undefined}
                            className={`min-h-16 rounded-lg border px-1.5 py-2 text-center text-sm transition-colors focus-visible:outline-2 focus-visible:outline-gray-400 sm:px-2 ${
                              isSelected
                                ? "border-gray-900 bg-gray-900 text-white shadow-sm"
                                : existingDay
                                  ? "border-gray-100 bg-gray-50 text-gray-400 shadow-sm"
                                  : "moto-content-surface text-gray-700"
                            } disabled:cursor-not-allowed`}
                            title={
                              existingDay
                                ? capitalizeFirst(
                                    getExistingStatusLabel(existingDay.status),
                                  )
                                : partialFrom
                                  ? `Ab ${partialFrom} Uhr entschuldigt`
                                  : undefined
                            }
                          >
                            <span className="block font-semibold">
                              {WEEKDAY_LABELS[index]}
                            </span>
                            <span className="text-xs">
                              {format(date, "dd.MM.", { locale: de })}
                            </span>
                            {partialFrom ? (
                              <span className="mt-1 block text-xs leading-tight font-medium">
                                ab {partialFrom}
                              </span>
                            ) : null}
                            {existingDay ? (
                              <span className="mt-1 block text-sm leading-tight font-medium">
                                <span className="block">bereits</span>
                                <span className="block">
                                  {getStatusLabel(
                                    existingDay.status,
                                  ).toLowerCase()}
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
                <Alert
                  type="info"
                  message="Vorhandene Status-Tage werden geprüft…"
                />
              ) : null}
              <LoadErrorAlert error={conflictLoad.error} />
              {hasIndividualSelectionTooWide ? (
                <Alert
                  type="warning"
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
              {partialConflicts.length > 0 ? (
                <div className="space-y-2">
                  <Alert
                    type="warning"
                    message={getPartialConflictMessage(
                      partialConflicts,
                      selectableDateKeys.length,
                      replacingPartial,
                    )}
                  />
                  {canReplacePartial ? (
                    <ConsentCheckbox
                      name="replace-partial-absence"
                      checked={replacePartial}
                      onChange={setReplacePartial}
                      label={`Durch ${statusNoun} ersetzen`}
                      description={`Die ${statusNoun} gilt dann den ganzen Tag. Geänderte Betreuungsblöcke und die Abholzeit gelten wieder wie geplant.`}
                    />
                  ) : null}
                </div>
              ) : null}

              {!isClassTrip && selectedDates.length > 0 && (
                <div>
                  <p className="mb-2 text-sm font-medium text-gray-900">
                    {blockedDayCount > 0
                      ? `${selectableDateKeys.length} von ${candidateDateKeys.length} Tagen werden gespeichert`
                      : selectedDates.length === 1
                        ? "1 Tag ausgewählt"
                        : `${selectedDates.length} Tage ausgewählt`}
                  </p>
                  <div className="flex flex-wrap gap-2">
                    {selectedDates.map((date) => {
                      const key = toISODate(date);
                      const label = format(date, "dd.MM.yyyy", { locale: de });
                      return (
                        <RemovableChip
                          key={key}
                          label={label}
                          onRemove={() =>
                            setSortedDates(
                              selectedDates.filter(
                                (selected) => !isSameDay(selected, date),
                              ),
                            )
                          }
                          removeLabel={`${label} entfernen`}
                        />
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
                        className="moto-content-surface flex items-center justify-between gap-3 rounded-lg border px-3 py-2 text-sm text-gray-700"
                      >
                        <span>
                          <span className="block">
                            {formatDateLabel(day.date)}
                          </span>
                          <span className="block text-xs text-gray-500">
                            {capitalizeFirst(
                              getExistingStatusLabel(day.status),
                            )}
                          </span>
                          {day.note ? (
                            <span className="mt-0.5 block text-xs text-gray-600 italic">
                              {day.note}
                            </span>
                          ) : null}
                        </span>
                        {onDeleteStatusDay &&
                        (canDeleteStatusDay?.(day) ?? true) ? (
                          <OverflowMenu
                            ariaLabel={`Aktionen für ${formatDateLabel(day.date)}`}
                            items={[
                              {
                                label:
                                  deletingStatusDayId === day.id
                                    ? "Wird entfernt…"
                                    : "Entfernen",
                                icon: (
                                  <Trash2 className="h-4 w-4" aria-hidden />
                                ),
                                destructive: true,
                                disabled:
                                  isSubmitting ||
                                  save.pending ||
                                  deletingStatusDayId === day.id,
                                // Entfernen läuft erst nach der Rückfrage
                                // (Bauart 2 Regel 6, #3109).
                                onClick: () =>
                                  setStatusDayPendingDeletion({
                                    id: day.id,
                                    date: day.date,
                                    status: day.status,
                                  }),
                              },
                            ]}
                          />
                        ) : null}
                      </div>
                    ))}
                  </div>
                </div>
              ) : null}

              {listedPartialAbsences.length > 0 ? (
                <div>
                  <p className="mb-2 text-sm font-medium text-gray-900">
                    Bereits ab Uhrzeit entschuldigt
                  </p>
                  <div className="space-y-2">
                    {listedPartialAbsences.map((absence) => (
                      <div
                        key={absence.id}
                        className="moto-content-surface flex flex-col gap-3 rounded-lg border px-3 py-2 text-sm text-gray-700 sm:flex-row sm:items-center sm:justify-between"
                      >
                        <span className="min-w-0 flex-1 break-words">
                          <span className="block">
                            {formatDateLabel(absence.date)}
                            {absence.auto ? (
                              <span className="ml-2 inline-flex align-middle">
                                <StatusBadge
                                  label="Automatisch (Abholzeit)"
                                  tone="orange"
                                  title="Automatisch aus der vorverlegten Abholzeit abgeleitet. Zum Aufheben die Abholzeit des Tages ändern oder entfernen."
                                />
                              </span>
                            ) : null}
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
                        <span className="flex justify-end sm:shrink-0">
                          <OverflowMenu
                            ariaLabel={`Aktionen für Teilentschuldigung vom ${formatDateLabel(absence.date)}`}
                            items={[
                              // Editing switches to „Ab Uhrzeit“, which only
                              // an excusal offers.
                              ...(isPartialExcusal
                                ? [
                                    {
                                      label: "Bearbeiten",
                                      icon: (
                                        <Pencil
                                          className="h-4 w-4"
                                          aria-hidden
                                        />
                                      ),
                                      disabled: isSubmitting || save.pending,
                                      onClick: () =>
                                        handleEditPartialAbsence(absence),
                                    },
                                  ]
                                : []),
                              ...(onDeletePartialAbsence && !absence.auto
                                ? [
                                    ...(isPartialExcusal
                                      ? [{ kind: "separator" } as const]
                                      : []),
                                    {
                                      label: "Entfernen",
                                      icon: (
                                        <Trash2
                                          className="h-4 w-4"
                                          aria-hidden
                                        />
                                      ),
                                      destructive: true,
                                      disabled: isSubmitting || save.pending,
                                      onClick: () => {
                                        clearPartialDeleteError();
                                        setPartialAbsencePendingDeletion(
                                          absence,
                                        );
                                      },
                                    },
                                  ]
                                : []),
                            ]}
                          />
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              ) : null}

              {(isSick || isClassTrip) && (
                <Textarea
                  id="planned-sick-reason"
                  label={isSick ? "Grund (optional)" : "Hinweis (optional)"}
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  rows={2}
                  maxLength={2000}
                  placeholder={
                    isSick
                      ? "z. B. Fieber, beim Arzt"
                      : "z. B. Ziel oder Kontakt vor Ort"
                  }
                />
              )}
            </div>
          </SlideOverBody>
          <SlideOverFooter>{footer}</SlideOverFooter>
        </SlideOverContent>
      </SlideOver>
      <ConfirmDeleteModal
        isOpen={isOpen && statusDayPendingDeletion !== null}
        title="Geplanten Tag entfernen?"
        description={
          statusDayPendingDeletion ? (
            <p>
              Der Eintrag „
              {capitalizeFirst(
                getExistingStatusLabel(statusDayPendingDeletion.status),
              )}
              “ am {formatDateLabel(statusDayPendingDeletion.date)} wird
              entfernt. Das Kind gilt an diesem Tag wieder als erwartet.
            </p>
          ) : null
        }
        gate={{ mode: "twoStep", firstStepLabel: "Entfernen bestätigen" }}
        onConfirm={() => {
          if (statusDayPendingDeletion)
            void handleDeleteStatusDay(statusDayPendingDeletion.id);
        }}
        onClose={() => {
          setStatusDayPendingDeletion(null);
          clearStatusDayDeleteError();
        }}
        loading={
          isSubmitting ||
          save.pending ||
          (deletingStatusDayId !== null &&
            deletingStatusDayId === statusDayPendingDeletion?.id)
        }
        error={statusDayDeleteErrors.error}
        confirmLabel="Eintrag entfernen"
        loadingLabel="Wird entfernt…"
      />
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
          clearPartialDeleteError();
        }}
        loading={isSubmitting || save.pending}
        error={partialDeleteErrors.error}
        confirmLabel="Teilentschuldigung entfernen"
        loadingLabel="Wird entfernt…"
      />
    </>
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

function getPartialConflictMessage(
  conflicts: StudentPartialAbsence[],
  selectableCount: number,
  replacing: boolean,
): string {
  const [first] = conflicts;
  const named =
    conflicts.length === 1 && first
      ? `${formatCalendarDate(first.date)} ist bereits ab ${first.fromTime} Uhr entschuldigt.`
      : `Bereits ab einer Uhrzeit entschuldigt: ${conflicts
          .map(
            (absence) =>
              `${formatCalendarDate(absence.date)} (ab ${absence.fromTime} Uhr)`,
          )
          .join(", ")}.`;
  const saved =
    selectableCount === 0
      ? "Es wird nichts gespeichert."
      : `${selectableCount} ${selectableCount === 1 ? "Tag wird" : "Tage werden"} gespeichert.`;
  if (replacing) {
    return `${named} ${conflicts.length === 1 ? "Dieser Eintrag wird" : "Diese Einträge werden"} ersetzt. ${saved}`;
  }
  const kept = `${conflicts.length === 1 ? "Dieser Tag bleibt" : "Diese Tage bleiben"} unverändert.`;
  return `${named} ${kept} ${saved}`;
}

function getSaveConflictMessage(conflicts: StudentStatusDay[]): string {
  if (conflicts.length === 0) {
    return "Vorhandene Status-Tage wurden nicht überschrieben. Bitte Auswahl prüfen.";
  }
  const details = conflicts
    .map(
      (day) =>
        `${formatCalendarDate(day.date)} (${getStatusLabel(day.status).toLowerCase()})`,
    )
    .join(", ");
  return `${details} ${conflicts.length === 1 ? "wurde" : "wurden"} zwischenzeitlich eingetragen und nicht überschrieben. Bitte Auswahl prüfen.`;
}
