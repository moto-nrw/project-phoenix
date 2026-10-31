"use client";

import { useEffect, useRef, useState } from "react";
import { ISODateInput } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import {
  SegmentedControl,
  type SegmentedControlItem,
} from "~/components/ui/segmented-control";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { berlinTodayISO } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type ContributionRule,
  type ContributionRuleInput,
  type ContributionRuleKind,
  createContributionRule,
  updateContributionRule,
} from "~/lib/traeger-contributions-api";
import { centsToEuroInput, euroInputToCents } from "./holiday-period-model";

const logger = createLogger({ component: "ContributionRuleModal" });

/** Which rule the modal writes: a new one of a kind or an existing one. */
export type ContributionRuleEdit =
  | { readonly kind: "create"; readonly ruleKind: ContributionRuleKind }
  | { readonly kind: "change"; readonly rule: ContributionRule };

type DiscountMode = "share" | "amount";

interface Draft {
  readonly validFrom: string;
  readonly validUntil: string;
  readonly siblingFromChild: "2" | "3";
  readonly mode: DiscountMode;
  /** Percent or euro as the person types it. */
  readonly value: string;
  readonly note: string;
}

const CHILD_ITEMS: readonly SegmentedControlItem<"2" | "3">[] = [
  { value: "2", label: "ab dem 2. Kind" },
  { value: "3", label: "ab dem 3. Kind" },
];

const MODE_ITEMS: readonly SegmentedControlItem<DiscountMode>[] = [
  { value: "share", label: "Anteil in Prozent" },
  { value: "amount", label: "Betrag in Euro" },
];

function draftOf(edit: ContributionRuleEdit): Draft {
  if (edit.kind === "create") {
    return {
      validFrom: berlinTodayISO(),
      validUntil: "",
      siblingFromChild: "2",
      mode: "share",
      value: "",
      note: "",
    };
  }
  const { rule } = edit;
  return {
    validFrom: rule.validFrom,
    validUntil: rule.validUntil ?? "",
    siblingFromChild: rule.siblingFromChild === 3 ? "3" : "2",
    mode: rule.discountCents !== null ? "amount" : "share",
    value:
      rule.discountCents !== null
        ? centsToEuroInput(rule.discountCents)
        : rule.discountBasisPoints !== null
          ? String(rule.discountBasisPoints / 100).replace(".", ",")
          : "",
    note: rule.note,
  };
}

/** "50" or "33,33" → basis points; null when it is no share. */
function percentToBasisPoints(value: string): number | null {
  const match = /^(\d{1,3})(?:,(\d{1,2}))?$/.exec(value.trim());
  if (!match) return null;
  const points =
    Number(match[1]) * 100 + Number((match[2] ?? "").padEnd(2, "0"));
  return points >= 1 && points <= 10000 ? points : null;
}

/**
 * Adds or changes a contribution rule of the Träger (#4664). The backend
 * checks the rule as a whole and refuses a second rule of the same kind in
 * the same time.
 */
