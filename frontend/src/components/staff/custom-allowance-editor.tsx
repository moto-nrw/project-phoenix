"use client";

import { useState } from "react";

import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { formatDayCount } from "~/lib/absence-helpers";
import {
  absenceTypeService,
  type AbsenceType,
  type AbsenceTypeAllowanceSummary,
} from "~/lib/absence-type-api";

type EditorProps = {
  readonly staffId: string;
  readonly year: number;
  readonly entry: {
    type: AbsenceType;
    summary: AbsenceTypeAllowanceSummary;
  };
  readonly onClose: () => void;
  readonly onSaved: () => Promise<void>;
};

export function AllowanceValue({
  label,
  value,
}: {
  label: string;
  value: number;
}) {
  return (
    <div className="rounded-lg bg-gray-50 px-3 py-2">
      <dt className="text-xs text-gray-500">{label}</dt>
      <dd className="font-semibold text-gray-900 tabular-nums">
        {formatDayCount(value)}
      </dd>
    </div>
  );
}

function useAllowanceEditor({ staffId, year, entry, onSaved }: EditorProps) {
  const [days, setDays] = useState(String(entry.summary.entitledDays));
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const toast = useToast();
  const entitledDays = Number(days.replace(",", "."));
  const validDays =
    Number.isFinite(entitledDays) &&
    entitledDays >= 0 &&
    entitledDays <= 366 &&
    Number.isInteger(entitledDays * 2);
  const save = async () => {
    if (!validDays || reason.trim() === "") return;
    setSaving(true);
    try {
      await absenceTypeService.setAllowance(entry.type.id, staffId, {
        year,
        entitledDays,
        reason: reason.trim(),
      });
      toast.success("Anspruch gespeichert.");
      await onSaved();
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : "Anspruch konnte nicht gespeichert werden.",
      );
    } finally {
      setSaving(false);
    }
  };
  return { days, setDays, reason, setReason, saving, validDays, save };
}

function EditorFooter({
  saving,
  disabled,
  onClose,
  onSave,
}: {
  saving: boolean;
  disabled: boolean;
  onClose: () => void;
  onSave: () => void;
}) {
  return (
    <div className="flex w-full justify-end gap-2">
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
        onClick={onSave}
        disabled={disabled}
      >
        Speichern
      </Button>
    </div>
  );
}

function EditorFields({
  days,
  reason,
  setDays,
  setReason,
}: {
  days: string;
  reason: string;
  setDays: (value: string) => void;
  setReason: (value: string) => void;
}) {
  return (
    <div className="space-y-4">
      <div>
        <Input
          id="custom-allowance-days"
          label="Anspruch in Tagen"
          inputMode="decimal"
          value={days}
          onChange={(event) => setDays(event.target.value)}
        />
        <p className="mt-1 text-xs text-gray-500">
          Ganze und halbe Tage sind möglich.
        </p>
      </div>
      <div>
        <Input
          id="custom-allowance-reason"
          label="Begründung"
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          placeholder="z. B. Tariflicher Anspruch"
        />
        <p className="mt-1 text-xs text-gray-500">
          Die Begründung erscheint im Änderungsprotokoll.
        </p>
      </div>
    </div>
  );
}

export function EditCustomAllowanceModal(props: EditorProps) {
  const state = useAllowanceEditor(props);
  const disabled =
    state.saving || !state.validDays || state.reason.trim() === "";
  return (
    <Modal
      isOpen
      onClose={() => !state.saving && props.onClose()}
      title={`${props.entry.type.name}: Anspruch ${props.year}`}
      footer={
        <EditorFooter
          saving={state.saving}
          disabled={disabled}
          onClose={props.onClose}
          onSave={() => void state.save()}
        />
      }
    >
      <EditorFields
        days={state.days}
        reason={state.reason}
        setDays={state.setDays}
        setReason={state.setReason}
      />
    </Modal>
  );
}
