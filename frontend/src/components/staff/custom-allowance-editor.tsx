"use client";

// Anspruch einer eigenen Abwesenheitsart für eine Person (#2874, #3256).
// Bearbeitet wird an Ort und Stelle der Kontingent-Karte (BAUARTEN-SPEC
// Bauart 2): Tage mit Minus und Plus oder direkt eintippen, Begründung für das
// Änderungsprotokoll, unten `EditActions`.

import { Minus, Plus } from "lucide-react";
import { useLayoutEffect, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { formatDayCount } from "~/lib/absence-helpers";
import {
  absenceTypeService,
  type AbsenceType,
  type AbsenceTypeAllowanceSummary,
} from "~/lib/absence-type-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CustomAllowanceEditor" });

const MAX_DAYS = 366;

function parseDays(raw: string): number | null {
  const value = Number(raw.trim().replace(",", "."));
  if (raw.trim() === "" || !Number.isFinite(value)) return null;
  if (value < 0 || value > MAX_DAYS || !Number.isInteger(value * 2)) {
    return null;
  }
  return value;
}

function formatInput(days: number): string {
  return String(days).replace(".", ",");
}

export function CustomAllowanceEditForm({
  staffId,
  year,
  type,
  summary,
  onCancel,
  onSaved,
}: {
  readonly staffId: string;
  readonly year: number;
  readonly type: AbsenceType;
  readonly summary: AbsenceTypeAllowanceSummary;
  readonly onCancel: () => void;
  readonly onSaved: () => void | Promise<void>;
}) {
  const [days, setDays] = useState(formatInput(summary.entitledDays));
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const formErrors = useApiFormError();
  const toast = useToast();
  // „Wiederholen“ sendet den aktuellen Entwurf, nicht den vom Fehlerzeitpunkt.
  const latestSaveRef = useRef<() => Promise<void>>(async () => undefined);

  const used = summary.takenDays + summary.reservedDays;
  const parsed = parseDays(days);

  const step = (delta: number) => {
    const next = Math.min(MAX_DAYS, Math.max(used, (parsed ?? 0) + delta));
    setDays(formatInput(next));
  };

  const save = async () => {
    formErrors.clear();
    const daysError =
      parsed === null
        ? "Bitte ganze oder halbe Tage zwischen 0 und 366 eintragen."
        : parsed < used
          ? `Mindestens ${formatDayCount(used)}. So viele sind schon eingetragen oder beantragt.`
          : undefined;
    const reasonError =
      reason.trim() === ""
        ? "Bitte kurz sagen, warum sich der Anspruch ändert."
        : undefined;
    if (daysError || reasonError || parsed === null) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", {
        ...(daysError ? { entitled_days: daysError } : {}),
        ...(reasonError ? { reason: reasonError } : {}),
      });
      return;
    }
    setSaving(true);
    try {
      await absenceTypeService.setAllowance(type.id, staffId, {
        year,
        entitledDays: parsed,
        reason: reason.trim(),
      });
      toast.success(`Der Anspruch ${type.name} ist gespeichert.`);
      await onSaved();
    } catch (cause) {
      logger.error("custom_allowance_save_failed", {
        staff_id: staffId,
        absence_type_id: type.id,
        error: cause instanceof Error ? cause.message : String(cause),
      });
      await formErrors.show(cause, {
        object: "die Änderung am Anspruch",
        retry: () => void latestSaveRef.current(),
      });
    } finally {
      setSaving(false);
    }
  };

  useLayoutEffect(() => {
    latestSaveRef.current = save;
  });

  return (
    <form
      className="space-y-4"
      noValidate
      aria-label={`Anspruch ${type.name} ${year}`}
      onSubmit={(event) => {
        event.preventDefault();
        void save();
      }}
    >
      <FormErrorAlert message={formErrors.error} />
      <div>
        <label
          htmlFor={`allowance-days-${type.id}`}
          className="mb-1 block text-sm font-medium text-gray-700"
        >
          Anspruch {year} in Tagen
        </label>
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="icon"
            onClick={() => step(-1)}
            disabled={saving || (parsed ?? 0) - 1 < used}
            aria-label="Einen Tag weniger"
          >
            <Minus className="h-4 w-4" aria-hidden />
          </Button>
          <Input
            id={`allowance-days-${type.id}`}
            name="entitled_days"
            inputMode="decimal"
            controlSize="compact"
            className="w-24 text-center tabular-nums"
            value={days}
            onChange={(event) => setDays(event.target.value)}
            error={formErrors.fieldError("entitled_days")}
            disabled={saving}
          />
          <Button
            type="button"
            variant="outline"
            size="icon"
            onClick={() => step(1)}
            disabled={saving || (parsed ?? 0) + 1 > MAX_DAYS}
            aria-label="Einen Tag mehr"
          >
            <Plus className="h-4 w-4" aria-hidden />
          </Button>
        </div>
        <p className="mt-1 text-xs text-gray-500">
          Halbe Tage mit Komma, z. B. 1,5. Beim Start mit moto: die Tage
          eintragen, die noch übrig sind.
        </p>
      </div>
      <Input
        id={`allowance-reason-${type.id}`}
        name="reason"
        label="Begründung"
        value={reason}
        onChange={(event) => setReason(event.target.value)}
        placeholder="z. B. in den Sommerferien krank"
        error={formErrors.fieldError("reason")}
        disabled={saving}
      />
      <EditActions onCancel={onCancel} saving={saving} />
    </form>
  );
}
