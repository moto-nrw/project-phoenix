"use client";

// Anspruch einer eigenen Abwesenheitsart für eine Person (#2874, #3256).
// Bearbeitet wird an Ort und Stelle der Kontingent-Karte (BAUARTEN-SPEC
// Bauart 2): Tage mit Minus und Plus oder direkt eintippen, Begründung für das
// Änderungsprotokoll, unten `EditActions`.

import { Minus, Plus } from "lucide-react";
import { useState } from "react";

import { Button } from "~/components/ui/button";
import { EditActions } from "~/components/ui/edit-actions";
import { useFormError } from "~/components/ui/form-error";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { useToast } from "~/contexts/ToastContext";
import { formatDayCount } from "~/lib/absence-helpers";
import {
  absenceTypeService,
  type AbsenceType,
  type AbsenceTypeAllowanceSummary,
} from "~/lib/absence-type-api";

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
  const [fieldErrors, setFieldErrors] = useState<{
    days?: string;
    reason?: string;
  }>({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useFormError();
  const toast = useToast();

  const used = summary.takenDays + summary.reservedDays;
  const parsed = parseDays(days);

  const step = (delta: number) => {
    const next = Math.min(MAX_DAYS, Math.max(used, (parsed ?? 0) + delta));
    setDays(formatInput(next));
    setFieldErrors((current) => ({ ...current, days: undefined }));
  };

  const save = async () => {
    setError(null);
    const nextErrors = {
      days:
        parsed === null
          ? "Bitte ganze oder halbe Tage zwischen 0 und 366 eintragen."
          : parsed < used
            ? `Mindestens ${formatDayCount(used)}. So viele sind schon eingetragen oder beantragt.`
            : undefined,
      reason:
        reason.trim() === ""
          ? "Bitte kurz sagen, warum sich der Anspruch ändert."
          : undefined,
    };
    setFieldErrors(nextErrors);
    if (nextErrors.days || nextErrors.reason || parsed === null) {
      setError("Bitte prüfen Sie die markierten Felder.");
      return;
    }
    setSaving(true);
    try {
      await absenceTypeService.setAllowance(type.id, staffId, {
        year,
        entitledDays: parsed,
        reason: reason.trim(),
      });
      toast.success(`Anspruch ${type.name} gespeichert.`);
      await onSaved();
    } catch (cause) {
      setError(
        cause instanceof Error && cause.message
          ? cause.message
          : "Der Anspruch konnte nicht gespeichert werden.",
      );
    } finally {
      setSaving(false);
    }
  };

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
      <FormErrorAlert message={error} />
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
            inputMode="decimal"
            controlSize="compact"
            className="w-24 text-center tabular-nums"
            value={days}
            onChange={(event) => setDays(event.target.value)}
            error={fieldErrors.days}
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
        label="Begründung"
        value={reason}
        onChange={(event) => setReason(event.target.value)}
        placeholder="z. B. in den Sommerferien krank"
        error={fieldErrors.reason}
        disabled={saving}
      />
      <EditActions onCancel={onCancel} saving={saving} />
    </form>
  );
}
