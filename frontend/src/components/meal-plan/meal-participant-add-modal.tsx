"use client";

import { useLayoutEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";

import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { FormModal } from "~/components/ui/form-modal";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import {
  setMealParticipantDay,
  type DailyMealParticipant,
} from "~/lib/meal-plan-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "MealParticipantAddModal" });

/**
 * Das Team meldet ein Kind des Tages zum Mittagessen an, auch nach der
 * Frist (#3164), zum Beispiel nach einem Anruf der Familie.
 */
export function MealParticipantAddModal({
  date,
  dateLabel,
  available,
  onClose,
  onAdded,
}: Readonly<{
  date: string;
  dateLabel: string;
  available: readonly DailyMealParticipant[];
  onClose: () => void;
  onAdded: () => void | Promise<void>;
}>) {
  const t = useTranslations("mealParticipantList");
  const toast = useToast();
  const formErrors = useApiFormError();
  const [studentId, setStudentId] = useState("");
  const [saving, setSaving] = useState(false);
  // „Wiederholen“ sendet die aktuelle Auswahl, nicht die vom Fehlerzeitpunkt.
  const retryRef = useRef<() => void>(() => undefined);

  async function save() {
    formErrors.clear();
    const child = available.find(
      (participant) => participant.studentId === studentId,
    );
    if (!child) {
      formErrors.invalid(t("addChildRequired"), {
        student_id: t("addChildRequired"),
      });
      return;
    }
    setSaving(true);
    try {
      await setMealParticipantDay(child.studentId, date, true);
      toast.success(
        t("registeredToast", {
          name: `${child.firstName} ${child.lastName}`,
          date: dateLabel,
        }),
      );
      await onAdded();
    } catch (cause) {
      logger.error("meal_participant_add_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      await formErrors.show(cause, {
        object: t("changeErrorObject"),
        retry: () => retryRef.current(),
      });
    } finally {
      setSaving(false);
    }
  }
  useLayoutEffect(() => {
    retryRef.current = () => void save();
  });

  return (
    <FormModal
      isOpen
      onClose={onClose}
      closeDisabled={saving}
      title={t("addChildTitle", { date: dateLabel })}
      size="sm"
      error={formErrors.error}
      footer={
        <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={saving}
          >
            {t("cancel")}
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void save()}
            isLoading={saving}
            loadingText={t("saving")}
            disabled={saving || available.length === 0}
          >
            {t("addChild")}
          </Button>
        </div>
      }
    >
      {available.length === 0 ? (
        <p className="text-sm text-gray-700">{t("addChildEmpty")}</p>
      ) : (
        <div className="space-y-2">
          <label
            id="meal-participant-add-label"
            htmlFor="meal-participant-add-child"
            className="block text-sm font-medium text-gray-700"
          >
            {t("addChildLabel")}
          </label>
          <CustomSelect
            id="meal-participant-add-child"
            name="student_id"
            labelId="meal-participant-add-label"
            value={studentId}
            onChange={setStudentId}
            placeholder={t("addChildPlaceholder")}
            invalid={Boolean(formErrors.fieldError("student_id"))}
            disabled={saving}
            options={available.map((participant) => ({
              value: participant.studentId,
              label: participant.schoolClass
                ? `${participant.lastName}, ${participant.firstName} (${participant.schoolClass})`
                : `${participant.lastName}, ${participant.firstName}`,
            }))}
          />
        </div>
      )}
    </FormModal>
  );
}
