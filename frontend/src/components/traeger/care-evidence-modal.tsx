"use client";

// Einen Nachweis eines Vertrags eintragen oder ändern (#4685): die
// Einkommensstufe oder „alleinerziehend“, mit Art des Nachweises, Prüfdatum
// und Gültigkeit. moto speichert kein Dokument und kein Einkommen.

import { useEffect, useRef, useState } from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { Input } from "~/components/ui/input";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { berlinTodayISO } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type CareEvidence,
  type CareEvidenceContract,
  type CareEvidenceKind,
  type CareEvidenceSet,
  saveCareEvidence,
} from "~/lib/traeger-care-reductions-api";
import { ruleEffect } from "./care-reduction-format";

const logger = createLogger({ component: "CareEvidenceModal" });

export const EVIDENCE_KIND_LABELS: Record<CareEvidenceKind, string> = {
  income_level: "Einkommensstufe",
  single_parent: "Alleinerziehend",
};

const EVIDENCE_PLACEHOLDER: Record<CareEvidenceKind, string> = {
  income_level: "z. B. Einkommensteuerbescheid 2025",
  single_parent: "z. B. Erklärung der Mutter vom 01.09.2026",
};

interface Draft {
  ruleId: string;
  evidence: string;
  checkedOn: string;
  validFrom: string;
  validUntil: string;
}

export interface EvidenceTarget {
  readonly contract: CareEvidenceContract;
  readonly kind: CareEvidenceKind;
  readonly evidence: CareEvidence | null;
}

export function CareEvidenceModal({
  data,
  target,
  onClose,
  onSaved,
}: Readonly<{
  data: CareEvidenceSet;
  /** The contract, kind and evidence to change; null keeps it closed. */
  target: EvidenceTarget | null;
  onClose: () => void;
  onSaved: (data: CareEvidenceSet) => void;
}>) {
  const today = berlinTodayISO();
  const [draft, setDraft] = useState<Draft>({
    ruleId: "",
    evidence: "",
    checkedOn: today,
    validFrom: today,
    validUntil: "",
  });
  const save = useFormModalPending(target !== null);
  const saving = save.pending;
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!target) return;
    const evidence = target.evidence;
    setDraft({
      ruleId: evidence?.ruleId ?? "",
      evidence: evidence?.evidence ?? "",
      checkedOn: evidence?.checkedOn ?? today,
      validFrom: evidence?.validFrom ?? today,
      validUntil: evidence?.validUntil ?? "",
    });
    clearErrors();
  }, [target, today, clearErrors]);

  const set = (patch: Partial<Draft>) =>
    setDraft((current) => ({ ...current, ...patch }));
  const income = target?.kind === "income_level";
  const child = target
    ? `${target.contract.childFirstName} ${target.contract.childLastName}`
    : "";

  const submit = async () => {
    if (!target) return;
    formErrors.clear();
    const fieldErrors: Record<string, string> = {};
    if (income && !draft.ruleId) {
      fieldErrors.rule_id = "Bitte eine Stufe wählen.";
    }
    if (!draft.evidence.trim()) {
      fieldErrors.evidence = "Bitte den Nachweis nennen.";
    }
    if (!draft.validFrom) fieldErrors.valid_from = "Bitte ein Datum wählen.";
    if (draft.validUntil && draft.validUntil < draft.validFrom) {
      fieldErrors.valid_until = "Das Ende liegt vor dem Beginn.";
    }
    if (!draft.checkedOn) fieldErrors.checked_on = "Bitte ein Datum wählen.";
    if (Object.keys(fieldErrors).length > 0) {
      formErrors.invalid(
        "Bitte prüfen Sie die markierten Felder.",
        fieldErrors,
      );
      return;
    }
    await save.run(async (op) => {
      try {
        const result = await saveCareEvidence(target.evidence?.id ?? null, {
          contractId: target.contract.contractId,
          kind: target.kind,
          ruleId: income ? draft.ruleId : null,
          evidence: draft.evidence.trim(),
          checkedOn: draft.checkedOn,
          validFrom: draft.validFrom,
          validUntil: draft.validUntil || null,
        });
        if (op.isCurrent()) onSaved(result);
        success(`Der Nachweis für ${child} ist gespeichert.`);
      } catch (err) {
        logger.error("care_evidence_save_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!op.isCurrent()) return;
        void formErrors.show(err, { object: "die Angabe zum Nachweis" });
      }
    });
  };

  return (
    <FormModal
      isOpen={target !== null}
      onClose={onClose}
      pending={save}
      isBackdropDismissDisabled
      title={
        target ? `${EVIDENCE_KIND_LABELS[target.kind]}: ${child}` : "Nachweis"
      }
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Nachweis speichern"
        />
      }
    >
      <form
        ref={formRef}
        noValidate
        className="space-y-5"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        {income ? (
          <CustomSelect
            id="care-evidence-level"
            name="rule_id"
            label="Stufe"
            required
            value={draft.ruleId}
            options={data.levels
              .filter((level) =>
                level.schoolIds.includes(target?.contract.schoolId ?? ""),
              )
              .map((level) => ({
                value: level.id,
                label: `${level.name} (${ruleEffect(level)})`,
              }))}
            error={formErrors.fieldError("rule_id")}
            onChange={(value) => set({ ruleId: value })}
          />
        ) : null}
        <Input
          controlSize="touch"
          id="care-evidence-proof"
          name="evidence"
          label="Geprüfter Nachweis"
          required
          maxLength={120}
          placeholder={target ? EVIDENCE_PLACEHOLDER[target.kind] : ""}
          hint="Nur die Art des Nachweises. Das Dokument bleibt bei Ihnen."
          value={draft.evidence}
          error={formErrors.fieldError("evidence")}
          onChange={(event) => set({ evidence: event.target.value })}
        />
        <ISODatePicker
          id="care-evidence-checked"
          label="Geprüft am"
          value={draft.checkedOn}
          max={today}
          error={formErrors.fieldError("checked_on")}
          onChange={(value) => set({ checkedOn: value })}
        />
        <div className="grid gap-3 sm:grid-cols-2">
          <ISODatePicker
            id="care-evidence-from"
            label="Gilt ab"
            value={draft.validFrom}
            error={formErrors.fieldError("valid_from")}
            onChange={(value) => set({ validFrom: value })}
          />
          <ISODatePicker
            id="care-evidence-until"
            label="Gilt bis"
            value={draft.validUntil}
            min={draft.validFrom}
            placeholder="Ohne Ende"
            error={formErrors.fieldError("valid_until")}
            onChange={(value) => set({ validUntil: value })}
          />
        </div>
      </form>
    </FormModal>
  );
}
