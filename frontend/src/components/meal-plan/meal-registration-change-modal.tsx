"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";

import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { FormModal } from "~/components/ui/form-modal";
import { SegmentedControl } from "~/components/ui/segmented-control";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useLocalizedDatePicker } from "~/lib/hooks/use-localized-date-picker";
import { createLogger } from "~/lib/logger";
import {
  clearChildMealDay,
  getChildMealSchedule,
  replaceChildMealSchedule,
  setChildMealDay,
  type DailyMealParticipant,
} from "~/lib/meal-plan-api";

const logger = createLogger({ component: "MealRegistrationChangeModal" });

const WEEKDAYS = [1, 2, 3, 4, 5] as const;

type ChangeScope = "day" | "regular";
// "regular" gibt den Tag an die festen Tage zurück.
type DayChoice = "eats" | "skips" | "regular";
type ScheduleState = "idle" | "loading" | "loaded" | "failed";

/**
 * Die Schule ändert die Anmeldung eines Kindes, auch nach der Frist (#3873):
 * einen Tag, den Tag zurück auf die festen Tage, oder die festen Tage ab
 * einem Datum. Eltern sehen die Änderung im Elternportal.
 */
export function MealRegistrationChangeModal({
  isOpen,
  onClose,
  onSaved,
  date,
  participants,
  others,
}: Readonly<{
  isOpen: boolean;
  onClose: () => void;
  onSaved: () => void;
  date: string;
  participants: readonly DailyMealParticipant[];
  others: readonly DailyMealParticipant[];
}>) {
  const t = useTranslations("mealParticipantList");
  const locale = useLocale();
  const today = useBerlinToday();
  const datePicker = useLocalizedDatePicker();
  const { success } = useToast();
  const { error, show: showError, clear: clearError } = useApiFormError();
  const {
    error: scheduleError,
    show: showScheduleError,
    clear: clearScheduleError,
  } = useApiLoadError();
  const [studentId, setStudentId] = useState("");
  const [scheduleState, setScheduleState] = useState<ScheduleState>("idle");
  // Nur die Antwort der letzten Anfrage zählt, wenn schnell gewechselt wird.
  const scheduleRequest = useRef(0);
  const [scope, setScope] = useState<ChangeScope>("day");
  const [choice, setChoice] = useState<DayChoice | null>(null);
  const [weekdays, setWeekdays] = useState<number[]>([]);
  const [effectiveFrom, setEffectiveFrom] = useState(date);
  const [saving, setSaving] = useState(false);

  const eating = useMemo(
    () => new Set(participants.map((child) => child.studentId)),
    [participants],
  );
  const options = useMemo(
    () =>
      [...participants, ...others]
        .sort(
          (a, b) =>
            a.lastName.localeCompare(b.lastName, locale) ||
            a.firstName.localeCompare(b.firstName, locale),
        )
        .map((child) => ({
          value: child.studentId,
          // Name, Klasse und ob das Kind isst; das Muster hat keine Wörter.
          label: [
            `${child.lastName}, ${child.firstName}`,
            child.schoolClass || "–",
            eating.has(child.studentId) ? t("eats") : t("skips"),
          ].join(" · "),
        })),
    [eating, locale, others, participants, t],
  );

  useEffect(() => {
    if (!isOpen) return;
    setStudentId("");
    setScope("day");
    setChoice(null);
    setWeekdays([]);
    setScheduleState("idle");
    scheduleRequest.current += 1;
    setEffectiveFrom(date < today ? today : date);
    clearError();
    clearScheduleError();
  }, [clearError, clearScheduleError, date, isOpen, today]);

  // Die festen Tage des gewählten Kindes vorbelegen. Bis sie da sind, bleiben
  // die Tage leer und Speichern gesperrt, damit nie die Tage eines anderen
  // Kindes gespeichert werden.
  const loadSchedule = useCallback(
    async (child: string, scheduleDate: string) => {
      const request = ++scheduleRequest.current;
      setWeekdays([]);
      setScheduleState("loading");
      clearScheduleError();
      try {
        const schedule = await getChildMealSchedule(child, scheduleDate);
        if (request !== scheduleRequest.current) return;
        setWeekdays(schedule.weekdays);
        setScheduleState("loaded");
      } catch (cause) {
        if (request !== scheduleRequest.current) return;
        logger.error("meal_child_schedule_load_failed", {
          error: cause instanceof Error ? cause.message : String(cause),
        });
        setScheduleState("failed");
        void showScheduleError(cause, {
          object: t("errorObjectSchedule"),
          retry: () => void loadSchedule(child, scheduleDate),
        });
      }
    },
    [clearScheduleError, showScheduleError, t],
  );

  function chooseChild(child: string) {
    setStudentId(child);
    setChoice(eating.has(child) ? "skips" : "eats");
    void loadSchedule(child, effectiveFrom);
  }

  // Die Schule legt feste Tage höchstens ein Jahr im Voraus fest.
  const latestStart = useMemo(() => {
    const date = parseISODate(today);
    date.setDate(date.getDate() + 365);
    return toISODate(date);
  }, [today]);
  const ready =
    studentId !== "" &&
    (scope === "day"
      ? choice !== null
      : effectiveFrom >= today &&
        effectiveFrom <= latestStart &&
        scheduleState === "loaded");
  const childName = useMemo(() => {
    const child = [...participants, ...others].find(
      (candidate) => candidate.studentId === studentId,
    );
    return child ? `${child.firstName} ${child.lastName}` : "";
  }, [others, participants, studentId]);

  async function save() {
    if (!ready) return;
    setSaving(true);
    clearError();
    try {
      if (scope === "day" && choice === "regular") {
        await clearChildMealDay(studentId, date);
      } else if (scope === "day") {
        await setChildMealDay(studentId, date, choice === "eats");
      } else {
        await replaceChildMealSchedule(studentId, weekdays, effectiveFrom);
      }
    } catch (cause) {
      logger.error("meal_registration_change_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void showError(cause, { object: t("errorObjectChange") });
      setSaving(false);
      return;
    }
    setSaving(false);
    success(t("changeSaved", { name: childName }));
    onSaved();
    onClose();
  }

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={t("changeTitle")}
      size="md"
      closeDisabled={saving}
      isBackdropDismissDisabled
      error={error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => void save()}
          saving={saving}
          disabled={!ready}
        />
      }
    >
      <div className="space-y-5">
        <p className="text-sm text-gray-600">{t("changeHint")}</p>
        <div className="space-y-1.5">
          <span
            id="meal-change-child-label"
            className="text-sm font-medium text-gray-800"
          >
            {t("child")}
          </span>
          <CustomSelect
            value={studentId}
            options={options}
            onChange={chooseChild}
            ariaLabelledBy="meal-change-child-label"
            placeholder={t("childPlaceholder")}
          />
        </div>
        <SegmentedControl<ChangeScope>
          items={[
            {
              value: "day",
              label: t("scopeDay", { date: formatDate(date, false, locale) }),
            },
            { value: "regular", label: t("scopeRegular") },
          ]}
          value={scope}
          onChange={setScope}
          ariaLabel={t("scopeAria")}
          fullWidth
        />
        {scope === "day" ? (
          <SegmentedControl<DayChoice>
            items={[
              { value: "eats", label: t("eats") },
              { value: "skips", label: t("skips") },
              { value: "regular", label: t("asRegular") },
            ]}
            value={choice}
            onChange={setChoice}
            ariaLabel={t("choiceAria")}
            fullWidth
          />
        ) : (
          <div className="space-y-3">
            {scheduleError ? <LoadErrorAlert error={scheduleError} /> : null}
            {scheduleState === "loading" ? (
              <p className="text-sm text-gray-600" role="status">
                {t("scheduleLoading")}
              </p>
            ) : null}
            <fieldset disabled={scheduleState !== "loaded"}>
              <legend className="text-sm font-medium text-gray-800">
                {t("regularDays")}
              </legend>
              <div className="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-5">
                {WEEKDAYS.map((weekday) => (
                  <ChoiceTile
                    key={weekday}
                    selected={weekdays.includes(weekday)}
                    className="flex min-h-11 items-center gap-2 px-3 py-2 text-sm font-medium"
                  >
                    <Checkbox
                      checked={weekdays.includes(weekday)}
                      onChange={() =>
                        setWeekdays((current) =>
                          current.includes(weekday)
                            ? current.filter((value) => value !== weekday)
                            : [...current, weekday].sort((a, b) => a - b),
                        )
                      }
                    />
                    {t(`weekday${weekday}`)}
                  </ChoiceTile>
                ))}
              </div>
            </fieldset>
            <ISODatePicker
              label={t("effectiveFrom")}
              value={effectiveFrom}
              onChange={(nextDate) => {
                setEffectiveFrom(nextDate);
                if (studentId) void loadSchedule(studentId, nextDate);
              }}
              min={today}
              max={latestStart}
              hideClearButton
              required
              {...datePicker}
            />
          </div>
        )}
      </div>
    </FormModal>
  );
}
