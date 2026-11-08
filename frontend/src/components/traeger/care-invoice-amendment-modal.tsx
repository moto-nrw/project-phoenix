"use client";

// Der Dialog für eine Änderung einer Rechnung in der Prüfphase (#6197):
// Betrag, Position, Empfänger, Anschrift, eine Position entfernen oder eine
// Änderung zurücknehmen. Jede Änderung braucht einen Grund. Die Seite der
// Rechnung lädt diese Datei, die Seite des Laufs nicht.

import { useEffect, useRef, useState } from "react";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { createLogger } from "~/lib/logger";
import {
  type CareAmendmentInput,
  amendCareInvoice,
} from "~/lib/traeger-care-amendments-api";
import type {
  CareAmendment,
  CareInvoice,
  CareRunDetail,
} from "~/lib/traeger-care-billing-api";
import { formatEuro } from "./contribution-format";
import { centsToEuroInput, euroInputToCents } from "./holiday-period-model";

const logger = createLogger({ component: "CareInvoiceAmendmentModal" });

/** The longest reason and label the backend keeps. */
const MAX_REASON = 500;
const MAX_LABEL = 200;

/** What the dialog changes: a new change, a removal or a revocation. */
export type CareAmendmentDraft =
  | { readonly kind: "amount" | "add_position" | "recipient" | "address" }
  | {
      readonly kind: "remove_position";
      readonly positionKey: string;
      readonly label: string;
      readonly amountCents: number;
    }
  | { readonly kind: "revoke"; readonly amendment: CareAmendment };

const TITLES: Record<CareAmendmentDraft["kind"], string> = {
  amount: "Betrag ändern",
  add_position: "Position hinzufügen",
  remove_position: "Position entfernen",
  recipient: "Empfänger ändern",
  address: "Anschrift ändern",
  revoke: "Änderung zurücknehmen",
};

const SAVE_LABELS: Record<CareAmendmentDraft["kind"], string> = {
  amount: "Betrag ändern",
  add_position: "Hinzufügen",
  remove_position: "Entfernen",
  recipient: "Empfänger ändern",
  address: "Anschrift ändern",
  revoke: "Zurücknehmen",
};

const REASON_HINTS: Record<CareAmendmentDraft["kind"], string> = {
  amount: "Zum Beispiel: Kulanz wegen Schließtag",
  add_position: "Zum Beispiel: Ausflug laut Elternbrief",
  remove_position: "Zum Beispiel: Modul war nicht gebucht",
  recipient: "Zum Beispiel: Rechnung geht an den Vater",
  address: "Zum Beispiel: Familie ist umgezogen",
  revoke: "Zum Beispiel: Betrag ist wieder richtig",
};

interface Fields {
  total: string;
  label: string;
  amount: string;
  firstName: string;
  lastName: string;
  secondName: string;
  email: string;
  street: string;
  postalCode: string;
  city: string;
  reason: string;
}

function initialFields(invoice: CareInvoice): Fields {
  const address = invoice.recipient.address;
  return {
    total: centsToEuroInput(invoice.totalCents),
    label: "",
    amount: "",
    firstName: invoice.recipient.firstName,
    lastName: invoice.recipient.lastName,
    secondName: invoice.recipient.secondName ?? "",
    email: invoice.recipient.email,
    street: address?.street ?? "",
    postalCode: address?.postalCode ?? "",
    city: address?.city ?? "",
    reason: "",
  };
}

type FieldErrors = Partial<Record<keyof Fields, string>>;

const SERVER_FIELDS: Readonly<Record<string, keyof Fields>> = {
  total_cents: "total",
  amount_cents: "amount",
  "recipient.first_name": "firstName",
  "recipient.last_name": "lastName",
  "recipient.second_name": "secondName",
  "recipient.email": "email",
  "address.street": "street",
  "address.postal_code": "postalCode",
  "address.city": "city",
  recipient: "firstName",
  address: "street",
};

