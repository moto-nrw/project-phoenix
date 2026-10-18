"use client";

import { useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";

import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useLocalizedDatePicker } from "~/lib/hooks/use-localized-date-picker";
import { createLogger } from "~/lib/logger";
import {
  getChildMealSchedule,
  replaceChildMealSchedule,
  setChildMealDay,
  type DailyMealParticipant,
} from "~/lib/meal-plan-api";

const logger = createLogger({ component: "MealRegistrationChangeModal" });

const WEEKDAYS = [1, 2, 3, 4, 5] as const;

type ChangeScope = "day" | "regular";
type DayChoice = "eats" | "skips";

/**
 * Die Schule ändert die Anmeldung eines Kindes, auch nach der Frist (#3873):
 * einen Tag oder die festen Tage ab einem Datum. Eltern sehen die Änderung
 * im Elternportal.
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
  const [studentId, setStudentId] = useState("");
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
          label: t("childOption", {
            name: `${child.lastName}, ${child.firstName}`,
            schoolClass: child.schoolClass || "–",
            state: eating.has(child.studentId) ? t("eats") : t("skips"),
          }),
        })),
    [eating, locale, others, participants, t],
  );

  useEffect(() => {
    if (!isOpen) return;
    setStudentId("");
    setScope("day");
    setChoice(null);
    setWeekdays([]);
    setEffectiveFrom(date < today ? today : date);
    clearError();
  }, [clearError, date, isOpen, today]);

  // Die festen Tage des gewählten Kindes vorbelegen.
  useEffect(() => {
    if (!isOpen || !studentId) return;
    let cancelled = false;
    setChoice(eating.has(studentId) ? "skips" : "eats");
    void getChildMealSchedule(studentId, date)
      .then((schedule) => {
        if (!cancelled) setWeekdays(schedule.weekdays);
      })
      .catch((cause: unknown) => {
        logger.error("meal_child_schedule_load_failed", {
          error: cause instanceof Error ? cause.message : String(cause),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [date, eating, isOpen, studentId]);

  const ready =
    studentId !== "" &&
    (scope === "day" ? choice !== null : effectiveFrom >= today);

  async function save() {
    if (!ready) return;
    setSaving(true);
    clearError();
    try {
      if (scope === "day") {
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
    success(t("changeSaved"));
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
            onChange={setStudentId}
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
            ]}
            value={choice}
            onChange={setChoice}
            ariaLabel={t("choiceAria")}
            fullWidth
          />
        ) : (
          <div className="space-y-3">
            <fieldset>
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
              onChange={setEffectiveFrom}
              min={today}
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
