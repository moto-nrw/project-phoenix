"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { InsetPanel } from "~/components/ui/inset-panel";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  saveTraegerWithdrawalContact,
  type TraegerContact,
  type TraegerWithdrawalContact,
} from "~/lib/traeger-api";
import { traegerPath } from "~/lib/traeger-url";
import {
  withdrawalAddressee,
  withdrawalReach,
  type WithdrawalReachSource,
  type WithdrawalReachValue,
} from "~/lib/traeger-withdrawal-contact";

const logger = createLogger({ component: "TraegerWithdrawalContactCard" });

const SOURCE_BADGES: Record<
  WithdrawalReachSource,
  { readonly label: string; readonly tone: StatusBadgeTone }
> = {
  own: { label: "Eigene Angabe", tone: "green" },
  contact: { label: "Vom Ansprechpartner", tone: "gray" },
  missing: { label: "Fehlt", tone: "orange" },
};

function sameReach(
  a: TraegerWithdrawalContact,
  b: TraegerWithdrawalContact,
): boolean {
  return a.phone === b.phone && a.email === b.email;
}

function fallbackHint(contactValue: string, what: string): string {
  return contactValue === ""
    ? `Beim Ansprechpartner steht keine ${what}.`
    : `Leer lassen: Dann steht hier ${contactValue} vom Ansprechpartner.`;
}

function ReachField({
  label,
  reach,
}: Readonly<{ label: string; reach: WithdrawalReachValue }>) {
  const badge = SOURCE_BADGES[reach.source];
  return (
    <DataField label={label}>
      <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
        <span className="min-w-0 wrap-anywhere">
          {reach.value === "" ? "–" : reach.value}
        </span>
        <StatusBadge label={badge.label} tone={badge.tone} size="xs" />
      </span>
    </DataField>
  );
}

/**
 * Phone and e-mail for withdrawals (#6200): the withdrawal notice of every
 * care contract names them, on the PDF, the signing page, in the mail and in
 * the text the school copies into its enrollment form.
 * An empty field takes the contact person's. The preview shows the notice's
 * sentence as parents read it, with where each value comes from.
 */
export function TraegerWithdrawalContactCard({
  withdrawal,
  contact,
  letterheadText,
  canEdit,
  onSaved,
}: Readonly<{
  withdrawal: TraegerWithdrawalContact;
  contact: TraegerContact;
  letterheadText: string;
  canEdit: boolean;
  onSaved: () => Promise<unknown>;
}>) {
  const [draft, setDraft] = useState<TraegerWithdrawalContact>(withdrawal);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { success } = useToast();

  // A refresh after another change on the page brings a new object; an
  // open draft stays until it is saved or cancelled.
  const shown = useRef(withdrawal);
  const [stored, setStored] = useState(withdrawal);
  useEffect(() => {
    const before = shown.current;
    shown.current = withdrawal;
    setStored(withdrawal);
    setDraft((current) => (sameReach(current, before) ? withdrawal : current));
  }, [withdrawal]);

  const changed = !sameReach(draft, stored);
  const reach = withdrawalReach(contact, draft);
  const addressee = withdrawalAddressee(letterheadText, reach);
  const missing = [
    reach.phone.source === "missing" ? "Telefonnummer" : null,
    reach.email.source === "missing" ? "E-Mail-Adresse" : null,
  ].filter((value) => value !== null);

  const submit = async () => {
    formErrors.clear();
    setSaving(true);
    const submitted = draft;
    try {
      const saved = await saveTraegerWithdrawalContact(submitted);
      shown.current = saved;
      setStored(saved);
      setDraft((current) => (sameReach(current, submitted) ? saved : current));
      success("Der Kontakt für Widerrufe ist gespeichert.");
      await onSaved();
    } catch (err) {
      logger.error("traeger_withdrawal_contact_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "der Kontakt für Widerrufe" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard
      title="Kontakt für Widerrufe"
      description="Eltern können einen Vertrag 14 Tage lang widerrufen. Diese Angaben stehen dafür in der Widerrufsbelehrung."
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
        <FormErrorAlert message={formErrors.error} />
        <div className="grid gap-4 md:grid-cols-2">
          <Input
            controlSize="touch"
            id="traeger-withdrawal-phone"
            label="Telefon für Widerrufe"
            name="phone"
            type="tel"
            value={draft.phone}
            placeholder={contact.phone}
            hint={fallbackHint(contact.phone, "Telefonnummer")}
            onChange={(e) => setDraft({ ...draft, phone: e.target.value })}
            error={formErrors.fieldError("phone")}
            maxLength={40}
            disabled={!canEdit}
          />
          <Input
            controlSize="touch"
            id="traeger-withdrawal-email"
            label="E-Mail für Widerrufe"
            name="email"
            type="email"
            value={draft.email}
            placeholder={contact.email}
            hint={fallbackHint(contact.email, "E-Mail-Adresse")}
            onChange={(e) => setDraft({ ...draft, email: e.target.value })}
            error={formErrors.fieldError("email")}
            maxLength={254}
            disabled={!canEdit}
          />
        </div>

        {missing.length > 0 ? (
          <Alert
            type="warning"
            message={`Die Widerrufsbelehrung braucht Telefon und E-Mail. Es fehlt: ${missing.join(" und ")}. Bitte tragen Sie sie hier oder beim Ansprechpartner ein.`}
          />
        ) : null}

        <figure aria-label="Vorschau der Widerrufsbelehrung">
          <InsetPanel padding="md" className="space-y-3">
            <SectionLabel as="p">So lesen Eltern die Belehrung</SectionLabel>
            <blockquote className="text-sm leading-relaxed text-pretty text-gray-700">
              Um Ihr Widerrufsrecht auszuüben, müssen Sie uns (
              <span className="font-medium text-gray-900">
                {addressee.join(", ")}
              </span>
              ) mittels einer eindeutigen Erklärung (z. B. ein mit der Post
              versandter Brief oder E-Mail) über Ihren Entschluss, diesen
              Vertrag zu widerrufen, informieren.
            </blockquote>
            <DataGrid>
              <ReachField label="Telefon" reach={reach.phone} />
              <ReachField label="E-Mail" reach={reach.email} />
            </DataGrid>
            <p className="text-xs text-gray-600">
              Name und Anschrift kommen aus dem{" "}
              {/* A new tab keeps what is typed in the contact form beside
                  this text; leaving the page would drop it. */}
              <Link
                href={traegerPath("/traeger/einstellungen/briefkopf")}
                target="_blank"
                rel="noopener"
                className="font-medium underline"
              >
                Briefkopf
                <span className="sr-only"> (öffnet in einem neuen Tab)</span>
              </Link>
              . Nennen die Vertragsbedingungen einen anderen Vertragspartner,
              steht er davor. Nach einer Änderung kopiert die Schule die
              Belehrung neu ins Anmeldeformular.
            </p>
          </InsetPanel>
        </figure>

        {canEdit && changed ? (
          <EditActions
            onCancel={() => {
              setDraft(stored);
              formErrors.clear();
            }}
            saving={saving}
            saveType="submit"
          />
        ) : null}
      </form>
    </SectionCard>
  );
}
