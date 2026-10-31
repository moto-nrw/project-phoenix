"use client";

import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useLocale, useTranslations } from "next-intl";
import { Ban, Check, Minus, Utensils } from "lucide-react";
import {
  CaretLeftIcon,
  CaretRightIcon,
  PencilSimpleIcon,
} from "@phosphor-icons/react/ssr";

import { Alert } from "~/components/ui/alert";
import { CustomSelect } from "~/components/ui/custom-select";
import { ConfirmationModal } from "~/components/ui/modal";
import { EmptyState } from "~/components/ui/empty-state";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import type { FormErrorInput } from "~/components/ui/form-error";
import { Checkbox } from "~/components/ui/checkbox";
import { StatusBadge } from "~/components/ui/status-badge";
import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";
import { ParentSubsection } from "~/components/parent/shell/parent-section";
import { parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import {
  getChildFeatures,
  getChildMealPlan,
  listMyChildren,
  type MealParticipation,
  type MealPlanEntry,
} from "~/lib/parent-api";
import { useMealParticipation } from "~/lib/hooks/use-meal-participation";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import { usedMealAllergens } from "~/lib/meal-allergens";
import {
  MealAllergenLegend,
  MealAllergenMarks,
} from "~/components/meal-plan/meal-allergens";
import {
  useApiErrorDisplay,
  useApiFormError,
  useApiLoadError,
} from "~/contexts/ToastContext";

const logger = createLogger({ component: "ParentMealPlanPage" });

// One school the parent has a child at, with a representative child used for the
// per-child (school-scoped) backend calls.
interface ChildOption {
  tenantId: string;
  schoolName: string;
  studentId: string;
  childName: string;
  registrationEnabled: boolean;
}

// Parents read the meal plan for this and next week; with the registration on
// they reach four weeks, so deadlines with a lead time stay reachable (#3873).
type WeekOffset = 0 | 1 | 2 | 3;
const LAST_MEAL_PLAN_WEEK: WeekOffset = 1;
const LAST_PARTICIPATION_WEEK: WeekOffset = 3;

function mondayISOFromOffset(todayISO: string, weekOffset: number): string {
  const base = parseISODate(todayISO);
  base.setDate(base.getDate() + weekOffset * 7);
  const offset = (base.getDay() + 6) % 7; // Mon=0 .. Sun=6
  base.setDate(base.getDate() - offset);
  return toISODate(base);
}

function workWeekDates(mondayISO: string): string[] {
  const monday = parseISODate(mondayISO);
  return Array.from({ length: 5 }, (_, i) => {
    const d = new Date(monday);
    d.setDate(monday.getDate() + i);
    return toISODate(d);
  });
}

function isoWeek(iso: string): number {
  const date = parseISODate(iso);
  date.setHours(0, 0, 0, 0);
  date.setDate(date.getDate() + 3 - ((date.getDay() + 6) % 7));
  const weekOne = new Date(date.getFullYear(), 0, 4);
  return (
    1 +
    Math.round(
      ((date.getTime() - weekOne.getTime()) / 86_400_000 -
        3 +
        ((weekOne.getDay() + 6) % 7)) /
        7,
    )
  );
}

// One dish tile as shown in the today card and the desktop grid. On the
// highlighted "today" column the tile is solid white (neutral ring) so it stays
// opaque against the faint tint instead of looking washed out.
function DishCard({
  entry,
  highlight = false,
}: {
  entry: MealPlanEntry;
  highlight?: boolean;
}) {
  return (
    <div
      className={`rounded-lg px-3 py-2.5 ring-1 ${
        highlight
          ? "bg-white shadow-sm ring-gray-200"
          : "bg-gray-50 ring-gray-100"
      }`}
    >
      <p className="text-base leading-snug font-medium text-gray-900">
        {entry.dish}
      </p>
      {entry.note ? (
        <p className="mt-1 text-sm leading-snug text-gray-500">{entry.note}</p>
      ) : null}
      <MealAllergenMarks allergens={entry.allergens} className="mt-1" />
    </div>
  );
}

function MealPlanWeekSkeleton({ loadingLabel }: { loadingLabel: string }) {
  const dishWidths = ["w-4/5", "w-3/5", "w-full", "w-4/5", "w-3/4"];

  return (
    <div aria-busy="true" data-testid="meal-plan-week-skeleton">
      <div role="status" aria-label={loadingLabel} className="sr-only">
        {loadingLabel}
      </div>

      <div className="md:hidden" aria-hidden="true">
        <div className="border-b border-gray-200">
          <div className="flex items-center justify-between border-b border-gray-200 px-4 py-3">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-5 w-12 rounded-full" />
          </div>
          <div className="p-3">
            <div className="rounded-lg bg-gray-50 p-3 ring-1 ring-gray-100">
              <Skeleton className="h-5 w-3/4" />
              <Skeleton className="mt-2 h-4 w-1/2" />
            </div>
          </div>
        </div>
        <div className="divide-y divide-gray-100">
          {dishWidths.slice(1).map((width, index) => (
            <div key={index} className="flex gap-3 px-4 py-3">
              <div className="w-20 shrink-0 space-y-2">
                <Skeleton className="h-4 w-16" />
                <Skeleton className="h-3 w-10" />
              </div>
              <div className="min-w-0 flex-1 space-y-2">
                <Skeleton className={`h-4 ${width}`} />
                <Skeleton className="h-3 w-1/2" />
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="hidden grid-cols-5 md:grid" aria-hidden="true">
        {dishWidths.map((width, index) => (
          <div
            key={index}
            className="border-r border-gray-200 last:border-r-transparent"
          >
            <div className="space-y-2 border-b border-gray-200 px-4 py-3">
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-3 w-10" />
            </div>
            <div className="min-h-48 p-3">
              <div className="rounded-lg bg-gray-50 p-3 ring-1 ring-gray-100">
                <Skeleton className={`h-5 ${width}`} />
                <Skeleton className="mt-2 h-4 w-2/3" />
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function MealPlanLoadingSection({ loadingLabel }: { loadingLabel: string }) {
  return (
    <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
      <div
        className="flex items-center justify-center gap-2 border-b border-gray-200 px-3 py-2.5"
        aria-hidden="true"
      >
        <Skeleton className="size-8 rounded-md" />
        <div className="min-w-40 space-y-2 py-0.5">
          <Skeleton className="mx-auto h-4 w-28" />
          <Skeleton className="mx-auto h-3 w-36" />
        </div>
        <Skeleton className="size-8 rounded-md" />
      </div>
      <MealPlanWeekSkeleton loadingLabel={loadingLabel} />
    </section>
  );
}

function WeekNavigation({
  weekOffset,
  lastWeek,
  mondayISO,
  weekRange,
  weekReady,
  onWeekChange,
  actions,
}: Readonly<{
  weekOffset: WeekOffset;
  lastWeek: WeekOffset;
  mondayISO: string;
  weekRange: string;
  weekReady: boolean;
  onWeekChange: (offset: WeekOffset) => void;
  /** Aktionen für die ganze Woche: rechts neben dem Wochenwechsel, auf dem
      Handy darunter in voller Breite. */
  actions?: ReactNode;
}>) {
  const t = useTranslations("parentMealPlan");
  const weekLabel =
    weekOffset === 0
      ? t("thisWeek")
      : weekOffset === 1
        ? t("nextWeek")
        : t("inWeeks", { count: weekOffset });

  return (
    <div className="grid grid-cols-1 items-center gap-2 border-b border-gray-200 px-3 py-2.5 sm:grid-cols-[1fr_auto_1fr] sm:px-5">
      <nav
        aria-label={t("weekNavigation")}
        className="flex items-center justify-center gap-2 sm:col-start-2"
      >
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="min-h-11 min-w-11"
          aria-label={t("previousWeek")}
          disabled={!weekReady || weekOffset === 0}
          onClick={() => onWeekChange((weekOffset - 1) as WeekOffset)}
        >
          <CaretLeftIcon size={16} weight="bold" aria-hidden="true" />
        </Button>
        <div
          role="status"
          aria-live="polite"
          aria-atomic="true"
          className="min-w-40 text-center"
        >
          <p className="text-sm font-semibold text-gray-900">
            {t("calendarWeek", { week: isoWeek(mondayISO) })}
            <span className="ml-1.5 text-gray-500">· {weekLabel}</span>
          </p>
          <p className="text-xs text-gray-500">{weekRange}</p>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="min-h-11 min-w-11"
          aria-label={t("nextWeek")}
          disabled={!weekReady || weekOffset >= lastWeek}
          onClick={() => onWeekChange((weekOffset + 1) as WeekOffset)}
        >
          <CaretRightIcon size={16} weight="bold" aria-hidden="true" />
        </Button>
      </nav>
      {actions ? (
        <div className="flex w-full flex-col gap-2 sm:col-start-3 sm:w-auto sm:flex-row sm:flex-wrap sm:justify-end sm:justify-self-end [&>*]:w-full sm:[&>*]:w-auto">
          {actions}
        </div>
      ) : null}
    </div>
  );
}

const participationWeekdays = [1, 2, 3, 4, 5] as const;

type DraftDayChange =
  | { readonly mode: "set"; readonly participating: boolean }
  | { readonly mode: "reset" };

function MealParticipationEditActions({
  saving,
  onCancel,
  onSave,
  align = "end",
  className = "",
}: Readonly<{
  saving: boolean;
  onCancel: () => void;
  onSave: () => void;
  align?: "start" | "end";
  className?: string;
}>) {
  const t = useTranslations("parentMealPlan");

  return (
    <div
      className={`flex flex-wrap gap-2 ${align === "start" ? "justify-start" : "justify-end"} ${className}`}
    >
      <Button
        type="button"
        variant="ghost"
        size="md"
        className="min-h-11"
        onClick={onCancel}
        disabled={saving}
      >
        {t("cancel")}
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        className="min-h-11"
        onClick={onSave}
        disabled={saving}
        isLoading={saving}
        loadingText={t("saving")}
      >
        {t("save")}
      </Button>
    </div>
  );
}

function MealParticipationDaysSkeleton({
  loadingLabel,
}: Readonly<{ loadingLabel: string }>) {
  return (
    <div
      aria-busy="true"
      data-testid="meal-participation-week-skeleton"
      className="divide-y divide-gray-100"
    >
      <div role="status" aria-label={loadingLabel} className="sr-only">
        {loadingLabel}
      </div>
      {participationWeekdays.map((weekday) => (
        <div
          key={weekday}
          className="grid gap-3 px-4 py-4 sm:px-5 md:grid-cols-[minmax(9rem,0.7fr)_minmax(0,1.4fr)_minmax(12rem,0.9fr)] md:items-center"
          aria-hidden="true"
        >
          <div className="space-y-2">
            <Skeleton className="h-4 w-28" />
            <Skeleton className="h-3 w-16" />
          </div>
          <div className="space-y-2">
            <Skeleton className="h-4 w-4/5" />
            <Skeleton className="h-3 w-1/2" />
          </div>
          <div className="space-y-2">
            <Skeleton className="h-6 w-36 rounded-full" />
            <Skeleton className="h-8 w-24 rounded-md" />
          </div>
        </div>
      ))}
    </div>
  );
}

function ParticipationWeek({
  schoolId,
  studentId,
  childName,
  today,
  weekDates,
  weekOffset,
  mondayISO,
  weekRange,
  weekReady,
  weekError,
  dishesByDate,
  planLater,
  onWeekChange,
  onEditingChange,
}: Readonly<{
  schoolId: string;
  studentId: string;
  childName: string;
  today: string;
  weekDates: string[];
  weekOffset: WeekOffset;
  mondayISO: string;
  weekRange: string;
  weekReady: boolean;
  weekError: FormErrorInput;
  dishesByDate: ReadonlyMap<string, MealPlanEntry[]>;
  // The week lies beyond the meal plan parents may read.
  planLater: boolean;
  onWeekChange: (offset: WeekOffset) => void;
  onEditingChange: (editing: boolean) => void;
}>) {
  const t = useTranslations("parentMealPlan");
  const locale = useLocale();
  const weekDishes = weekDates.flatMap((date) => dishesByDate.get(date) ?? []);
  const hasDishes = weekDishes.length > 0;
  const weekAllergens = usedMealAllergens(weekDishes);
  const [weekdays, setWeekdays] = useState<number[]>([]);
  const [editingRegularDays, setEditingRegularDays] = useState(false);
  const [editingDate, setEditingDate] = useState<string | null>(null);
  const [dayChange, setDayChange] = useState<DraftDayChange | null>(null);
  const [saving, setSaving] = useState(false);
  const [confirmingWeek, setConfirmingWeek] = useState(false);
  const [weekResult, setWeekResult] = useState<{
    readonly monday: string;
    readonly participating: boolean;
    readonly locked: string[];
  } | null>(null);
  const {
    error: saveError,
    show: showSaveError,
    clear: clearSaveError,
  } = useApiFormError();
  // A failed week cancellation stays in its dialog; a failed week
  // registration has no dialog and reports as a toast.
  const {
    error: weekCancelError,
    show: showWeekCancelError,
    clear: clearWeekCancelError,
  } = useApiFormError();
  const { show: showWeekRegisterError } = useApiErrorDisplay();
  const weekRetryContextRef = useRef({ active: true });
  useLayoutEffect(() => {
    const context = { active: true };
    weekRetryContextRef.current = context;
    // Toasts outlive this child. Never retry an old child or week, even if
    // the family switches away and then back to the same selection.
    return () => {
      context.active = false;
    };
  }, [studentId, mondayISO]);
  // Retry sends the draft as it is now, not as it was when saving failed.
  const retrySaveRef = useRef<() => void>(() => undefined);
  const dayEditorRef = useRef<HTMLFieldSetElement | null>(null);
  const dayEditTriggerRefs = useRef(
    new Map<string, HTMLButtonElement | null>(),
  );
  const restoreFocusDateRef = useRef<string | null>(null);

  const isEditing = editingRegularDays || editingDate !== null;

  useEffect(() => {
    onEditingChange(isEditing);
  }, [isEditing, onEditingChange]);

  useEffect(
    () => () => {
      onEditingChange(false);
    },
    [onEditingChange],
  );

  useEffect(() => {
    if (editingDate) {
      dayEditorRef.current?.focus();
      return;
    }

    const date = restoreFocusDateRef.current;
    if (!date) return;
    dayEditTriggerRefs.current.get(date)?.focus();
    restoreFocusDateRef.current = null;
  }, [editingDate]);

  const range = useMemo(() => {
    const monday = mondayISOFromOffset(today, 0);
    const end = parseISODate(
      mondayISOFromOffset(today, LAST_PARTICIPATION_WEEK),
    );
    end.setDate(end.getDate() + 4);
    return { from: monday, to: toISODate(end) };
  }, [today]);

  const meal = useMealParticipation(
    schoolId,
    studentId,
    range.from,
    range.to,
    today,
  );
  const participation = meal.data;
  const loading = meal.isLoading;
  const loadError = useSwrLoadError(
    meal.error,
    t("errorObjectParticipation"),
    () => meal.mutate(),
  );
  useEffect(() => {
    if (participation && !editingRegularDays)
      setWeekdays(participation.weekdays);
  }, [participation, editingRegularDays]);

  // Ist in dieser Woche kein Tag mehr änderbar (Wochenende, Freitag nach
  // Annahmeschluss), öffnet die Seite einmalig die nächste Woche: Nur dort
  // kann die Familie noch etwas ändern (#3923).
  const weekChosenRef = useRef(false);
  useEffect(() => {
    weekChosenRef.current = false;
  }, [today]);
  useEffect(() => {
    if (!participation || weekChosenRef.current) return;
    weekChosenRef.current = true;
    if (weekOffset !== 0) return;
    const thisWeek = new Set(workWeekDates(mondayISOFromOffset(today, 0)));
    const thisWeekDays = participation.days.filter((day) =>
      thisWeek.has(day.date),
    );
    if (
      thisWeekDays.length > 0 &&
      thisWeekDays.every((day) => !day.changeable)
    ) {
      onWeekChange(1);
    }
  }, [onWeekChange, participation, today, weekOffset]);

  function toggleWeekday(weekday: number) {
    setWeekdays((current) =>
      current.includes(weekday)
        ? current.filter((value) => value !== weekday)
        : [...current, weekday].sort((a, b) => a - b),
    );
  }

  async function saveRegularDays() {
    setSaving(true);
    clearSaveError();
    try {
      const outcome = await meal.saveSchedule(weekdays);
      if (outcome.status === "discarded") {
        setSaving(false);
        return;
      }
      if (outcome.status === "failed") throw outcome.error;
    } catch (err) {
      logger.error("parent_meal_participation_schedule_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showSaveError(err, {
        object: t("errorObjectParticipation"),
        retry: () => retrySaveRef.current(),
      });
      setSaving(false);
      return;
    }
    // The saved state stays on screen while the reader loads again in the
    // background; a failed reload shows as load error above it.
    setEditingRegularDays(false);
    setSaving(false);
  }

  function beginRegularDaysEdit() {
    clearSaveError();
    setEditingRegularDays(true);
  }

  function cancelRegularDaysEdit() {
    clearSaveError();
    setWeekdays(participation?.weekdays ?? []);
    setEditingRegularDays(false);
  }

  function beginDayEdit(date: string) {
    clearSaveError();
    restoreFocusDateRef.current = date;
    setDayChange(null);
    setEditingDate(date);
  }

  function cancelDayEdit() {
    clearSaveError();
    setDayChange(null);
    setEditingDate(null);
  }

  function stageDayChange(
    day: MealParticipation["days"][number],
    participating: boolean,
  ) {
    setDayChange(
      participating === day.participating
        ? null
        : { mode: "set", participating },
    );
  }

  function stageDayReset() {
    setDayChange({ mode: "reset" });
  }

  async function saveDayChange() {
    if (!editingDate || !dayChange) {
      cancelDayEdit();
      return;
    }

    setSaving(true);
    clearSaveError();
    try {
      const outcome = await meal.saveDay(
        editingDate,
        dayChange.mode === "reset" ? null : dayChange.participating,
      );
      if (outcome.status === "discarded") {
        setSaving(false);
        return;
      }
      if (outcome.status === "failed") throw outcome.error;
    } catch (err) {
      logger.error("parent_meal_participation_day_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showSaveError(err, {
        object: t("errorObjectParticipation"),
        retry: () => retrySaveRef.current(),
      });
      setSaving(false);
      return;
    }
    // The saved state stays on screen while the reader loads again in the
    // background; a failed reload shows as load error above it.
    cancelDayEdit();
    setSaving(false);
  }

  // Registers or cancels every day of the week the family may still change;
  // the backend leaves out the days whose deadline has passed.
  async function saveWeek(participating: boolean) {
    const retryContext = weekRetryContextRef.current;
    setSaving(true);
    clearWeekCancelError();
    try {
      const outcome = await meal.saveWeek(weekDates, mondayISO, participating);
      if (outcome.status === "discarded") {
        setSaving(false);
        return;
      }
      if (outcome.status === "failed") throw outcome.error;
      const change = outcome.result;
      setWeekResult({
        monday: mondayISO,
        participating,
        locked: change.locked,
      });
    } catch (err) {
      logger.error("parent_meal_participation_week_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      const retry = () => {
        if (retryContext.active) void saveWeek(participating);
      };
      if (participating) {
        void showWeekRegisterError(err, {
          object: t("errorObjectWeekRegister"),
          retry,
        });
      } else {
        // The dialog stays open with the error; Wiederholen sends again.
        void showWeekCancelError(err, {
          object: t("errorObjectWeekCancel"),
          retry,
        });
      }
      setSaving(false);
      return;
    }
    setConfirmingWeek(false);
    setSaving(false);
  }

  useLayoutEffect(() => {
    retrySaveRef.current = () =>
      void (editingRegularDays ? saveRegularDays() : saveDayChange());
  });

  const regularDaysLabel = useMemo(() => {
    if (!participation || participation.weekdays.length === 0)
      return t("noRegularDays");
    if (participation.weekdays.length === participationWeekdays.length)
      return t("allWeekdays");
    const labels = participation.weekdays.map((weekday) =>
      t(`weekday${weekday}`),
    );
    return new Intl.ListFormat(locale, {
      style: "long",
      type: "conjunction",
    }).format(labels);
  }, [locale, participation, t]);

  const participationByDate = useMemo(
    () => new Map(participation?.days.map((day) => [day.date, day]) ?? []),
    [participation],
  );

  const fullDate = (iso: string) =>
    parseISODate(iso).toLocaleDateString(locale, {
      weekday: "long",
      day: "numeric",
      month: "long",
    });
  // A deadline is an instant; parents read it in Berlin wall-clock time.
  const deadline = (instant: string) =>
    new Date(instant).toLocaleString(locale, {
      weekday: "short",
      day: "2-digit",
      month: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      timeZone: "Europe/Berlin",
    });
  const deadlineLines = (day: MealParticipation["days"][number]) => {
    if (!day.changeable) return [];
    if (
      day.can_register &&
      day.can_cancel &&
      day.register_until === day.cancel_until
    ) {
      return [t("deadlineBoth", { date: deadline(day.cancel_until) })];
    }
    return [
      ...(day.can_register
        ? [t("deadlineRegister", { date: deadline(day.register_until) })]
        : []),
      ...(day.can_cancel
        ? [t("deadlineCancel", { date: deadline(day.cancel_until) })]
        : []),
    ];
  };
  // Whether the parent may still move the day to participating (true) or
  // away from it (false).
  const mayChangeTo = (
    day: MealParticipation["days"][number],
    participating: boolean,
  ) => (participating ? day.can_register : day.can_cancel);
  const weekCancellable =
    !isEditing &&
    weekDates.some((date) => {
      const day = participationByDate.get(date);
      return day?.participating === true && day.can_cancel;
    });
  // A sick or absent day stays registered; registering the week leaves it.
  const weekRegistrable =
    !isEditing &&
    weekDates.some((date) => {
      const day = participationByDate.get(date);
      return (
        day?.participating === false &&
        day.source !== "sick" &&
        day.source !== "absent" &&
        day.source !== "closed" &&
        day.can_register
      );
    });
  const lockedLabel = (dates: string[]) =>
    new Intl.ListFormat(locale, { style: "long", type: "conjunction" }).format(
      dates.map((date) =>
        parseISODate(date).toLocaleDateString(locale, { weekday: "long" }),
      ),
    );

  return (
    <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
      <div className="p-4 sm:p-5">
        <h2 className="text-lg font-semibold text-gray-900">
          {t("participationTitle", { name: childName })}
        </h2>

        {loadError ? (
          <LoadErrorAlert error={loadError} className="mt-4" />
        ) : null}
        {!participation && loadError ? null : loading || !participation ? (
          <div
            className="mt-4 rounded-xl bg-gray-50 p-4"
            aria-label={t("participationLoading")}
          >
            <Skeleton className="h-4 w-24" />
            <Skeleton className="mt-2 h-5 w-52 max-w-full" />
            <Skeleton className="mt-3 h-9 w-24 rounded-md" />
          </div>
        ) : editingRegularDays ? (
          <div className="mt-4 rounded-xl bg-gray-50 p-4">
            <fieldset>
              <legend className="text-sm font-semibold text-gray-900">
                {t("editRegularDaysTitle")}
              </legend>
              <p className="mt-1 text-sm text-gray-600">
                {t("editRegularDaysHint", { name: childName })}
              </p>
              <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-5">
                {participationWeekdays.map((weekday) => (
                  <label
                    key={weekday}
                    className="flex min-h-12 cursor-pointer items-center gap-2 rounded-xl bg-white px-3 py-2 text-sm font-medium text-gray-800 ring-1 ring-gray-200"
                  >
                    <Checkbox
                      checked={weekdays.includes(weekday)}
                      onChange={() => toggleWeekday(weekday)}
                      disabled={saving}
                    />
                    {t(`weekday${weekday}`)}
                  </label>
                ))}
              </div>
              <p className="mt-3 text-xs text-gray-500">{t("finishEdit")}</p>
              <FormErrorAlert message={saveError} className="mt-3" />
              <MealParticipationEditActions
                saving={saving}
                onCancel={cancelRegularDaysEdit}
                onSave={() => void saveRegularDays()}
                className="mt-4"
              />
            </fieldset>
            {!participation.schedule_changeable ? (
              <p className="mt-2 text-sm text-gray-600">
                {t("scheduleLocked")}
              </p>
            ) : null}
          </div>
        ) : (
          <div className="mt-4">
            <ParentSubsection
              title={t("regularDays")}
              actions={
                !editingDate && participation.schedule_changeable ? (
                  <Button
                    type="button"
                    variant="surface"
                    size="md"
                    className="gap-2"
                    aria-expanded={false}
                    onClick={beginRegularDaysEdit}
                  >
                    <PencilSimpleIcon
                      size={20}
                      weight="bold"
                      aria-hidden="true"
                    />
                    {t("edit")}
                  </Button>
                ) : undefined
              }
            >
              <div>
                <p className="text-base font-medium text-gray-800">
                  {regularDaysLabel}
                </p>
                {participation.effective_from ? (
                  <p className="mt-1 text-xs text-gray-500">
                    {t("regularEffective", {
                      date: parseISODate(
                        participation.effective_from,
                      ).toLocaleDateString(locale, {
                        day: "2-digit",
                        month: "2-digit",
                        year: "numeric",
                      }),
                    })}
                  </p>
                ) : null}
                {!participation.schedule_changeable ? (
                  <p className="mt-2 text-sm text-gray-600">
                    {t("scheduleLocked")}
                  </p>
                ) : null}
              </div>
            </ParentSubsection>
          </div>
        )}

        {participation ? (
          <p className="mt-3 text-sm text-gray-600">
            {t("participationDeadlines")}
          </p>
        ) : null}
      </div>

      <div className="border-t border-gray-200">
        <WeekNavigation
          weekOffset={weekOffset}
          lastWeek={LAST_PARTICIPATION_WEEK}
          mondayISO={mondayISO}
          weekRange={weekRange}
          weekReady={weekReady && !isEditing}
          onWeekChange={onWeekChange}
          actions={
            weekReady && !loading && (weekRegistrable || weekCancellable) ? (
              <>
                {weekRegistrable ? (
                  <Button
                    type="button"
                    variant="success"
                    size="md"
                    className="max-sm:min-h-11"
                    disabled={saving}
                    onClick={() => void saveWeek(true)}
                  >
                    {t("registerWeek")}
                  </Button>
                ) : null}
                {weekCancellable ? (
                  <Button
                    type="button"
                    variant="outline_danger"
                    size="md"
                    className="max-sm:min-h-11"
                    disabled={saving}
                    onClick={() => {
                      clearWeekCancelError();
                      setConfirmingWeek(true);
                    }}
                  >
                    {t("cancelWeek")}
                  </Button>
                ) : null}
              </>
            ) : null
          }
        />
      </div>

      {/* Die Wochenaktionen stehen in der Zeile des Wochenwechsels; diese
          Zeile trägt nur noch die Erfolgsmeldung. */}
      {weekReady && !loading && weekResult?.monday === mondayISO ? (
        <div className="border-b border-gray-200 px-4 py-3 sm:px-5">
          <Alert
            type="success"
            message={
              weekResult.participating
                ? weekResult.locked.length > 0
                  ? `${t("registerWeekDone")} ${t("registerWeekLocked", { days: lockedLabel(weekResult.locked) })}`
                  : t("registerWeekDone")
                : weekResult.locked.length > 0
                  ? `${t("cancelWeekDone")} ${t("cancelWeekLocked", { days: lockedLabel(weekResult.locked) })}`
                  : t("cancelWeekDone")
            }
          />
        </div>
      ) : null}

      <ConfirmationModal
        isOpen={confirmingWeek}
        onClose={() => {
          clearWeekCancelError();
          setConfirmingWeek(false);
        }}
        onConfirm={() => void saveWeek(false)}
        title={t("cancelWeekTitle")}
        confirmText={t("cancelWeekConfirm")}
        cancelText={t("cancel")}
        closeLabel={t("cancel")}
        backdropLabel={t("cancel")}
        confirmVariant="danger"
        isConfirmLoading={saving}
        isDismissDisabled={saving}
        loadingText={t("saving")}
        mobileSheet
      >
        <div className="space-y-3">
          <p className="text-sm text-gray-700">
            {t("cancelWeekText", { name: childName })}
          </p>
          <FormErrorAlert message={weekCancelError} />
        </div>
      </ConfirmationModal>

      {!weekReady ? (
        <MealParticipationDaysSkeleton loadingLabel={t("loading")} />
      ) : (
        <>
          {weekError ? (
            <div className="border-b border-gray-200 p-4">
              <LoadErrorAlert error={weekError} />
            </div>
          ) : null}
          <ul className="divide-y divide-gray-100">
            {weekDates.map((date) => {
              const dishes = dishesByDate.get(date) ?? [];
              const day = participationByDate.get(date);
              const draftChange = editingDate === date ? dayChange : null;
              // The server knows holidays and closed days; the weekdays alone
              // would promise a meal the school does not serve.
              const regularParticipation = day
                ? day.regular_participating
                : weekdays.includes(parseISODate(date).getDay());
              const displayedParticipation = draftChange
                ? draftChange.mode === "reset"
                  ? regularParticipation
                  : draftChange.participating
                : day?.participating;
              const displayedSource = draftChange
                ? draftChange.mode === "reset"
                  ? regularParticipation
                    ? "regular"
                    : "none"
                  : "override"
                : day?.source;
              const isToday = date === today;
              const statusLabel = day
                ? displayedSource === "sick"
                  ? t("cancelledBySickNote")
                  : displayedSource === "absent"
                    ? t("cancelledByAbsence")
                    : displayedSource === "closed"
                      ? t("closedDay")
                      : displayedParticipation
                        ? t("registered")
                        : t("notRegistered")
                : t("participationUnavailable");
              const statusTone =
                displayedSource === "sick"
                  ? "red"
                  : displayedParticipation
                    ? "green"
                    : "gray";
              const absence =
                displayedSource === "sick" || displayedSource === "absent";
              // Kein Icon für „Anmeldung nicht geladen“: Das ist ein Ladefehler,
              // kein Teilnahmezustand, und darf nicht wie „nicht angemeldet“ aussehen.
              const statusIcon = day
                ? absence
                  ? Ban
                  : displayedParticipation
                    ? Check
                    : Minus
                : undefined;

              return (
                <li
                  key={date}
                  className={`grid gap-3 px-4 py-4 sm:px-5 md:grid-cols-[minmax(9rem,0.7fr)_minmax(0,1.4fr)_minmax(12rem,0.9fr)] md:items-center ${
                    isToday ? "bg-moto-green/[0.04]" : ""
                  }`}
                >
                  <div className="flex items-start justify-between gap-3 md:block">
                    <div>
                      <p className="text-sm font-semibold text-gray-900">
                        {fullDate(date)}
                      </p>
                      {isToday ? (
                        <p className="text-moto-green-strong mt-1 text-xs font-semibold">
                          {t("today")}
                        </p>
                      ) : null}
                    </div>
                  </div>

                  <div className="order-3 min-w-0 md:order-2">
                    {weekError ? (
                      <p className="text-sm text-gray-500">
                        {t("mealUnavailable")}
                      </p>
                    ) : planLater ? (
                      <p className="text-sm text-gray-500">
                        {t("mealPlanLater")}
                      </p>
                    ) : dishes.length > 0 ? (
                      <div className="space-y-1.5">
                        {dishes.map((entry) => (
                          <div key={entry.position}>
                            <p className="text-sm leading-snug font-medium text-gray-900">
                              {entry.dish}
                            </p>
                            {entry.note ? (
                              <p className="text-xs leading-snug text-gray-500">
                                {entry.note}
                              </p>
                            ) : null}
                            <MealAllergenMarks allergens={entry.allergens} />
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-sm text-gray-500">{t("noMeal")}</p>
                    )}
                  </div>

                  <div className="order-2 md:order-3">
                    {loading ? (
                      <Skeleton className="h-6 w-40 rounded-full" />
                    ) : (
                      <div className="flex flex-wrap items-center gap-2">
                        <StatusBadge
                          label={statusLabel}
                          tone={statusTone}
                          icon={statusIcon}
                        />
                        {day?.changeable &&
                        !absence &&
                        !editingRegularDays &&
                        editingDate === null ? (
                          <Button
                            ref={(node) => {
                              dayEditTriggerRefs.current.set(date, node);
                            }}
                            type="button"
                            variant="ghost"
                            size="md"
                            className="ml-auto min-h-11 shrink-0 px-3 text-sm font-medium"
                            aria-label={t("editDayRegistration", {
                              date: fullDate(date),
                            })}
                            onClick={() => beginDayEdit(date)}
                          >
                            {t("edit")}
                          </Button>
                        ) : null}
                      </div>
                    )}
                    {displayedSource === "override" ? (
                      <p className="mt-1.5 text-xs text-gray-500">
                        {t("singleChange")}
                      </p>
                    ) : null}
                    {day && !day.changeable && day.source !== "closed" ? (
                      <p className="mt-1.5 text-xs text-gray-500">
                        {t("closed")}
                      </p>
                    ) : null}
                    {day && editingDate !== date
                      ? deadlineLines(day).map((line) => (
                          <p
                            key={line}
                            className="mt-1.5 text-xs text-gray-500"
                          >
                            {line}
                          </p>
                        ))
                      : null}
                    {editingDate === date && day?.changeable && !absence ? (
                      <fieldset
                        ref={dayEditorRef}
                        tabIndex={-1}
                        className="mt-3 rounded-xl bg-gray-50 p-3 ring-1 ring-gray-200 focus-visible:ring-2 focus-visible:ring-gray-400"
                      >
                        <legend className="sr-only">
                          {t("editDayRegistration", { date: fullDate(date) })}
                        </legend>
                        <div className="flex flex-wrap gap-2">
                          {mayChangeTo(day, !displayedParticipation) ||
                          draftChange !== null ? (
                            <Button
                              type="button"
                              variant={
                                displayedParticipation
                                  ? "outline_danger"
                                  : "success"
                              }
                              size="compact"
                              className="min-h-11"
                              disabled={saving}
                              onClick={() =>
                                stageDayChange(day, !displayedParticipation)
                              }
                            >
                              {displayedParticipation
                                ? t("unregister")
                                : t("register")}
                            </Button>
                          ) : null}
                          {displayedSource === "override" &&
                          (regularParticipation === day.participating ||
                            mayChangeTo(day, regularParticipation)) ? (
                            <Button
                              type="button"
                              variant="ghost"
                              size="compact"
                              className="min-h-11"
                              disabled={saving}
                              onClick={stageDayReset}
                            >
                              {t("useRegular")}
                            </Button>
                          ) : null}
                        </div>
                        {deadlineLines(day).map((line) => (
                          <p key={line} className="mt-2 text-xs text-gray-500">
                            {line}
                          </p>
                        ))}
                        <p className="mt-2 text-xs text-gray-500">
                          {t("finishEdit")}
                        </p>
                        <FormErrorAlert message={saveError} className="mt-2" />
                        <MealParticipationEditActions
                          saving={saving}
                          onCancel={cancelDayEdit}
                          onSave={() => void saveDayChange()}
                          align="start"
                          className="mt-2"
                        />
                      </fieldset>
                    ) : null}
                  </div>
                </li>
              );
            })}
          </ul>
          {hasDishes ? (
            <MealAllergenLegend
              allergens={weekAllergens}
              className="border-t border-gray-200 px-4 py-3 sm:px-5"
            />
          ) : null}
        </>
      )}

      {participation ? (
        <p className="border-t border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-600 sm:px-5">
          {participation.sick_cancels_after_deadline
            ? t("participationSicknessDay")
            : t("participationSicknessDeadline")}
        </p>
      ) : null}
    </section>
  );
}

export function ParentMealPlanPage() {
  const t = useTranslations("parentMealPlan");
  const locale = useLocale();

  const [schools, setSchools] = useState<ChildOption[]>([]);
  const [hasLinkedChildren, setHasLinkedChildren] = useState(false);
  const [selectedStudent, setSelectedStudent] = useState<string | null>(null);
  const [weekOffset, setWeekOffset] = useState<WeekOffset>(0);
  const [entries, setEntries] = useState<MealPlanEntry[]>([]);
  // The Monday the current `entries` were loaded for. Used to detect when
  // `entries` belongs to a previous week (a slower request still in flight, or
  // a week switch not yet resolved) so we never map stale dishes onto the newly
  // selected week's dates.
  const [loadedMonday, setLoadedMonday] = useState<string | null>(null);
  const [loadingSchools, setLoadingSchools] = useState(true);
  // Distinguish operational failures (backend/network/session errors) from a
  // genuinely empty result. Without these flags a failed request would render
  // as "no meal plan available" / "no plan entered" — false information.
  const {
    error: schoolsError,
    show: showSchoolsError,
    clear: clearSchoolsError,
  } = useApiLoadError();
  const {
    error: weekError,
    show: showWeekError,
    clear: clearWeekError,
  } = useApiLoadError();
  // Bumped by Wiederholen to run the matching load effect again.
  const [schoolsAttempt, setSchoolsAttempt] = useState(0);
  const [weekAttempt, setWeekAttempt] = useState(0);
  const [participationEditing, setParticipationEditing] = useState(false);

  // useBerlinToday re-renders on the Berlin midnight rollover, so a page left
  // mounted overnight follows the date instead of freezing on yesterday.
  const today = useBerlinToday();
  // Anchor the week on the Berlin `today`, not just weekOffset: if the page
  // stays mounted across Berlin midnight, the selected week must roll forward
  // with the date. Keying the memo on `today` also keeps it aligned with the
  // backend's current/next-week guard, which is computed from today's Monday.
  const mondayISO = useMemo(
    () => mondayISOFromOffset(today, weekOffset),
    [today, weekOffset],
  );
  const weekDates = useMemo(() => workWeekDates(mondayISO), [mondayISO]);

  // Resolve which of the parent's schools actually run a meal plan.
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      setLoadingSchools(true);
      clearSchoolsError();
      try {
        const children = await listMyChildren();
        // Let a failed feature lookup reject the whole resolution. A transient
        // 500/session/proxy error must surface as an error, never be treated
        // as meal_plan_enabled: false (which silently removes the school).
        const features = await Promise.all(
          children.map((child) => getChildFeatures(child.student_id)),
        );
        const enabled: ChildOption[] = [];
        children.forEach((child, index) => {
          if (features[index]?.meal_plan_enabled) {
            enabled.push({
              tenantId: child.tenant_id,
              schoolName: child.school_name,
              studentId: child.student_id,
              childName: `${child.first_name} ${child.last_name}`,
              registrationEnabled:
                features[index]?.meal_registration_enabled === true,
            });
          }
        });
        if (cancelled) return;
        setHasLinkedChildren(children.length > 0);
        setSchools(enabled);
        setSelectedStudent(enabled[0]?.studentId ?? null);
      } catch (err) {
        if (cancelled) return;
        logger.error("parent_meal_plan_schools_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        // Awaited so no empty state flashes before the error arrives.
        await showSchoolsError(err, {
          object: t("errorObjectSchools"),
          retry: () => setSchoolsAttempt((attempt) => attempt + 1),
        });
      } finally {
        if (!cancelled) setLoadingSchools(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [clearSchoolsError, schoolsAttempt, showSchoolsError, t]);

  const selectedSchool = useMemo(
    () =>
      schools.find((school) => school.studentId === selectedStudent) ?? null,
    [schools, selectedStudent],
  );

  // Switching schools must drop the previous school's menu immediately, or it
  // would show under the new school's label until the new request returns —
  // wrong information. Clearing loadedMonday makes the week "not ready" so the
  // loading skeleton shows until the new school's week resolves.
  useEffect(() => {
    setEntries([]);
    setLoadedMonday(null);
  }, [selectedStudent]);

  // Load the selected school's week. A slower response from a previous
  // school/week selection must never overwrite the current one, so each run
  // captures a `cancelled` flag the cleanup flips when the inputs change.
  // A school without the registration only shows the meal plan, so a week
  // beyond it cannot stay selected after switching to that school.
  useEffect(() => {
    if (selectedSchool && !selectedSchool.registrationEnabled) {
      setWeekOffset((offset) =>
        offset > LAST_MEAL_PLAN_WEEK ? LAST_MEAL_PLAN_WEEK : offset,
      );
    }
  }, [selectedSchool]);

  // Beyond next week parents only see their registration; the meal plan for
  // those weeks is not public yet, so it is not loaded.
  const planLater = weekOffset > LAST_MEAL_PLAN_WEEK;
  useEffect(() => {
    if (!selectedSchool) return;
    let cancelled = false;
    const { studentId } = selectedSchool;
    clearWeekError();
    if (planLater) {
      setEntries([]);
      setLoadedMonday(mondayISO);
      return;
    }
    void (async () => {
      try {
        const rows = await getChildMealPlan(studentId, mondayISO);
        if (cancelled) return;
        setEntries(rows);
      } catch (err) {
        if (cancelled) return;
        logger.error("parent_meal_plan_week_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        // Operational failure — show the load error, not an empty week
        // (which would falsely tell parents no plan was entered).
        setEntries([]);
        await showWeekError(err, {
          object: t("errorObjectWeek"),
          retry: () => setWeekAttempt((attempt) => attempt + 1),
        });
      } finally {
        if (!cancelled) {
          // Mark which week `entries` now reflects (success or error) so the
          // render can tell a resolved week from stale/in-flight data.
          setLoadedMonday(mondayISO);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [
    clearWeekError,
    mondayISO,
    planLater,
    selectedSchool,
    showWeekError,
    t,
    weekAttempt,
  ]);

  const dishesByDate = useMemo(() => {
    const map = new Map<string, MealPlanEntry[]>();
    for (const e of entries) {
      const list = map.get(e.date) ?? [];
      list.push(e);
      map.set(e.date, list);
    }
    for (const list of map.values())
      list.sort((a, b) => a.position - b.position);
    return map;
  }, [entries]);

  // `entries` only describes the selected week once the load for that exact
  // Monday has resolved. Until then (first load, a week switch, a slow request
  // still returning the previous week) the data is stale: rendering it would
  // map old dishes onto the new dates — a false-empty or wrong week.
  const weekReady = loadedMonday === mondayISO;
  const weekIsEmpty = entries.length === 0;

  const weekdayLabel = (iso: string) =>
    parseISODate(iso).toLocaleDateString(locale, { weekday: "long" });
  const shortDate = (iso: string) =>
    parseISODate(iso).toLocaleDateString(locale, {
      day: "2-digit",
      month: "2-digit",
    });
  const weekRange = t("dateRange", {
    start: parseISODate(weekDates[0]!).toLocaleDateString(locale, {
      day: "2-digit",
      month: "2-digit",
    }),
    end: parseISODate(weekDates[4]!).toLocaleDateString(locale, {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
    }),
  });

  // Today only counts when it falls inside the displayed (work) week.
  const todayInWeek = weekDates.includes(today);

  // The head must not promise more than the body below it delivers. Without the
  // meal registration the page only lists dishes, so the "when" half of the
  // subtitle would be a promise the page never keeps. In the states that show no
  // plan at all (loading, load error, an empty week, no linked child, meal
  // plan switched off) the head carries no subtitle; the body explains those
  // states itself.
  const showsPlan =
    !loadingSchools &&
    !schoolsError &&
    hasLinkedChildren &&
    schools.length > 0 &&
    weekReady &&
    !weekError &&
    !weekIsEmpty;
  const headerDescription = !showsPlan
    ? undefined
    : selectedSchool?.registrationEnabled
      ? t("subtitle")
      : t("subtitleMealsOnly");

  return (
    <ParentPage>
      <ParentPageHeader
        kicker={t("kicker")}
        title={t("title")}
        description={headerDescription}
        actions={
          schools.length > 1 ? (
            <CustomSelect
              value={selectedStudent ?? ""}
              options={schools.map((school) => ({
                value: school.studentId,
                label: `${school.childName} · ${school.schoolName}`,
              }))}
              onChange={setSelectedStudent}
              ariaLabel={t("child")}
              disabled={participationEditing}
              className="w-full sm:w-60"
            />
          ) : undefined
        }
      />

      {loadingSchools ? (
        <MealPlanLoadingSection loadingLabel={t("loading")} />
      ) : schoolsError ? (
        <LoadErrorAlert error={schoolsError} />
      ) : !hasLinkedChildren ? (
        <EmptyState
          icon={<Utensils className="h-10 w-10" />}
          title={t("noLinkedChildrenTitle")}
          description={t("noLinkedChildren")}
          className="moto-content-surface rounded-2xl border px-5 shadow-sm backdrop-blur-md"
        />
      ) : schools.length === 0 ? (
        <EmptyState
          icon={<Utensils className="h-10 w-10" />}
          title={t("emptyTitle")}
          description={t("empty")}
          className="moto-content-surface rounded-2xl border px-5 shadow-sm backdrop-blur-md"
        />
      ) : selectedSchool?.registrationEnabled ? (
        <ParticipationWeek
          key={selectedSchool.studentId}
          schoolId={selectedSchool.tenantId}
          studentId={selectedSchool.studentId}
          childName={selectedSchool.childName}
          today={today}
          weekDates={weekDates}
          weekOffset={weekOffset}
          mondayISO={mondayISO}
          weekRange={weekRange}
          weekReady={weekReady}
          weekError={weekError}
          dishesByDate={dishesByDate}
          planLater={planLater}
          onWeekChange={setWeekOffset}
          onEditingChange={setParticipationEditing}
        />
      ) : (
        <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
          <WeekNavigation
            weekOffset={weekOffset}
            lastWeek={LAST_MEAL_PLAN_WEEK}
            mondayISO={mondayISO}
            weekRange={weekRange}
            weekReady={weekReady}
            onWeekChange={setWeekOffset}
          />

          {!weekReady ? (
            <MealPlanWeekSkeleton loadingLabel={t("loading")} />
          ) : weekError ? (
            <div className="p-4">
              <LoadErrorAlert error={weekError} />
            </div>
          ) : weekIsEmpty ? (
            <EmptyState
              icon={<Utensils className="h-10 w-10" />}
              title={
                weekOffset === 0
                  ? t("emptyThisWeekTitle")
                  : t("emptyNextWeekTitle")
              }
              description={t("emptyWeek")}
              className="px-5"
            />
          ) : (
            <>
              {/* Mobile: today first, then the rest of the week as a list. */}
              <div className="md:hidden">
                {todayInWeek &&
                  (() => {
                    const dishes = dishesByDate.get(today) ?? [];
                    return (
                      <div className="overflow-hidden border-b border-gray-200">
                        <div className="flex items-center justify-between gap-2 border-b border-gray-200 px-4 py-3">
                          <div className="text-sm font-semibold text-gray-900">
                            {weekdayLabel(today)}
                          </div>
                          <span className="bg-moto-green/15 text-moto-green-strong rounded-full px-2 py-0.5 text-xs font-semibold">
                            {t("today")}
                          </span>
                        </div>
                        <div className="flex flex-col gap-2.5 p-3">
                          {dishes.length > 0 ? (
                            dishes.map((entry) => (
                              <DishCard key={entry.position} entry={entry} />
                            ))
                          ) : (
                            <p className="py-3 text-center text-sm text-gray-500">
                              {t("noMeal")}
                            </p>
                          )}
                        </div>
                      </div>
                    );
                  })()}

                <div className="overflow-hidden">
                  <div className="divide-y divide-gray-100">
                    {weekDates
                      .filter((date) => !(todayInWeek && date === today))
                      .map((date) => {
                        const dishes = dishesByDate.get(date) ?? [];
                        return (
                          <div key={date} className="flex gap-3 px-4 py-3">
                            <div className="w-20 shrink-0">
                              <div className="text-sm font-semibold text-gray-900">
                                {weekdayLabel(date)}
                              </div>
                              <div className="text-xs text-gray-500">
                                {shortDate(date)}
                              </div>
                            </div>
                            <div className="min-w-0 flex-1 space-y-1.5">
                              {dishes.length > 0 ? (
                                dishes.map((entry) => (
                                  <div key={entry.position}>
                                    <p className="text-sm leading-snug font-medium text-gray-900">
                                      {entry.dish}
                                    </p>
                                    {entry.note ? (
                                      <p className="text-xs leading-snug text-gray-500">
                                        {entry.note}
                                      </p>
                                    ) : null}
                                    <MealAllergenMarks
                                      allergens={entry.allergens}
                                    />
                                  </div>
                                ))
                              ) : (
                                <p className="text-sm text-gray-500">
                                  {t("noMeal")}
                                </p>
                              )}
                            </div>
                          </div>
                        );
                      })}
                  </div>
                </div>
              </div>

              {/* Desktop: full five-column week grid. */}
              <div className="hidden md:block">
                <div className="grid grid-cols-5">
                  {weekDates.map((date) => {
                    const dishes = dishesByDate.get(date) ?? [];
                    const isToday = date === today;
                    return (
                      <div
                        key={date}
                        className={`flex flex-col border-r border-gray-200 last:border-r-transparent ${
                          isToday ? "bg-moto-green/[0.04]" : ""
                        }`}
                      >
                        <div className="flex items-center justify-between gap-2 border-b border-gray-200 px-4 py-3">
                          <div>
                            <div className="text-sm font-semibold text-gray-900">
                              {weekdayLabel(date)}
                            </div>
                            <div className="text-xs text-gray-500">
                              {shortDate(date)}
                            </div>
                          </div>
                          {isToday && (
                            <span className="bg-moto-green/15 text-moto-green-strong rounded-full px-2 py-0.5 text-xs font-semibold">
                              {t("today")}
                            </span>
                          )}
                        </div>

                        <div className="flex flex-1 flex-col gap-2.5 p-3">
                          {dishes.length > 0 ? (
                            dishes.map((entry) => (
                              <DishCard
                                key={entry.position}
                                entry={entry}
                                highlight={isToday}
                              />
                            ))
                          ) : (
                            <p className="py-6 text-center text-sm text-gray-500">
                              {t("noMeal")}
                            </p>
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
              <MealAllergenLegend
                allergens={usedMealAllergens(entries)}
                className="border-t border-gray-200 px-4 py-3 sm:px-5"
              />
            </>
          )}
        </section>
      )}
    </ParentPage>
  );
}
