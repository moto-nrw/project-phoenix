"use client";

import { useCallback, useEffect, useState } from "react";

import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { StatusBadge } from "~/components/ui/status-badge";
import { Textarea } from "~/components/ui/textarea";
import {
  getFamilyProtection,
  setFamilyProtection,
} from "~/lib/change-request-list-api";

interface FamilyProtectionControlProps {
  readonly studentId: string;
  readonly canManage: boolean;
  readonly initialEnabled?: boolean;
  readonly compact?: boolean;
  readonly onChanged?: (enabled: boolean) => void;
}

function useProtectionValue(studentId: string, initialEnabled?: boolean) {
  const [enabled, setEnabled] = useState<boolean | null>(
    initialEnabled ?? null,
  );
  const [loadError, setLoadError] = useState(false);
  useEffect(() => {
    if (initialEnabled !== undefined) {
      setEnabled(initialEnabled);
      return;
    }
    let active = true;
    setEnabled(null);
    void getFamilyProtection(studentId)
      .then((state) => active && setEnabled(state.enabled))
      .catch(() => active && setLoadError(true));
    return () => {
      active = false;
    };
  }, [initialEnabled, studentId]);
  return { enabled, setEnabled, loadError };
}

function useProtectionSave(
  studentId: string,
  enabled: boolean | null,
  onSaved: (enabled: boolean) => void,
) {
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState(false);
  const save = useCallback(
    async (reason: string) => {
      if (enabled === null || reason.trim() === "") return false;
      setSaving(true);
      setSaveError(false);
      try {
        const next = !enabled;
        await setFamilyProtection(studentId, next, reason.trim());
        onSaved(next);
        return true;
      } catch {
        setSaveError(true);
        return false;
      } finally {
        setSaving(false);
      }
    },
    [enabled, onSaved, studentId],
  );
  return { save, saveError, saving };
}

function ProtectionStatus({
  enabled,
  compact,
}: Readonly<{
  enabled: boolean | null;
  compact: boolean;
}>) {
  if (enabled === null) return null;
  const label = compact
    ? `Familienschutz ${enabled ? "aktiv" : "aus"}`
    : enabled
      ? "Aktiv"
      : "Aus";
  return <StatusBadge tone={enabled ? "red" : "gray"} label={label} />;
}

interface ProtectionModalProps {
  readonly studentId: string;
  readonly enabled: boolean;
  readonly open: boolean;
  readonly saving: boolean;
  readonly reason: string;
  readonly setReason: (value: string) => void;
  readonly close: () => void;
  readonly save: () => void;
}

function ProtectionModal(props: ProtectionModalProps) {
  const { studentId, enabled, open, saving, reason, setReason, close, save } =
    props;
  return (
    <ConfirmationModal
      isOpen={open}
      onClose={close}
      onConfirm={save}
      title={enabled ? "Familienschutz aufheben" : "Familienschutz einschalten"}
      confirmText={enabled ? "Schutz aufheben" : "Schutz einschalten"}
      cancelText="Zurück"
      isConfirmLoading={saving}
      isConfirmDisabled={reason.trim() === ""}
      isDismissDisabled={saving}
      mobileSheet
    >
      <label
        htmlFor={`family-protection-reason-${studentId}`}
        className="block space-y-1 text-sm font-medium text-gray-800"
      >
        <span>Begründung</span>
        <Textarea
          id={`family-protection-reason-${studentId}`}
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          rows={3}
        />
      </label>
    </ConfirmationModal>
  );
}

function ProtectionDescription() {
  return (
    <p className="text-xs text-gray-600">
      Alle berechtigten Sorgeberechtigten sehen den wirksamen Stand des Kindes.
      Wer eine Anfrage gestellt hat und freie Begründungen sehen nur die
      einreichende Person und die OGS. Bei aktivem Familienschutz können private
      Angaben nicht mit anderen Sorgeberechtigten geteilt werden.
    </p>
  );
}

export function FamilyProtectionControl({
  studentId,
  canManage,
  initialEnabled,
  compact = false,
  onChanged,
}: FamilyProtectionControlProps) {
  const { enabled, setEnabled, loadError } = useProtectionValue(
    studentId,
    initialEnabled,
  );
  const [reason, setReason] = useState("");
  const [modalOpen, setModalOpen] = useState(false);
  const onSaved = useCallback(
    (next: boolean) => {
      setEnabled(next);
      onChanged?.(next);
      setReason("");
      setModalOpen(false);
    },
    [onChanged, setEnabled],
  );
  const { save, saveError, saving } = useProtectionSave(
    studentId,
    enabled,
    onSaved,
  );
  const content = (
    <>
      <div className="flex flex-wrap items-center justify-between gap-2">
        {!compact && (
          <span className="font-medium text-gray-900">Familienschutz</span>
        )}
        <ProtectionStatus enabled={enabled} compact={compact} />
      </div>
      {!compact && <ProtectionDescription />}
      {(loadError || saveError) && (
        <p className="text-moto-red text-sm">
          Der Familienschutz konnte nicht gespeichert oder geladen werden.
        </p>
      )}
      {canManage && enabled !== null && (
        <Button
          type="button"
          variant="outline"
          onClick={() => setModalOpen(true)}
        >
          Familienschutz {enabled ? "aufheben" : "einschalten"}
        </Button>
      )}
      {enabled !== null && (
        <ProtectionModal
          studentId={studentId}
          enabled={enabled}
          open={modalOpen}
          saving={saving}
          reason={reason}
          setReason={setReason}
          close={() => setModalOpen(false)}
          save={() => void save(reason)}
        />
      )}
    </>
  );
  return compact ? (
    <div className="flex flex-wrap items-center gap-2">{content}</div>
  ) : (
    <div className="border-moto-sand mt-4 space-y-3 border-t pt-4">
      {content}
    </div>
  );
}
