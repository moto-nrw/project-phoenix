"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  bulkUpsertPickupSchedules,
  type BulkPickupScheduleInput,
} from "~/lib/pickup-schedule-api";
import { WEEKDAYS } from "~/lib/student-arrival-api";

const logger = createLogger({ component: "SelectionBulkPickupModal" });

interface SelectionBulkPickupModalProps {
  isOpen: boolean;
  onClose: () => void;
  studentIds: string[];
  onSuccess?: () => void;
}

export function SelectionBulkPickupModal({
  isOpen,
  onClose,
  studentIds,
  onSuccess,
}: SelectionBulkPickupModalProps) {
  const { success, error } = useToast();
  const [times, setTimes] = useState<Record<number, string>>({});
  const [saving, setSaving] = useState(false);
  const [needsExceptionConfirmation, setNeedsExceptionConfirmation] =
    useState(false);
  const [exceptionConfirmed, setExceptionConfirmed] = useState(false);
  const exceptionHeadingRef = useRef<HTMLHeadingElement>(null);
  const schedules = useMemo<BulkPickupScheduleInput[]>(
    () =>
      WEEKDAYS.flatMap(({ value }) => {
        const pickupTime = times[value]?.trim();
        return pickupTime ? [{ weekday: value, pickup_time: pickupTime }] : [];
      }),
    [times],
  );

  const handleClose = () => {
    setNeedsExceptionConfirmation(false);
    setExceptionConfirmed(false);
    onClose();
  };

  useEffect(() => {
    if (needsExceptionConfirmation) {
      exceptionHeadingRef.current?.focus();
    }
  }, [needsExceptionConfirmation]);

  const handleSubmit = async () => {
    if (schedules.length === 0) {
      error("Mindestens eine Gehzeit angeben");
      return;
    }
    setSaving(true);
    try {
      await bulkUpsertPickupSchedules(
        studentIds,
        schedules,
        needsExceptionConfirmation && exceptionConfirmed,
      );
      success(
        `Gehzeiten für ${studentIds.length === 1 ? "1 Kind" : `${studentIds.length} Kinder`} gesetzt`,
      );
      onSuccess?.();
      handleClose();
    } catch (err) {
      if (
        err instanceof Error &&
        "code" in err &&
        err.code === "pickup.bulk_exception_confirmation_required"
      ) {
        setNeedsExceptionConfirmation(true);
        setExceptionConfirmed(false);
        return;
      }
      const message = err instanceof Error ? err.message : String(err);
      logger.error("failed to bulk upsert pickup schedules", {
        error: message,
      });
      error(message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={handleClose}
      title="Gehzeiten für Auswahl"
      size="md"
      mobilePosition="bottom"
      footer={
        <div className="flex justify-end gap-2 p-4">
          <Button
            variant="outline"
            size="md"
            onClick={handleClose}
            disabled={saving}
          >
            Abbrechen
          </Button>
          <Button
            variant="success"
            size="md"
            onClick={handleSubmit}
            disabled={
              saving ||
              schedules.length === 0 ||
              (needsExceptionConfirmation && !exceptionConfirmed)
            }
          >
            {saving
              ? "Speichern..."
              : needsExceptionConfirmation
                ? "Ausnahmen speichern"
                : `Für ${studentIds.length} setzen`}
          </Button>
        </div>
      }
    >
      <div className="space-y-4 p-4">
        <p className="text-sm text-gray-600">
          Nur ausgefüllte Wochentage werden geändert. Andere Gehzeiten und
          vorhandene Notizen bleiben unverändert.
        </p>
        {needsExceptionConfirmation ? (
          <div className="space-y-3">
            <h3 ref={exceptionHeadingRef} tabIndex={-1} className="sr-only">
              Dauerhafte Ausnahmen bestätigen
            </h3>
            <Alert
              type="warning"
              message="Die Zeiten werden für alle ausgewählten Kinder als dauerhafte Ausnahmen gespeichert. Die gebuchten Angebote ändern sich nicht."
            />
            <label
              htmlFor="bulk-pickup-exception-confirmation"
              className="flex cursor-pointer items-start gap-3 text-sm text-gray-800"
            >
              <Checkbox
                id="bulk-pickup-exception-confirmation"
                checked={exceptionConfirmed}
                onChange={(event) =>
                  setExceptionConfirmed(event.target.checked)
                }
              />
              <span>Ich bestätige die dauerhaften Ausnahmen.</span>
            </label>
          </div>
        ) : null}
        <div className="space-y-2">
          {WEEKDAYS.map((day) => (
            <div
              key={day.value}
              className="flex items-center gap-3 rounded-lg border border-gray-200 bg-gray-50 px-4 py-2.5"
            >
              <label
                htmlFor={`bulk-pickup-${day.value}`}
                className="w-28 text-sm font-medium text-gray-700"
              >
                {day.label}
              </label>
              <Input
                id={`bulk-pickup-${day.value}`}
                type="time"
                controlSize="compact"
                value={times[day.value] ?? ""}
                onChange={(event) => {
                  setNeedsExceptionConfirmation(false);
                  setExceptionConfirmed(false);
                  setTimes((previous) => ({
                    ...previous,
                    [day.value]: event.target.value.slice(0, 5),
                  }));
                }}
                className="w-32"
              />
              {!times[day.value] ? (
                <span className="text-xs text-gray-400 italic">
                  nicht ändern
                </span>
              ) : null}
            </div>
          ))}
        </div>
      </div>
    </FormModal>
  );
}
