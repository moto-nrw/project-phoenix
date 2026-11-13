"use client";

// Einen Kostenträger-Anteil eines Vertrags eintragen oder ändern (#4687):
// Art, Kostenträger, wofür er zahlt (Betreuung, Mensa oder beides, #4686),
// Anteil in Prozent oder als fester Betrag im Monat, Gültigkeit mit Ende, geprüft am und ein Verweis auf den Bescheid. moto
// speichert kein Dokument und nichts aus dem Bescheid.

import { useEffect, useRef, useState } from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel } from "~/components/ui/field-label";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { Input } from "~/components/ui/input";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { berlinTodayISO } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import type {
  CareCostShareKind,
  CareCostShareScope,
} from "~/lib/traeger-care-billing-api";
import {
  type CareCostShare,
  type CareCostShareContract,
  type CareCostShareSet,
  saveCareCostShare,
} from "~/lib/traeger-care-cost-shares-api";
import {
  COST_SHARE_KIND_LABELS,
  COST_SHARE_KINDS,
  COST_SHARE_SCOPE_LABELS,
} from "./care-cost-share-format";
import { centsToEuroInput, euroInputToCents } from "./holiday-period-model";

const logger = createLogger({ component: "CareCostShareModal" });

type ShareMode = "percent" | "amount";

interface Draft {
  kind: CareCostShareKind | null;
  costBearerId: string;
  scope: CareCostShareScope;
  /** Set once the person picks the scope; until then the kind suggests it. */
  scopeChosen: boolean;
  mode: ShareMode;
  value: string;
  reference: string;
  checkedOn: string;
  validFrom: string;
  validUntil: string;
}

export interface CostShareTarget {
  readonly contract: CareCostShareContract;
  readonly share: CareCostShare | null;
}

const COST_SHARE_SCOPES: readonly CareCostShareScope[] = [
  "care",
  "meal",
  "both",
];

/** Bildung und Teilhabe übernimmt meist das Mittagessen, die anderen die Betreuung. */
const SUGGESTED_SCOPE: Record<CareCostShareKind, CareCostShareScope> = {
  education_participation: "meal",
  youth_office: "care",
  other_subsidy: "care",
};

const REFERENCE_PLACEHOLDER: Record<CareCostShareKind, string> = {
  education_participation: "z. B. BuT-Bescheid Az. 51-2026-117",
  youth_office: "z. B. Bescheid Jugendamt Az. 4-12-881",
  other_subsidy: "z. B. Zusage der Stiftung vom 01.09.2026",
};

const SCOPE_PERCENT_HINT: Record<CareCostShareScope, string> = {
  care: "100 % heißt: Die Familie zahlt nichts für die Betreuung. Der Anteil gilt nach allen Ermäßigungen.",
  meal: "100 % heißt: Die Familie zahlt nichts für die Mensa.",
  both: "100 % heißt: Die Familie zahlt nichts. Bei der Betreuung gilt der Anteil nach allen Ermäßigungen.",
};

const SCOPE_AMOUNT_HINT: Record<CareCostShareScope, string> = {
  care: "Höchstens so viel, wie die Betreuung nach den Ermäßigungen kostet.",
  meal: "Höchstens so viel, wie die Mensa im Monat kostet.",
  both: "Erst für die Betreuung, der Rest für die Mensa.",
};

/** "12,5" → 1250 basis points, or null. */
function percentToBasisPoints(value: string): number | null {
  const match = /^(\d{1,3})(?:[.,](\d{1,2}))?$/.exec(value.trim());
  if (!match) return null;
  const points =
    Number(match[1]) * 100 + Number((match[2] ?? "").padEnd(2, "0"));
  return points >= 1 && points <= 10000 ? points : null;
}

function basisPointsToInput(points: number): string {
  return (points / 100).toLocaleString("de-DE", { maximumFractionDigits: 2 });
}

/** The first day of the month of a "YYYY-MM-DD" day. */
function monthStart(day: string): string {
  return `${day.slice(0, 8)}01`;
}

/** The 31 July that ends the school year a day falls in. */
function schoolYearEnd(day: string): string {
  const year = Number(day.slice(0, 4));
  const month = Number(day.slice(5, 7));
  return `${month >= 8 ? year + 1 : year}-07-31`;
}

