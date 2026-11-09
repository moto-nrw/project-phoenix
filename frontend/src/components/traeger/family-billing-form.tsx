"use client";

// Empfänger und Zahlung einer Familie (#4689): wer die Rechnung bekommt
// (die Person im Vertrag, beide Eltern oder eine andere Person mit eigener
// Anschrift), ein eigener Zahlungsweg und ein eigener Zahltag. Die Familie
// ist die E-Mail-Adresse der Person im Vertrag; die Abweichung gilt für alle
// ihre Rechnungen ab dem nächsten Lauf. Weicht nichts mehr ab, ist sie
// entfernt.

import { useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldLabel } from "~/components/ui/field-label";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { Radio } from "~/components/ui/radio";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type FamilyBilling,
  type FamilyBillingInput,
  MAX_DUE_DAY,
  PAYMENT_METHODS,
  type PaymentMethod,
  type RecipientKind,
  saveFamilyBilling,
} from "~/lib/traeger-billing-terms-api";
import { PAYMENT_METHOD_LABELS } from "./billing-payment-format";

const logger = createLogger({ component: "FamilyBillingForm" });

/** The contract whose family the dialog sets. */
export interface FamilyBillingTarget {
  readonly contractId: string;
  /** The person in the contract, e.g. "Anna Muster". */
  readonly applicantName: string;
  /** The child of the invoice the dialog was opened from. */
  readonly childName: string;
  /** The invoice is issued: the change reaches only the next run. */
  readonly issued: boolean;
}

export interface FamilyBillingDraft {
  recipientKind: RecipientKind;
  secondFirstName: string;
  secondLastName: string;
  payerFirstName: string;
  payerLastName: string;
  payerEmail: string;
  street: string;
  postalCode: string;
  city: string;
  paymentMethod: PaymentMethod | "";
  dueDay: string;
}

export const EMPTY_FAMILY_BILLING: FamilyBillingDraft = {
  recipientKind: "applicant",
  secondFirstName: "",
  secondLastName: "",
  payerFirstName: "",
  payerLastName: "",
  payerEmail: "",
  street: "",
  postalCode: "",
  city: "",
  paymentMethod: "",
  dueDay: "",
};

function draftOf(existing: FamilyBilling | null): FamilyBillingDraft {
  if (!existing) return EMPTY_FAMILY_BILLING;
  return {
    recipientKind: existing.recipientKind,
    secondFirstName: existing.secondFirstName,
    secondLastName: existing.secondLastName,
    payerFirstName: existing.payerFirstName,
    payerLastName: existing.payerLastName,
    payerEmail: existing.payerEmail,
    street: existing.payerAddress?.street ?? "",
    postalCode: existing.payerAddress?.postalCode ?? "",
    city: existing.payerAddress?.city ?? "",
    paymentMethod: existing.paymentMethod,
    dueDay: existing.dueDay === null ? "" : String(existing.dueDay),
  };
}

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/**
 * The field hints of a draft the backend would refuse, keyed like the
 * backend's `errors[]`; empty when the draft may be sent.
 */
export function familyBillingErrors(
  draft: FamilyBillingDraft,
): Record<string, string> {
  const errors: Record<string, string> = {};
  const need = (field: string, value: string, hint: string) => {
    if (!value.trim()) errors[field] = hint;
  };
  if (draft.recipientKind === "both") {
    need(
      "second_first_name",
      draft.secondFirstName,
      "Bitte den Vornamen eingeben.",
    );
    need(
      "second_last_name",
      draft.secondLastName,
      "Bitte den Nachnamen eingeben.",
    );
  }
  if (draft.recipientKind === "payer") {
    need(
      "payer_first_name",
      draft.payerFirstName,
      "Bitte den Vornamen eingeben.",
    );
    need(
      "payer_last_name",
      draft.payerLastName,
      "Bitte den Nachnamen eingeben.",
    );
    if (!EMAIL_PATTERN.test(draft.payerEmail.trim())) {
      errors.payer_email =
        "Bitte eine E-Mail-Adresse wie name@beispiel.de eingeben.";
    }
    need(
      "payer_address.street",
      draft.street,
      "Bitte Straße und Hausnummer eingeben.",
    );
    if (!/^\d{5}$/.test(draft.postalCode.trim())) {
      errors["payer_address.postal_code"] =
        "Bitte eine Postleitzahl mit 5 Ziffern eingeben.";
    }
    need("payer_address.city", draft.city, "Bitte den Ort eingeben.");
  }
  return errors;
}

