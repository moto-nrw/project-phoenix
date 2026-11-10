"use client";

import { wireErrorCode } from "~/lib/api-error";
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { FieldLabel } from "~/components/ui/field-label";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { Input } from "~/components/ui/input";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  bulkUpsertPickupSchedules,
  type BulkPickupScheduleInput,
} from "~/lib/pickup-schedule-api";
import { WEEKDAYS } from "~/lib/student-arrival-api";
import { InsetPanel } from "~/components/ui/inset-panel";

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
  const { success } = useToast();
  const errors = useApiFormError();
  // „Wiederholen“ speichert die aktuellen Zeiten, nicht die vom Fehler.
  const latestSubmitRef = useRef<() => Promise<void>>(async () => undefined);
  const [times, setTimes] = useState<Record<number, string>>({});
  const save = useFormModalPending(isOpen);
  const saving = save.pending;
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
    errors.clear();
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
    errors.clear();
    if (schedules.length === 0) {
      errors.invalid("Bitte tragen Sie mindestens eine Gehzeit ein.");
      return;
    }
    await save.run(async (op) => {
      try {
        await bulkUpsertPickupSchedules(
          studentIds,
          schedules,
          needsExceptionConfirmation && exceptionConfirmed,
        );
        success(
          `Die Gehzeiten für ${studentIds.length === 1 ? "1 Kind" : `${studentIds.length} Kinder`} sind gespeichert.`,
        );
        onSuccess?.();
        // Ein Speichern, das erst nach dem Aushängen oder Schließen fertig
        // wird, schließt nicht mehr: Der Eltern-Schalter gehört dann schon dem
        // nächsten Dialog (#6503, #6533).
        if (op.isCurrent()) handleClose();
      } catch (err) {
        if (!op.isCurrent()) return;
        if (
          err instanceof Error &&
          "code" in err &&
          wireErrorCode(err.code) ===
            "pickup.bulk_exception_confirmation_required"
        ) {
          setNeedsExceptionConfirmation(true);
          setExceptionConfirmed(false);
          return;
        }
        logger.error("failed to bulk upsert pickup schedules", {
          error: err instanceof Error ? err.message : String(err),
        });
        await errors.show(err, {
          object: "die Gehzeit",
          retry: () => void latestSubmitRef.current(),
        });
      }
    });
  };
  useLayoutEffect(() => {
    latestSubmitRef.current = handleSubmit;
  });

  return (
    <FormModal
      isOpen={isOpen}
      onClose={handleClose}
      // X, Escape und Hintergrund bleiben gesperrt, solange gespeichert wird:
      // Ein scheinbarer Abbruch würde trotzdem speichern (#6503).
      pending={save}
      title="Gehzeiten für Auswahl"
      error={errors.error}
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
            <InsetPanel
              key={day.value}
              framed
              padding="compact"
              className="flex items-center gap-3"
            >
              <FieldLabel
                htmlFor={`bulk-pickup-${day.value}`}
                className="mb-0 w-28"
              >
                {day.label}
              </FieldLabel>
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
            </InsetPanel>
          ))}
        </div>
      </div>
    </FormModal>
  );
}
