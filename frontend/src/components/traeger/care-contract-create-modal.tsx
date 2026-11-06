"use client";

// Verträge aus angenommenen Anmeldungen erstellen (#4678). Das Modal zeigt
// je Schule die Kinder mit gebuchtem Angebot, die noch keinen Vertrag haben.
// Ohne Vertragsbedingungen für das Angebot entsteht kein Entwurf; die Zeile
// sagt das und nennt den nächsten Schritt.

import { useEffect, useMemo, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { EditActions } from "~/components/ui/edit-actions";
import { EmptyState } from "~/components/ui/empty-state";
import { FormModal } from "~/components/ui/form-modal";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiFormError } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type ContractCandidate,
  type CreateResult,
  createCareContracts,
} from "~/lib/traeger-care-contracts-api";
import { childName, formatWeekdays } from "./care-contract-format";

const logger = createLogger({ component: "CareContractCreateModal" });

function candidateKey(candidate: ContractCandidate): string {
  return `${candidate.schoolId}:${candidate.requestChildId}:${candidate.careOfferingId}`;
}

interface SchoolGroup {
  readonly schoolName: string;
  readonly candidates: readonly ContractCandidate[];
}

function bySchool(candidates: readonly ContractCandidate[]): SchoolGroup[] {
  const groups = new Map<string, ContractCandidate[]>();
  for (const candidate of candidates) {
    const group = groups.get(candidate.schoolName) ?? [];
    group.push(candidate);
    groups.set(candidate.schoolName, group);
  }
  return [...groups].map(([schoolName, rows]) => ({
    schoolName,
    candidates: rows,
  }));
}

export function CareContractCreateModal({
  isOpen,
  candidates,
  onClose,
  onCreated,
  onOpenConditions,
}: Readonly<{
  isOpen: boolean;
  candidates: readonly ContractCandidate[];
  onClose: () => void;
  onCreated: (result: CreateResult) => void;
  /** Wechselt zum Reiter „Vertragsbedingungen“. */
  onOpenConditions: () => void;
}>) {
  const ready = useMemo(
    () => candidates.filter((candidate) => candidate.hasTemplate),
    [candidates],
  );
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;

  useEffect(() => {
    if (!isOpen) return;
    setSelected(new Set(ready.map(candidateKey)));
    clearErrors();
  }, [isOpen, ready, clearErrors]);

  const missing = candidates.length - ready.length;
  const toggle = (key: string) =>
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  const allSelected = ready.length > 0 && selected.size === ready.length;

  const submit = async () => {
    formErrors.clear();
    const keys = ready.filter((candidate) =>
      selected.has(candidateKey(candidate)),
    );
    if (keys.length === 0) {
      formErrors.invalid("Bitte wählen Sie mindestens ein Kind aus.", {});
      return;
    }
    setSaving(true);
    try {
      onCreated(await createCareContracts(keys));
    } catch (err) {
      logger.error("care_contract_create_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Verträge" });
    } finally {
      setSaving(false);
    }
  };

  const count = selected.size;
  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title="Verträge erstellen"
      size="lg"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          disabled={count === 0}
          saveLabel={
            count === 1 ? "1 Entwurf erstellen" : `${count} Entwürfe erstellen`
          }
          savingLabel="Wird erstellt …"
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
        <p className="text-sm text-gray-600">
          Diese Kinder sind angenommen und haben ein Angebot gebucht. moto
          übernimmt Kind, Eltern, Tage und Beginn aus der Anmeldung. Sie prüfen
          jeden Entwurf, bevor Sie ihn senden.
        </p>
        {candidates.length === 0 ? (
          <EmptyState
            title="Jede angenommene Anmeldung hat einen Vertrag."
            description="Neue Anmeldungen erscheinen hier, sobald die Schule sie annimmt."
          />
        ) : null}
        {missing > 0 ? (
          <Alert
            type="warning"
            message={
              missing === 1
                ? "Für 1 Kind fehlen die Vertragsbedingungen des Angebots. Legen Sie sie zuerst fest."
                : `Für ${missing} Kinder fehlen die Vertragsbedingungen des Angebots. Legen Sie sie zuerst fest.`
            }
            actionLayout="responsive"
            action={
              <Button
                type="button"
                variant="ghost_tone"
                size="compact"
                onClick={onOpenConditions}
              >
                Bedingungen festlegen
              </Button>
            }
          />
        ) : null}
        {ready.length > 1 ? (
          <label className="flex items-center gap-3 text-sm font-medium text-gray-900">
            <Checkbox
              checked={allSelected}
              onChange={() =>
                setSelected(
                  allSelected ? new Set() : new Set(ready.map(candidateKey)),
                )
              }
            />
            Alle {ready.length} auswählen
          </label>
        ) : null}
        {bySchool(candidates).map((group) => (
          <fieldset key={group.schoolName} className="space-y-2">
            <legend className="mb-2 text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {group.schoolName}
            </legend>
            {group.candidates.map((candidate) => {
              const key = candidateKey(candidate);
              const isSelected = selected.has(key);
              return (
                <ChoiceTile
                  key={key}
                  selected={isSelected && candidate.hasTemplate}
                  disabled={!candidate.hasTemplate}
                  tone="blue"
                  className="flex items-start gap-3 px-4 py-3"
                >
                  <Checkbox
                    checked={isSelected && candidate.hasTemplate}
                    disabled={!candidate.hasTemplate}
                    onChange={() => toggle(key)}
                    aria-label={`${childName(candidate)} auswählen`}
                  />
                  <span className="min-w-0 flex-1">
                    <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
                      <span className="font-medium text-gray-900">
                        {childName(candidate)}
                      </span>
                      {candidate.hasTemplate ? null : (
                        <StatusBadge
                          tone="orange"
                          label="Vertragsbedingungen fehlen"
                        />
                      )}
                    </span>
                    <span className="mt-0.5 block text-sm text-gray-600">
                      {candidate.offeringName} ·{" "}
                      {formatWeekdays(candidate.bookedDays)} · ab{" "}
                      {formatDate(candidate.effectiveFrom)}
                    </span>
                    <span className="block text-sm wrap-anywhere text-gray-500">
                      {candidate.guardianName}
                      {candidate.guardianEmail
                        ? ` · ${candidate.guardianEmail}`
                        : ""}
                    </span>
                  </span>
                </ChoiceTile>
              );
            })}
          </fieldset>
        ))}
      </form>
    </FormModal>
  );
}