/** The request of a draft for the contract. */
export function familyBillingInput(
  contractId: string,
  draft: FamilyBillingDraft,
): FamilyBillingInput {
  return {
    contractId,
    recipientKind: draft.recipientKind,
    secondFirstName: draft.secondFirstName,
    secondLastName: draft.secondLastName,
    payerFirstName: draft.payerFirstName,
    payerLastName: draft.payerLastName,
    payerEmail: draft.payerEmail,
    payerAddress:
      draft.recipientKind === "payer"
        ? {
            street: draft.street,
            postalCode: draft.postalCode,
            city: draft.city,
          }
        : null,
    paymentMethod: draft.paymentMethod,
    dueDay: draft.dueDay === "" ? null : Number(draft.dueDay),
  };
}

const RECIPIENT_ITEMS: readonly { value: RecipientKind; label: string }[] = [
  { value: "applicant", label: "Person im Vertrag" },
  { value: "both", label: "Beide Eltern" },
  { value: "payer", label: "Andere Person" },
];

const DUE_DAY_OPTIONS = [
  { value: "", label: "Wie im Lauf" },
  ...Array.from({ length: MAX_DUE_DAY }, (_, index) => ({
    value: String(index + 1),
    label: `Am ${index + 1}. des Monats`,
  })),
];

