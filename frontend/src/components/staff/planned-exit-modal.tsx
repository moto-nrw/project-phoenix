"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { useApiFormError } from "~/contexts/ToastContext";
import { formatDate, todayISO } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { staffPlannedExitService } from "~/lib/staff-api";

const logger = createLogger({ component: "PlannedExitModal" });

interface PlannedExitModalProps {
  readonly isOpen: boolean;
  readonly staffId: string;
  readonly staffName: string;
  /** Der schon geplante letzte Arbeitstag, "YYYY-MM-DD". */
  readonly plannedDay?: string;
  /** Vertragsende aus den Stammdaten als Vorschlag, "YYYY-MM-DD". */
  readonly contractEndDate?: string | null;
  readonly onClose: () => void;
  readonly onSaved: (lastWorkingDay: string) => void;
}

/**
 * Austritt planen oder ändern (#4809): Die Leitung trägt den letzten
 * Arbeitstag ein. Nach dem Ende dieses Tages löscht moto die Person wie über
 * „Löschen“; bis dahin arbeitet sie normal.
 */
export function PlannedExitModal({
  isOpen,
  staffId,
  staffName,
  plannedDay,
  contractEndDate,
  onClose,
  onSaved,
}: PlannedExitModalProps) {
  const today = todayISO();
  // Ein Vertragsende in der Vergangenheit ist kein Vorschlag mehr.
  const suggestion =
    contractEndDate && contractEndDate >= today ? contractEndDate : undefined;
  const [day, setDay] = useState(plannedDay ?? suggestion ?? "");
  const [saving, setSaving] = useState(false);
  const formErrors = useApiFormError();
  const clearFormErrors = formErrors.clear;
  const latestSaveRef = useRef<() => Promise<void>>(async () => undefined);

  useEffect(() => {
    if (isOpen) {
      setDay(plannedDay ?? suggestion ?? "");
      setSaving(false);
      clearFormErrors();
    }
  }, [isOpen, plannedDay, suggestion, clearFormErrors]);

  const handleSave = async () => {
    setSaving(true);
    formErrors.clear();
    try {
      const saved = await staffPlannedExitService.plan(staffId, day);
      onSaved(saved);
    } catch (err) {
      logger.error("planned_exit_save_failed", {
        staff_id: staffId,
        error: err instanceof Error ? err.message : String(err),
      });
      await formErrors.show(err, {
        object: "den Austritt",
        retry: () => void latestSaveRef.current(),
      });
    } finally {
      setSaving(false);
    }
  };

  useLayoutEffect(() => {
    latestSaveRef.current = handleSave;
  });

  const footer = (
    <>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onClose}
        disabled={saving}
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={() => void handleSave()}
        isLoading={saving}
        loadingText="Wird gespeichert…"
        disabled={saving || day === "" || day < today}
      >
        {plannedDay ? "Speichern" : "Austritt planen"}
      </Button>
    </>
  );

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={plannedDay ? "Austritt ändern" : "Austritt planen"}
      size="md"
      footer={footer}
      closeDisabled={saving}
      error={formErrors.error}
    >
      <div className="space-y-4 text-sm text-gray-700">
        <p>
          {staffName} arbeitet bis zum Ende dieses Tages normal. Danach endet
          der Zugang zu moto, und die Person verschwindet aus allen Listen.
        </p>
        <ISODatePicker
          label="Letzter Arbeitstag"
          value={day}
          onChange={setDay}
          min={today}
          placeholder="Datum auswählen"
        />
        {suggestion && !plannedDay ? (
          <p className="text-gray-600">
            Vorgeschlagen ist das Vertragsende aus den Stammdaten:{" "}
            {formatDate(suggestion)}.
          </p>
        ) : null}
        <p className="text-gray-600">
          Anwesenheiten und Zeiterfassung bleiben bis zum Ende ihrer Frist
          erhalten. Sie bekommen eine Woche vorher und am letzten Arbeitstag
          eine Erinnerung. Bis dahin können Sie den Tag ändern oder den Austritt
          zurücknehmen.
        </p>
      </div>
    </FormModal>
  );
}