export function ContributionRuleModal({
  edit,
  onClose,
  onSaved,
}: Readonly<{
  edit: ContributionRuleEdit | null;
  onClose: () => void;
  onSaved: () => Promise<unknown>;
}>) {
  const [draft, setDraft] = useState<Draft | null>(null);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!edit) return;
    setDraft(draftOf(edit));
    clearErrors();
  }, [edit, clearErrors]);

  const ruleKind: ContributionRuleKind | null =
    edit?.kind === "create" ? edit.ruleKind : (edit?.rule.kind ?? null);
  const sibling = ruleKind === "sibling";

  const input = (current: Draft): ContributionRuleInput | null => {
    const fields: Record<string, string> = {};
    if (!current.validFrom)
      fields.valid_from = "Bitte geben Sie an, ab wann die Regel gilt.";
    let basisPoints: number | null = null;
    let cents: number | null = null;
    if (sibling) {
      if (current.mode === "share") {
        basisPoints = percentToBasisPoints(current.value);
        if (basisPoints === null)
          fields.discount_basis_points =
            "Bitte geben Sie einen Anteil zwischen 0,01 und 100 an.";
      } else {
        cents = euroInputToCents(current.value);
        if (cents === null || cents <= 0)
          fields.discount_cents = "Bitte geben Sie einen Betrag wie 10,00 an.";
      }
    } else if (!current.note.trim()) {
      fields.note = "Bitte geben Sie den Vermerk an.";
    }
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", fields);
      return null;
    }
    return {
      kind: sibling ? "sibling" : "benefits_exemption",
      validFrom: current.validFrom,
      validUntil: current.validUntil || null,
      siblingFromChild: sibling ? Number(current.siblingFromChild) : null,
      discountBasisPoints: basisPoints,
      discountCents: cents,
      note: sibling ? "" : current.note.trim(),
    };
  };

  const submit = async () => {
    if (!edit || !draft) return;
    formErrors.clear();
    const rule = input(draft);
    if (!rule) return;
    setSaving(true);
    try {
      if (edit.kind === "create") {
        await createContributionRule(rule);
        success("Die Regel ist gespeichert.");
      } else {
        await updateContributionRule(edit.rule.id, rule);
        success("Die Regel ist geändert.");
      }
      onClose();
      await onSaved();
    } catch (err) {
      logger.error("contribution_rule_save_failed", {
        kind: edit.kind,
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Regel" });
    } finally {
      setSaving(false);
    }
  };

  const set = (patch: Partial<Draft>) =>
    setDraft((current) => (current ? { ...current, ...patch } : current));
  const title = `${sibling ? "Geschwisterregel" : "Befreiung bei Leistungsbezug"} ${edit?.kind === "change" ? "ändern" : "hinzufügen"}`;

  return (
    <FormModal
      isOpen={edit !== null}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title={title}
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
        />
      }
    >
      {draft ? (
        <form
          ref={formRef}
          id="contribution-rule-form"
          noValidate
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <div className="grid gap-4 sm:grid-cols-2">
            <ISODateInput
              id="valid_from"
              label="Gilt ab"
              value={draft.validFrom}
              error={formErrors.fieldError("valid_from")}
              onChange={(validFrom) => set({ validFrom })}
            />
            <ISODateInput
              id="valid_until"
              label="Gilt bis (freiwillig)"
              value={draft.validUntil}
              error={formErrors.fieldError("valid_until")}
              onChange={(validUntil) => set({ validUntil })}
            />
          </div>
          {sibling ? (
            <>
              <div className="space-y-2">
                <p className="text-sm font-medium text-gray-700">
                  Für welche Kinder?
                </p>
                <SegmentedControl
                  items={CHILD_ITEMS}
                  value={draft.siblingFromChild}
                  onChange={(siblingFromChild) => set({ siblingFromChild })}
                  ariaLabel="Für welche Kinder"
                />
                <p className="text-sm text-gray-600">
                  Das Kind mit dem höchsten Preis ist das erste Kind.
                </p>
              </div>
              <div className="space-y-2">
                <p className="text-sm font-medium text-gray-700">
                  Wie viel weniger?
                </p>
                <SegmentedControl
                  items={MODE_ITEMS}
                  value={draft.mode}
                  onChange={(mode) => set({ mode, value: "" })}
                  ariaLabel="Wie viel weniger"
                />
                <Input
                  id="contribution-rule-value"
                  name={
                    draft.mode === "share"
                      ? "discount_basis_points"
                      : "discount_cents"
                  }
                  label={
                    draft.mode === "share"
                      ? "Anteil in Prozent"
                      : "Betrag in Euro je Kind"
                  }
                  inputMode="decimal"
                  placeholder={draft.mode === "share" ? "50" : "10,00"}
                  value={draft.value}
                  error={
                    formErrors.fieldError(
                      draft.mode === "share"
                        ? "discount_basis_points"
                        : "discount_cents",
                    ) ?? undefined
                  }
                  onChange={(event) => set({ value: event.target.value })}
                />
              </div>
            </>
          ) : (
            <Textarea
              id="contribution-rule-note"
              name="note"
              label="Vermerk auf dem Beitrag"
              placeholder="Die Familie bezieht Leistungen für Bildung und Teilhabe."
              value={draft.note}
              maxLength={300}
              error={formErrors.fieldError("note") ?? undefined}
              onChange={(event) => set({ note: event.target.value })}
            />
          )}
        </form>
      ) : null}
    </FormModal>
  );
}