/** The form of a family's recipient and payment, on its own page. */
export function FamilyBillingForm({
  target,
  existing,
  runDueText,
  onCancel,
  onSaved,
}: Readonly<{
  /** The contract whose family is set. */
  target: FamilyBillingTarget;
  /** The family's deviation as stored, null without one. */
  existing: FamilyBilling | null;
  /**
   * The run's due dates as the run page names them („Überweisung am …“),
   * so „Wie im Lauf“ says which dates it takes. Missing outside a run.
   */
  runDueText?: string | null;
  onCancel: () => void;
  onSaved: (family: FamilyBilling | null) => void;
}>) {
  const [draft, setDraft] = useState<FamilyBillingDraft>(() =>
    draftOf(existing),
  );
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { success } = useToast();

  const set = (patch: Partial<FamilyBillingDraft>) =>
    setDraft((current) => ({ ...current, ...patch }));
  const fieldError = (name: string) => formErrors.fieldError(name) ?? undefined;

  const submit = async () => {
    formErrors.clear();
    const errors = familyBillingErrors(draft);
    if (Object.keys(errors).length > 0) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", errors);
      return;
    }
    setSaving(true);
    try {
      const saved = await saveFamilyBilling(
        familyBillingInput(target.contractId, draft),
      );
      success(
        saved
          ? "Empfänger und Zahlung sind gespeichert."
          : "Die Familie hat wieder die Angaben aus dem Vertrag.",
      );
      onSaved(saved);
    } catch (err) {
      logger.error("family_billing_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, {
        object: "die Angabe zu Empfänger und Zahlung",
      });
    } finally {
      setSaving(false);
    }
  };

  const id = (field: string) => `family-billing-${field}`;
  const deviates =
    draft.recipientKind !== "applicant" ||
    draft.paymentMethod !== "" ||
    draft.dueDay !== "";

  return (
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
        {target.issued
          ? "Gilt für alle Rechnungen dieser Familie ab dem nächsten Lauf. Diese Rechnung ist schon ausgestellt und bleibt, wie sie ist."
          : "Gilt für alle Rechnungen dieser Familie, auch für Geschwister. Die Vorschau zeigt die Änderung gleich."}
      </p>
      <div className="space-y-2">
        <FieldLabel id={id("recipient-label")}>Rechnung an</FieldLabel>
        <div
          role="radiogroup"
          aria-labelledby={id("recipient-label")}
          className="grid gap-2 sm:grid-cols-3"
        >
          {RECIPIENT_ITEMS.map((item) => (
            <ChoiceTile
              key={item.value}
              selected={draft.recipientKind === item.value}
            >
              <Radio
                name={id("recipient")}
                value={item.value}
                checked={draft.recipientKind === item.value}
                onChange={() => set({ recipientKind: item.value })}
              />
              {item.label}
            </ChoiceTile>
          ))}
        </div>
        <p className="text-sm text-gray-600">
          {draft.recipientKind === "applicant"
            ? `Die Rechnung geht an ${target?.applicantName || "die Person im Vertrag"}.`
            : draft.recipientKind === "both"
              ? "Beide Namen stehen auf der Rechnung. Sie geht an die E-Mail-Adresse aus dem Vertrag."
              : "Die Rechnung geht an diese Person, mit ihrer Anschrift und E-Mail-Adresse."}
        </p>
      </div>
      {draft.recipientKind === "both" ? (
        <div className="grid gap-3 sm:grid-cols-2">
          <Input
            controlSize="touch"
            id={id("second-first")}
            name="second_first_name"
            label="Vorname des zweiten Elternteils"
            required
            maxLength={100}
            value={draft.secondFirstName}
            error={fieldError("second_first_name")}
            onChange={(event) => set({ secondFirstName: event.target.value })}
          />
          <Input
            controlSize="touch"
            id={id("second-last")}
            name="second_last_name"
            label="Nachname des zweiten Elternteils"
            required
            maxLength={100}
            value={draft.secondLastName}
            error={fieldError("second_last_name")}
            onChange={(event) => set({ secondLastName: event.target.value })}
          />
        </div>
      ) : null}
      {draft.recipientKind === "payer" ? (
        <div className="grid gap-3 sm:grid-cols-2">
          <Input
            controlSize="touch"
            id={id("payer-first")}
            name="payer_first_name"
            label="Vorname"
            required
            maxLength={100}
            value={draft.payerFirstName}
            error={fieldError("payer_first_name")}
            onChange={(event) => set({ payerFirstName: event.target.value })}
          />
          <Input
            controlSize="touch"
            id={id("payer-last")}
            name="payer_last_name"
            label="Nachname"
            required
            maxLength={100}
            value={draft.payerLastName}
            error={fieldError("payer_last_name")}
            onChange={(event) => set({ payerLastName: event.target.value })}
          />
          <div className="sm:col-span-2">
            <Input
              controlSize="touch"
              id={id("payer-email")}
              name="payer_email"
              type="email"
              label="E-Mail-Adresse"
              required
              maxLength={320}
              value={draft.payerEmail}
              error={fieldError("payer_email")}
              onChange={(event) => set({ payerEmail: event.target.value })}
            />
          </div>
          <div className="sm:col-span-2">
            <Input
              controlSize="touch"
              id={id("street")}
              name="payer_address.street"
              label="Straße und Hausnummer"
              required
              maxLength={100}
              value={draft.street}
              error={fieldError("payer_address.street")}
              onChange={(event) => set({ street: event.target.value })}
            />
          </div>
          <Input
            controlSize="touch"
            id={id("postal-code")}
            name="payer_address.postal_code"
            label="Postleitzahl"
            required
            inputMode="numeric"
            maxLength={5}
            value={draft.postalCode}
            error={fieldError("payer_address.postal_code")}
            onChange={(event) => set({ postalCode: event.target.value })}
          />
          <Input
            controlSize="touch"
            id={id("city")}
            name="payer_address.city"
            label="Ort"
            required
            maxLength={100}
            value={draft.city}
            error={fieldError("payer_address.city")}
            onChange={(event) => set({ city: event.target.value })}
          />
        </div>
      ) : null}
      <div className="space-y-2">
        <FieldLabel id={id("payment-label")}>Zahlungsweg</FieldLabel>
        <div
          role="radiogroup"
          aria-labelledby={id("payment-label")}
          className="grid gap-2 sm:grid-cols-3"
        >
          {[
            { value: "" as const, label: "Wie in den Abrechnungsvorgaben" },
            ...PAYMENT_METHODS.map((method) => ({
              value: method,
              label: PAYMENT_METHOD_LABELS[method],
            })),
          ].map((item) => (
            <ChoiceTile
              key={item.value}
              selected={draft.paymentMethod === item.value}
            >
              <Radio
                name={id("payment")}
                value={item.value}
                checked={draft.paymentMethod === item.value}
                onChange={() => set({ paymentMethod: item.value })}
              />
              {item.label}
            </ChoiceTile>
          ))}
        </div>
        {/* The precondition where the person chooses the debit. */}
        <p className="text-sm text-gray-600">
          Lastschrift geht nur mit gültigem Mandat. Den Stand sehen Sie unten
          unter „SEPA-Lastschriftmandat“.
        </p>
      </div>
      <CustomSelect
        id={id("due-day")}
        name="due_day"
        label="Zahltag"
        value={draft.dueDay}
        options={DUE_DAY_OPTIONS}
        hint={`Fällig ist der nächste gewählte Tag nach dem Rechnungsdatum. ${
          runDueText
            ? `„Wie im Lauf“: ${runDueText}.`
            : "„Wie im Lauf“ nimmt die Fälligkeit des Laufs."
        }`}
        error={fieldError("due_day")}
        onChange={(dueDay) => set({ dueDay })}
      />
      {deviates ? (
        <Button
          type="button"
          variant="outline"
          size="compact"
          className="self-start"
          onClick={() => setDraft(EMPTY_FAMILY_BILLING)}
        >
          Felder wie im Vertrag ausfüllen
        </Button>
      ) : null}
      <FormErrorAlert message={formErrors.error} />
      <EditActions onCancel={onCancel} saving={saving} />
    </form>
  );
}
