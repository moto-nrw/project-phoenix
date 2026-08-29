"use client";

import { useCallback, useEffect, useState } from "react";
import { Shield, ShieldCheck } from "lucide-react";

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
  if (compact && !enabled) return null;
  const label = compact
    ? "Familienschutz"
    : enabled
      ? "Eingeschaltet"
      : "Ausgeschaltet";
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
      title={enabled ? "Schutz aufheben" : "Private Angaben schützen"}
      confirmText={enabled ? "Schutz aufheben" : "Schutz einschalten"}
      cancelText="Zurück"
      isConfirmLoading={saving}
      isConfirmDisabled={reason.trim() === ""}
      isDismissDisabled={saving}
      mobileSheet
    >
      <p className="mb-4 text-sm text-gray-700">
        {enabled
          ? "Die Eltern können Anfragen danach wieder miteinander teilen."
          : "Andere Sorgeberechtigte sehen dann keine geteilten Anfragen und Begründungen mehr."}
      </p>
      <label
        htmlFor={`family-protection-reason-${studentId}`}
        className="block space-y-1 text-sm font-medium text-gray-800"
      >
        <span>Grund für die Änderung</span>
        <Textarea
          id={`family-protection-reason-${studentId}`}
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          rows={3}
          placeholder="Zum Beispiel besondere Familiensituation"
        />
      </label>
    </ConfirmationModal>
  );
}

function ProtectionDescription() {
  return (
    <p className="text-sm text-gray-600">
      Eltern können einzelne Anfragen miteinander teilen. Der Familienschutz
      verhindert das für dieses Kind.
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
  const modal = enabled !== null && (
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
  );
  if (compact) {
    return (
      <div className="flex flex-wrap items-center gap-1.5">
        <ProtectionStatus enabled={enabled} compact />
        {canManage && enabled !== null ? (
          <Button
            type="button"
            variant="ghost"
            size="compact"
            className="gap-1.5"
            onClick={() => setModalOpen(true)}
          >
            {enabled ? (
              <ShieldCheck className="size-4" aria-hidden="true" />
            ) : (
              <Shield className="size-4" aria-hidden="true" />
            )}
            {enabled ? "Schutz aufheben" : "Angaben schützen"}
          </Button>
        ) : null}
        {loadError ? (
          <p className="text-moto-red text-sm">
            Der Familienschutz konnte nicht geladen werden.
          </p>
        ) : null}
        {saveError ? (
          <p className="text-moto-red text-sm">
            Die Änderung konnte nicht gespeichert werden.
          </p>
        ) : null}
        {modal}
      </div>
    );
  }
  const content = (
    <>
      <ProtectionDescription />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <ProtectionStatus enabled={enabled} compact={false} />
        {canManage && enabled !== null ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => setModalOpen(true)}
          >
            {enabled ? "Aufheben" : "Einschalten"}
          </Button>
        ) : null}
      </div>
      {loadError ? (
        <p className="text-moto-red text-sm">
          Der Familienschutz konnte nicht geladen werden.
        </p>
      ) : null}
      {saveError ? (
        <p className="text-moto-red text-sm">
          Die Änderung konnte nicht gespeichert werden.
        </p>
      ) : null}
      {modal}
    </>
  );
  return <div className="space-y-3">{content}</div>;
}