function draftOf(target: CostShareTarget, today: string): Draft {
  const share = target.share;
  if (!share) {
    const from = monthStart(today);
    return {
      kind: null,
      costBearerId: "",
      scope: "care",
      scopeChosen: false,
      mode: "percent",
      value: "100",
      reference: "",
      checkedOn: today,
      validFrom: from,
      validUntil: schoolYearEnd(from),
    };
  }
  return {
    kind: share.kind,
    costBearerId: share.costBearerId,
    scope: share.scope,
    scopeChosen: true,
    mode: share.shareBasisPoints === null ? "amount" : "percent",
    value:
      share.shareBasisPoints === null
        ? centsToEuroInput(share.shareCents ?? 0)
        : basisPointsToInput(share.shareBasisPoints),
    reference: share.reference,
    checkedOn: share.checkedOn,
    validFrom: share.validFrom,
    validUntil: share.validUntil,
  };
}

export function CareCostShareModal({
  data,
  target,
  onClose,
  onSaved,
}: Readonly<{
  data: CareCostShareSet;
  /** The contract and share to change; null keeps it closed. */
  target: CostShareTarget | null;
  onClose: () => void;
  onSaved: (data: CareCostShareSet) => void;
}>) {
  const today = berlinTodayISO();
  const [draft, setDraft] = useState<Draft | null>(null);
  const save = useFormModalPending(target !== null);
  const saving = save.pending;
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!target) return;
    setDraft(draftOf(target, today));
    clearErrors();
  }, [target, today, clearErrors]);

  const set = (patch: Partial<Draft>) =>
    setDraft((current) => (current ? { ...current, ...patch } : current));
  const child = target
    ? `${target.contract.childFirstName} ${target.contract.childLastName}`
    : "";
  // A switched-off cost bearer stays on the share that names it, but is
  // offered for no new one.
  const bearers = data.costBearers.filter(
    (bearer) => bearer.active || bearer.id === target?.share?.costBearerId,
  );

  const submit = async () => {
    if (!target || !draft) return;
    formErrors.clear();
    const fieldErrors: Record<string, string> = {};
    if (!draft.kind) fieldErrors.kind = "Bitte die Art wählen.";
    if (!draft.costBearerId) {
      fieldErrors.cost_bearer_id = "Bitte den Kostenträger wählen.";
    }
    const basisPoints =
      draft.mode === "percent" ? percentToBasisPoints(draft.value) : null;
    const cents =
      draft.mode === "amount" ? euroInputToCents(draft.value) : null;
    if (draft.mode === "percent" && basisPoints === null) {
      fieldErrors.share_basis_points =
        "Bitte einen Anteil zwischen 0,01 und 100 % eintragen.";
    }
    if (draft.mode === "amount" && (cents === null || cents < 1)) {
      fieldErrors.share_cents = "Bitte einen Betrag in Euro eintragen.";
    }
    if (!draft.reference.trim()) {
      fieldErrors.reference = "Bitte den Bescheid nennen.";
    }
    if (!draft.validFrom) fieldErrors.valid_from = "Bitte ein Datum wählen.";
    if (!draft.validUntil) {
      fieldErrors.valid_until = "Bitte das Ende des Bescheids wählen.";
    } else if (draft.validUntil < draft.validFrom) {
      fieldErrors.valid_until = "Das Ende liegt vor dem Beginn.";
    }
    if (!draft.checkedOn) fieldErrors.checked_on = "Bitte ein Datum wählen.";
    if (Object.keys(fieldErrors).length > 0 || !draft.kind) {
      formErrors.invalid(
        "Bitte prüfen Sie die markierten Felder.",
        fieldErrors,
      );
      return;
    }
    const kind = draft.kind;
    await save.run(async (op) => {
      try {
        const result = await saveCareCostShare(target.share?.id ?? null, {
          contractId: target.contract.contractId,
          kind,
          costBearerId: draft.costBearerId,
          shareBasisPoints: basisPoints,
          shareCents: cents,
          scope: draft.scope,
          validFrom: draft.validFrom,
          validUntil: draft.validUntil,
          reference: draft.reference.trim(),
          checkedOn: draft.checkedOn,
        });
        if (op.isCurrent()) onSaved(result);
        success(`Der Anteil für ${child} ist gespeichert.`);
      } catch (err) {
        logger.error("care_cost_share_save_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!op.isCurrent()) return;
        void formErrors.show(err, { object: "die Kostenübernahme" });
      }
    });
  };

  const valueField =
    draft?.mode === "amount" ? "share_cents" : "share_basis_points";

  return (
    <FormModal
      isOpen={target !== null}
      onClose={onClose}
      pending={save}
      isBackdropDismissDisabled
      title={
        target?.share
          ? `Anteil ändern: ${child}`
          : target
            ? `Anteil eintragen: ${child}`
            : "Kostenträger-Anteil"
      }
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Anteil speichern"
        />
      }
    >
      {draft ? (
        <form
          ref={formRef}
          noValidate
          className="space-y-5"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <div
            className="space-y-2"
            role="group"
            aria-labelledby="care-cost-share-kind-label"
            aria-describedby={
              formErrors.fieldError("kind")
                ? "care-cost-share-kind-error"
                : undefined
            }
          >
            <FieldLabel id="care-cost-share-kind-label" required>
              Wer zahlt mit?
            </FieldLabel>
            <SegmentedControl
              ariaLabel="Art des Anteils"
              fullWidth
              items={COST_SHARE_KINDS.map((kind) => ({
                value: kind,
                label: COST_SHARE_KIND_LABELS[kind],
              }))}
              value={draft.kind}
              onChange={(kind) =>
                set(
                  draft.scopeChosen
                    ? { kind }
                    : { kind, scope: SUGGESTED_SCOPE[kind] },
                )
              }
            />
            <FieldErrorHint
              id="care-cost-share-kind-error"
              message={formErrors.fieldError("kind")}
            />
          </div>
          <CustomSelect
            id="care-cost-share-bearer"
            name="cost_bearer_id"
            label="Kostenträger"
            required
            value={draft.costBearerId}
            options={bearers.map((bearer) => ({
              value: bearer.id,
              label: bearer.active ? bearer.name : `${bearer.name} (aus)`,
            }))}
            error={formErrors.fieldError("cost_bearer_id")}
            onChange={(value) => set({ costBearerId: value })}
          />
          <div
            className="space-y-2"
            role="group"
            aria-labelledby="care-cost-share-scope-label"
            aria-describedby={
              formErrors.fieldError("scope")
                ? "care-cost-share-scope-error"
                : undefined
            }
          >
            <FieldLabel id="care-cost-share-scope-label" required>
              Wofür zahlt er?
            </FieldLabel>
            <SegmentedControl
              ariaLabel="Wofür der Kostenträger zahlt"
              fullWidth
              items={COST_SHARE_SCOPES.map((scope) => ({
                value: scope,
                label: COST_SHARE_SCOPE_LABELS[scope],
              }))}
              value={draft.scope}
              onChange={(scope) => set({ scope, scopeChosen: true })}
            />
            <FieldErrorHint
              id="care-cost-share-scope-error"
              message={formErrors.fieldError("scope")}
            />
          </div>
          <div className="space-y-2">
            <FieldLabel required>Anteil</FieldLabel>
            <SegmentedControl
              ariaLabel="Anteil als Prozent oder fester Betrag"
              fullWidth
              items={[
                { value: "percent", label: "Prozent vom Beitrag" },
                { value: "amount", label: "Fester Betrag im Monat" },
              ]}
              value={draft.mode}
              onChange={(mode) =>
                set({ mode, value: mode === "percent" ? "100" : "" })
              }
            />
            <Input
              controlSize="touch"
              id="care-cost-share-value"
              name={valueField}
              aria-label={
                draft.mode === "percent"
                  ? "Anteil in Prozent"
                  : "Betrag in Euro"
              }
              inputMode="decimal"
              value={draft.value}
              hint={
                draft.mode === "percent"
                  ? SCOPE_PERCENT_HINT[draft.scope]
                  : SCOPE_AMOUNT_HINT[draft.scope]
              }
              error={formErrors.fieldError(valueField)}
              onChange={(event) => set({ value: event.target.value })}
            />
          </div>
          <Input
            controlSize="touch"
            id="care-cost-share-reference"
            name="reference"
            label="Bescheid"
            required
            maxLength={120}
            placeholder={draft.kind ? REFERENCE_PLACEHOLDER[draft.kind] : ""}
            hint="Nur Aktenzeichen oder Datum. Den Bescheid behalten Sie, moto speichert nichts daraus."
            value={draft.reference}
            error={formErrors.fieldError("reference")}
            onChange={(event) => set({ reference: event.target.value })}
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <ISODatePicker
              id="care-cost-share-from"
              label="Gilt ab"
              value={draft.validFrom}
              error={formErrors.fieldError("valid_from")}
              onChange={(value) => set({ validFrom: value })}
            />
            <ISODatePicker
              id="care-cost-share-until"
              label="Gilt bis"
              value={draft.validUntil}
              min={draft.validFrom}
              error={formErrors.fieldError("valid_until")}
              onChange={(value) => set({ validUntil: value })}
            />
          </div>
          <ISODatePicker
            id="care-cost-share-checked"
            label="Bescheid geprüft am"
            value={draft.checkedOn}
            max={today}
            error={formErrors.fieldError("checked_on")}
            onChange={(value) => set({ checkedOn: value })}
          />
        </form>
      ) : null}
    </FormModal>
  );
}
