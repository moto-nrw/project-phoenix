"use client";

// Die Kinderzahl einer Familie bestätigen oder ändern (#5918). Die Zahl
// zählt alle Kinder der Familie, auch Geschwister ohne Betreuungsvertrag.
// moto speichert nur die Zahl, keine Namen dieser Geschwister.

import { useEffect, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { ISODatePicker } from "~/components/ui/date-picker";
import { DataField, DataGrid } from "~/components/ui/data-field";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldLabel } from "~/components/ui/field-label";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { InsetPanel } from "~/components/ui/inset-panel";
import { NumberStepper } from "~/components/ui/number-stepper";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { berlinTodayISO } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  confirmFamilyChildren,
  type FamilyChildren,
  type FamilyChildrenSet,
} from "~/lib/traeger-family-children-api";
import {
  childrenLabel,
  familyName,
  statedOn,
  validityLabel,
} from "./family-children-format";

const logger = createLogger({ component: "FamilyChildrenModal" });

const MAX_CHILDREN = 20;

interface Draft {
  children: number;
  validFrom: string;
  validUntil: string;
}

/** The parents' number when it is newer than the confirmation, else the confirmed one. */
function startingNumber(family: FamilyChildren): number {
  const count = family.count;
  const contracts = Math.max(family.children.length, 1);
  if (!count) return contracts;
  const statedLater =
    count.statedAt !== null &&
    (count.confirmedAt === null || count.statedAt > count.confirmedAt);
  if (statedLater && count.statedChildren !== null) return count.statedChildren;
  return count.confirmedChildren ?? count.statedChildren ?? contracts;
}

export function FamilyChildrenModal({
  family,
  onClose,
  onSaved,
}: Readonly<{
  /** The family to check; null keeps the dialog closed. */
  family: FamilyChildren | null;
  onClose: () => void;
  onSaved: (data: FamilyChildrenSet) => void;
}>) {
  const today = berlinTodayISO();
  const monthStart = `${today.slice(0, 8)}01`;
  const [draft, setDraft] = useState<Draft>({
    children: 1,
    validFrom: monthStart,
    validUntil: "",
  });
  const save = useFormModalPending(family !== null);
  const saving = save.pending;
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!family) return;
    const count = family.count;
    const expired =
      family.status === "expired" || count?.validFrom === null || !count;
    setDraft({
      children: startingNumber(family),
      validFrom: expired ? monthStart : (count?.validFrom ?? monthStart),
      validUntil: expired ? "" : (count?.validUntil ?? ""),
    });
    clearErrors();
  }, [family, monthStart, clearErrors]);

  const set = (patch: Partial<Draft>) =>
    setDraft((current) => ({ ...current, ...patch }));
  const withContract = family?.children.length ?? 0;
  const name = family ? familyName(family) : "";

  const submit = async () => {
    if (!family) return;
    formErrors.clear();
    const fieldErrors: Record<string, string> = {};
    if (!draft.validFrom) fieldErrors.valid_from = "Bitte ein Datum wählen.";
    if (draft.validUntil && draft.validUntil < draft.validFrom) {
      fieldErrors.valid_until = "Das Ende liegt vor dem Beginn.";
    }
    if (Object.keys(fieldErrors).length > 0) {
      formErrors.invalid(
        "Bitte prüfen Sie die markierten Felder.",
        fieldErrors,
      );
      return;
    }
    const contract = family.count?.contractId ?? family.children[0]?.contractId;
    if (!contract) return;
    await save.run(async (op) => {
      try {
        const result = await confirmFamilyChildren({
          contractId: contract,
          children: draft.children,
          validFrom: draft.validFrom,
          validUntil: draft.validUntil || null,
        });
        if (op.isCurrent()) onSaved(result);
        success(`Die Kinderzahl von ${name} ist bestätigt.`);
      } catch (err) {
        logger.error("family_children_confirm_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!op.isCurrent()) return;
        void formErrors.show(err, { object: "die Kinderzahl" });
      }
    });
  };

  const count = family?.count ?? null;

  return (
    <FormModal
      isOpen={family !== null}
      onClose={onClose}
      pending={save}
      isBackdropDismissDisabled
      title={family ? `Kinderzahl: ${name}` : "Kinderzahl"}
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Kinderzahl bestätigen"
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
        {family ? (
          <InsetPanel>
            <DataGrid>
              <DataField label="Kinder mit Vertrag">
                {withContract === 0 ? (
                  "Noch kein wirksamer Vertrag"
                ) : (
                  <>
                    {childrenLabel(withContract)}
                    <span className="mt-0.5 block font-normal text-gray-600">
                      {family.children
                        .map((child) => child.childFirstName)
                        .join(", ")}
                    </span>
                  </>
                )}
              </DataField>
              <DataField label="Angabe der Eltern">
                {count?.statedChildren ? (
                  <>
                    {childrenLabel(count.statedChildren)}
                    <span className="mt-0.5 block font-normal text-gray-600">
                      am {statedOn(count)}
                    </span>
                  </>
                ) : (
                  "Noch keine Angabe"
                )}
              </DataField>
              {count?.confirmedChildren ? (
                <DataField label="Bisher bestätigt">
                  {childrenLabel(count.confirmedChildren)}
                  <span className="mt-0.5 block font-normal text-gray-600">
                    {validityLabel(count)}
                  </span>
                </DataField>
              ) : null}
            </DataGrid>
          </InsetPanel>
        ) : null}
        <div>
          <FieldLabel htmlFor="family-children-count" required>
            Kinder in der Familie
          </FieldLabel>
          <NumberStepper
            id="family-children-count"
            name="children"
            value={String(draft.children)}
            onChange={(event) => {
              const next = Number(event.target.value);
              if (Number.isInteger(next) && next >= 1 && next <= MAX_CHILDREN)
                set({ children: next });
            }}
            onStep={(delta) =>
              set({
                children: Math.min(
                  MAX_CHILDREN,
                  Math.max(1, draft.children + delta),
                ),
              })
            }
            canDecrement={draft.children > 1}
            canIncrement={draft.children < MAX_CHILDREN}
            decrementLabel="Ein Kind weniger"
            incrementLabel="Ein Kind mehr"
            error={formErrors.fieldError("children")}
          />
          <p className="mt-1 text-sm text-gray-600">
            Alle Kinder der Familie, auch ohne Betreuungsvertrag.
          </p>
        </div>
        {draft.children < withContract ? (
          <Alert
            type="info"
            size="compact"
            message={`Die Familie hat ${childrenLabel(withContract)} mit Vertrag. moto zählt mindestens diese.`}
          />
        ) : null}
        <div className="grid gap-3 sm:grid-cols-2">
          <ISODatePicker
            id="family-children-from"
            label="Gilt ab"
            value={draft.validFrom}
            error={formErrors.fieldError("valid_from")}
            onChange={(value) => set({ validFrom: value })}
          />
          <ISODatePicker
            id="family-children-until"
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