/** The change the fields describe, or the field errors that keep it back. */
function draftInput(
  draft: CareAmendmentDraft,
  fields: Fields,
): { input: DraftChange } | { errors: FieldErrors } {
  const errors: FieldErrors = {};
  if (!fields.reason.trim()) errors.reason = "Bitte nennen Sie einen Grund.";
  let input: DraftChange | null = null;
  switch (draft.kind) {
    case "amount": {
      const cents = euroInputToCents(fields.total);
      if (cents === null)
        errors.total = "Bitte geben Sie einen Betrag ein, zum Beispiel 70,00.";
      else input = { kind: "amount", totalCents: cents };
      break;
    }
    case "add_position": {
      const cents = euroInputToCents(fields.amount);
      if (!fields.label.trim())
        errors.label = "Bitte benennen Sie die Position.";
      if (cents === null || cents === 0)
        errors.amount = "Bitte geben Sie einen Betrag über 0 ein.";
      else
        input = {
          kind: "add_position",
          label: fields.label.trim(),
          amountCents: cents,
        };
      break;
    }
    case "recipient":
      if (!fields.firstName.trim())
        errors.firstName = "Bitte geben Sie den Vornamen ein.";
      if (!fields.lastName.trim())
        errors.lastName = "Bitte geben Sie den Nachnamen ein.";
      if (!/^\S+@\S+\.\S+$/.test(fields.email.trim()))
        errors.email = "Bitte geben Sie eine gültige E-Mail-Adresse ein.";
      input = {
        kind: "recipient",
        recipient: {
          firstName: fields.firstName.trim(),
          lastName: fields.lastName.trim(),
          secondName: fields.secondName.trim(),
          email: fields.email.trim(),
        },
      };
      break;
    case "address":
      if (!fields.street.trim())
        errors.street = "Bitte geben Sie Straße und Hausnummer ein.";
      if (!/^\d{5}$/.test(fields.postalCode.trim()))
        errors.postalCode = "Bitte geben Sie fünf Ziffern ein.";
      if (!fields.city.trim()) errors.city = "Bitte geben Sie den Ort ein.";
      input = {
        kind: "address",
        address: {
          street: fields.street.trim(),
          postalCode: fields.postalCode.trim(),
          city: fields.city.trim(),
        },
      };
      break;
    case "remove_position":
      input = { kind: "remove_position", positionKey: draft.positionKey };
      break;
    case "revoke":
      input = {
        kind: "revoke",
        revokesId: draft.amendment.id,
        revokesCarried: draft.amendment.carriedFromNumber !== null,
      };
      break;
  }
  if (Object.keys(errors).length > 0 || !input) return { errors };
  return { input };
}

type DistributiveOmit<T, K extends PropertyKey> = T extends unknown
  ? Omit<T, K>
  : never;
type DraftChange = DistributiveOmit<
  CareAmendmentInput,
  "contractId" | "service" | "fingerprint" | "reason"
>;

/** One change of one invoice of the preview, with its reason. */
export function CareInvoiceAmendmentModal({
  run,
  invoice,
  draft,
  onClose,
  onSaved,
}: Readonly<{
  run: CareRunDetail;
  invoice: CareInvoice;
  draft: CareAmendmentDraft | null;
  onClose: () => void;
  onSaved: (run: CareRunDetail) => void;
}>) {
  const [fields, setFields] = useState<Fields>(() => initialFields(invoice));
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear } = formErrors;

  useEffect(() => {
    if (!draft) return;
    setFields(initialFields(invoice));
    clear();
  }, [draft, invoice, clear]);

  const set = (key: keyof Fields) => (event: { target: { value: string } }) =>
    setFields((current) => ({ ...current, [key]: event.target.value }));

  const submit = async () => {
    if (!draft) return;
    formErrors.clear();
    const result = draftInput(draft, fields);
    if ("errors" in result) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", result.errors);
      return;
    }
    setSaving(true);
    try {
      onSaved(
        await amendCareInvoice(run.id, {
          ...result.input,
          contractId: invoice.contractId,
          service: invoice.service,
          fingerprint: run.fingerprint,
          reason: fields.reason.trim(),
        }),
      );
    } catch (err) {
      logger.error("traeger_care_invoice_amend_failed", {
        kind: draft.kind,
        error: err instanceof Error ? err.message : String(err),
      });
      if (err instanceof ApiError && err.errors) {
        err.errors = err.errors.map((entry) => ({
          ...entry,
          field: SERVER_FIELDS[entry.field] ?? entry.field,
        }));
      }
      void formErrors.show(err, { object: "die Rechnung" });
    } finally {
      setSaving(false);
    }
  };

  const fieldError = (key: keyof Fields) => formErrors.fieldError(key);
  return (
    <FormModal
      isOpen={draft !== null}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title={draft ? TITLES[draft.kind] : ""}
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel={draft ? SAVE_LABELS[draft.kind] : ""}
        />
      }
    >
      <form
        ref={formRef}
        noValidate
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        {draft ? (
          <DraftFields
            draft={draft}
            invoice={invoice}
            fields={fields}
            set={set}
            fieldError={fieldError}
          />
        ) : null}
        <Textarea
          id="care-amendment-reason"
          name="reason"
          label="Grund"
          required
          autoGrow
          rows={2}
          maxLength={MAX_REASON}
          placeholder={draft ? REASON_HINTS[draft.kind] : undefined}
          hint="Der Grund steht im Verlauf der Rechnung und in der Prüfliste."
          value={fields.reason}
          error={fieldError("reason")}
          onChange={set("reason")}
        />
      </form>
    </FormModal>
  );
}

