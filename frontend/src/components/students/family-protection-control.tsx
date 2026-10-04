"use client";

import {
  type ReactNode,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { Shield, ShieldCheck } from "lucide-react";

import { Button } from "~/components/ui/button";
import type { FormError } from "~/components/ui/form-error";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { LockIcon } from "@phosphor-icons/react/ssr";

import { StatusBadge } from "~/components/ui/status-badge";
import { Textarea } from "~/components/ui/textarea";
import {
  getFamilyProtection,
  setFamilyProtection,
} from "~/lib/change-request-list-api";

/** Objekt der Fehlertexte aus dem Katalog (#2513). */
const PROTECTION_OBJECT = "die Einstellung zum Familienschutz";

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
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  // „Wiederholen“ zählt hoch und lädt damit neu.
  const [loadAttempt, setLoadAttempt] = useState(0);
  useEffect(() => {
    if (initialEnabled !== undefined) {
      setEnabled(initialEnabled);
      return;
    }
    let active = true;
    setEnabled(null);
    clearLoadError();
    void getFamilyProtection(studentId)
      .then((state) => active && setEnabled(state.enabled))
      .catch((err: unknown) => {
        if (!active) return;
        void showLoadError(err, {
          object: PROTECTION_OBJECT,
          retry: () => setLoadAttempt((attempt) => attempt + 1),
        });
      });
    return () => {
      active = false;
    };
  }, [clearLoadError, initialEnabled, loadAttempt, showLoadError, studentId]);
  return { enabled, setEnabled, loadError };
}

function useProtectionSave(
  studentId: string,
  enabled: boolean | null,
  onSaved: (enabled: boolean) => void,
) {
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLDivElement>(null);
  const errors = useApiFormError(formRef);
  const { show: showError, clear: clearError } = errors;
  // „Wiederholen“ sendet den aktuellen Grund, nicht den vom Fehler.
  const latestSaveRef = useRef<() => void>(() => undefined);
  const save = useCallback(
    async (reason: string) => {
      if (enabled === null || reason.trim() === "") return false;
      setSaving(true);
      clearError();
      try {
        const next = !enabled;
        await setFamilyProtection(studentId, next, reason.trim());
        onSaved(next);
        return true;
      } catch (err) {
        await showError(err, {
          object: PROTECTION_OBJECT,
          retry: () => latestSaveRef.current(),
        });
        return false;
      } finally {
        setSaving(false);
      }
    },
    [clearError, enabled, onSaved, showError, studentId],
  );
  return {
    save,
    saveError: errors.error,
    fieldError: errors.fieldError,
    clearError,
    formRef,
    latestSaveRef,
    saving,
  };
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
  // Blau statt rot (#2267): Familienschutz ist kein Fehler und kein
  // Widerspruch. Rot gehoert in dieser Liste den Widersprüchen; zwei
  // verschiedene Zustände duerfen nie denselben Farbton tragen.
  return (
    <span className="inline-flex items-center gap-1">
      {enabled ? <LockIcon size={14} weight="fill" aria-hidden="true" /> : null}
      <StatusBadge tone={enabled ? "blue" : "gray"} label={label} />
    </span>
  );
}

interface ProtectionModalProps {
  readonly studentId: string;
  readonly enabled: boolean;
  readonly open: boolean;
  readonly saving: boolean;
  readonly error: FormError | null;
  readonly fieldError: (name: string) => string | undefined;
  readonly formRef: React.RefObject<HTMLDivElement | null>;
  readonly reason: string;
  readonly setReason: (value: string) => void;
  readonly close: () => void;
  readonly save: () => void;
}

function ProtectionModal(props: ProtectionModalProps) {
  const {
    studentId,
    enabled,
    open,
    saving,
    error,
    fieldError,
    formRef,
    reason,
    setReason,
    close,
    save,
  } = props;
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
      <div ref={formRef}>
        <FormErrorAlert message={error} className="mb-4" />
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
            name="reason"
            value={reason}
            error={fieldError("reason")}
            onChange={(event) => setReason(event.target.value)}
            rows={3}
            placeholder="Zum Beispiel besondere Familiensituation"
          />
        </label>
      </div>
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

function ProtectionAction({
  enabled,
  compact,
  open,
}: Readonly<{ enabled: boolean | null; compact: boolean; open: () => void }>) {
  if (enabled === null) return null;
  return (
    <Button
      type="button"
      variant={compact ? "ghost" : "outline"}
      size="md"
      className={compact ? "gap-1.5 max-sm:min-h-11" : "max-sm:min-h-11"}
      onClick={open}
    >
      {compact ? (
        enabled ? (
          <ShieldCheck className="size-4" aria-hidden="true" />
        ) : (
          <Shield className="size-4" aria-hidden="true" />
        )
      ) : null}
      {compact
        ? enabled
          ? "Schutz aufheben"
          : "Angaben schützen"
        : enabled
          ? "Aufheben"
          : "Einschalten"}
    </Button>
  );
}

interface ProtectionViewProps {
  enabled: boolean | null;
  canManage: boolean;
  loadError: FormError | null;
  open: () => void;
  modal: ReactNode;
}

function CompactProtectionView(props: ProtectionViewProps) {
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <ProtectionStatus enabled={props.enabled} compact />
      {props.canManage ? (
        <ProtectionAction enabled={props.enabled} compact open={props.open} />
      ) : null}
      <LoadErrorAlert error={props.loadError} />
      {props.modal}
    </div>
  );
}

function FullProtectionView(props: ProtectionViewProps) {
  return (
    <div className="space-y-3">
      <ProtectionDescription />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <ProtectionStatus enabled={props.enabled} compact={false} />
        {props.canManage ? (
          <ProtectionAction
            enabled={props.enabled}
            compact={false}
            open={props.open}
          />
        ) : null}
      </div>
      <LoadErrorAlert error={props.loadError} />
      {props.modal}
    </div>
  );
}

function useProtectionControl(props: FamilyProtectionControlProps) {
  const { studentId, canManage, initialEnabled, onChanged } = props;
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
  const {
    save,
    saveError,
    fieldError,
    clearError,
    formRef,
    latestSaveRef,
    saving,
  } = useProtectionSave(studentId, enabled, onSaved);
  latestSaveRef.current = () => void save(reason);
  const modal = enabled !== null && (
    <ProtectionModal
      studentId={studentId}
      enabled={enabled}
      open={modalOpen}
      saving={saving}
      error={saveError}
      fieldError={fieldError}
      formRef={formRef}
      reason={reason}
      setReason={setReason}
      close={() => {
        clearError();
        setModalOpen(false);
      }}
      save={() => void save(reason)}
    />
  );
  const viewProps = {
    enabled,
    canManage,
    loadError,
    modal,
    open: () => setModalOpen(true),
  };
  return viewProps;
}

export function FamilyProtectionControl(props: FamilyProtectionControlProps) {
  const viewProps = useProtectionControl(props);
  return props.compact ? (
    <CompactProtectionView {...viewProps} />
  ) : (
    <FullProtectionView {...viewProps} />
  );
}