function DraftFields({
  draft,
  invoice,
  fields,
  set,
  fieldError,
}: Readonly<{
  draft: CareAmendmentDraft;
  invoice: CareInvoice;
  fields: Fields;
  set: (key: keyof Fields) => (event: { target: { value: string } }) => void;
  fieldError: (key: keyof Fields) => string | undefined;
}>) {
  switch (draft.kind) {
    case "amount":
      return (
        <Input
          id="care-amendment-total"
          name="total"
          label="Neuer Betrag in Euro"
          required
          inputMode="decimal"
          value={fields.total}
          error={fieldError("total")}
          onChange={set("total")}
          hint={`Jetzt ${formatEuro(invoice.totalCents)}. Die Rechnung bekommt eine Korrektur-Zeile mit dem Unterschied.`}
        />
      );
    case "add_position":
      return (
        <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_10rem]">
          <Input
            id="care-amendment-label"
            name="label"
            label="Bezeichnung"
            required
            maxLength={MAX_LABEL}
            placeholder="Zum Beispiel: Ausflug Wildpark"
            value={fields.label}
            error={fieldError("label")}
            onChange={set("label")}
          />
          <Input
            id="care-amendment-amount"
            name="amount"
            label="Betrag in Euro"
            required
            inputMode="decimal"
            placeholder="12,00"
            value={fields.amount}
            error={fieldError("amount")}
            onChange={set("amount")}
          />
        </div>
      );
    case "recipient":
      return (
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            id="care-amendment-first-name"
            name="firstName"
            label="Vorname"
            required
            value={fields.firstName}
            error={fieldError("firstName")}
            onChange={set("firstName")}
          />
          <Input
            id="care-amendment-last-name"
            name="lastName"
            label="Nachname"
            required
            value={fields.lastName}
            error={fieldError("lastName")}
            onChange={set("lastName")}
          />
          <Input
            id="care-amendment-second-name"
            name="secondName"
            label="Zweite Person (optional)"
            value={fields.secondName}
            error={fieldError("secondName")}
            onChange={set("secondName")}
          />
          <Input
            id="care-amendment-email"
            name="email"
            type="email"
            label="E-Mail"
            required
            value={fields.email}
            error={fieldError("email")}
            onChange={set("email")}
          />
          <p className="text-sm text-gray-600 sm:col-span-2">
            Gilt nur für diese Rechnung. Für alle Rechnungen der Familie ändern
            Sie „Empfänger und Zahlung“.
          </p>
        </div>
      );
    case "address":
      return (
        <div className="grid gap-4 sm:grid-cols-[8rem_minmax(0,1fr)]">
          <div className="sm:col-span-2">
            <Input
              id="care-amendment-street"
              name="street"
              label="Straße und Hausnummer"
              required
              value={fields.street}
              error={fieldError("street")}
              onChange={set("street")}
            />
          </div>
          <Input
            id="care-amendment-postal-code"
            name="postalCode"
            label="PLZ"
            required
            inputMode="numeric"
            maxLength={5}
            value={fields.postalCode}
            error={fieldError("postalCode")}
            onChange={set("postalCode")}
          />
          <Input
            id="care-amendment-city"
            name="city"
            label="Ort"
            required
            value={fields.city}
            error={fieldError("city")}
            onChange={set("city")}
          />
          <p className="text-sm text-gray-600 sm:col-span-2">
            Gilt nur für diese Rechnung, nicht für die Familie.
          </p>
        </div>
      );
    case "remove_position":
      return (
        <p className="text-sm text-gray-700">
          <span className="font-semibold text-gray-900">{draft.label}</span>{" "}
          entfällt. Die Rechnung bekommt eine Zeile mit{" "}
          <span className="tabular-nums">{formatEuro(-draft.amountCents)}</span>
          . Die Position bleibt sichtbar.
        </p>
      );
    case "revoke":
      return (
        <p className="text-sm text-gray-700">
          Die Rechnung gilt danach wieder ohne diese Änderung. Der Eintrag
          bleibt im Verlauf.
        </p>
      );
  }
}
