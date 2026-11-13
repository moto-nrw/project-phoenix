"use client";

// Die Lastschrift des Trägers (#4695): Gläubiger-ID, wie viele Tage vor der
// Abbuchung die Vorabankündigung bei der Familie ist, und was eine Rechnung
// ohne gültiges Mandat tut. Steht bei den Bankverbindungen, weil die
// Gläubiger-ID auf jeder Vorabankündigung steht.

import { useEffect, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { Radio } from "~/components/ui/radio";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldLabel } from "~/components/ui/field-label";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { traegerPath } from "~/lib/traeger-url";
import {
  MAX_PRENOTIFICATION_DAYS,
  MIN_PRENOTIFICATION_DAYS,
  type MissingMandate,
  type SepaCreditor,
  saveSepaCreditor,
} from "~/lib/traeger-sepa-api";
import {
  MISSING_MANDATE_HINTS,
  MISSING_MANDATE_LABELS,
  compactCode,
  creditorIdLooksValid,
} from "./sepa-format";

const logger = createLogger({ component: "SepaCreditorCard" });

const MISSING_CHOICES: readonly MissingMandate[] = ["transfer", "block"];

function daysLabel(days: number): string {
  return days === 1 ? "1 Tag vorher" : `${days} Tage vorher`;
}

function SepaCreditorModal({
  open,
  creditor,
  onClose,
  onSaved,
}: Readonly<{
  open: boolean;
  creditor: SepaCreditor;
  onClose: () => void;
  onSaved: (creditor: SepaCreditor) => void;
}>) {
  const [creditorId, setCreditorId] = useState(creditor.creditorId);
  const [days, setDays] = useState(String(creditor.prenotificationDays));
  const [missing, setMissing] = useState<MissingMandate>(
    creditor.missingMandate,
  );
  const save = useFormModalPending(open);
  const saving = save.pending;
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!open) return;
    setCreditorId(creditor.creditorId);
    setDays(String(creditor.prenotificationDays));
    setMissing(creditor.missingMandate);
    clearErrors();
  }, [open, creditor, clearErrors]);

  const submit = async () => {
    formErrors.clear();
    const fields: Record<string, string> = {};
    const compact = compactCode(creditorId);
    if (compact !== "" && !creditorIdLooksValid(compact)) {
      fields.creditor_id =
        "Diese Gläubiger-ID stimmt nicht. Bitte prüfen Sie jedes Zeichen.";
    }
    const parsed = /^\d{1,2}$/.test(days.trim()) ? Number(days.trim()) : NaN;
    if (!(
      parsed >= MIN_PRENOTIFICATION_DAYS && parsed <= MAX_PRENOTIFICATION_DAYS
    )) {
      fields.prenotification_days = `Bitte eine Zahl von ${MIN_PRENOTIFICATION_DAYS} bis ${MAX_PRENOTIFICATION_DAYS} eingeben.`;
    }
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", fields);
      return;
    }
    await save.run(async (op) => {
      try {
        const saved = await saveSepaCreditor({
          creditorId: compact,
          prenotificationDays: parsed,
          missingMandate: missing,
        });
        success("Die Angaben zur Lastschrift sind gespeichert.");
        if (op.isCurrent()) onSaved(saved);
      } catch (err) {
        logger.error("sepa_creditor_save_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!op.isCurrent()) return;
        void formErrors.show(err, { object: "die Angaben zur Lastschrift" });
      }
    });
  };

  return (
    <FormModal
      isOpen={open}
      onClose={onClose}
      pending={save}
      isBackdropDismissDisabled
      title="Lastschrift"
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
      <form
        ref={formRef}
        noValidate
        className="space-y-5"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <Input
          controlSize="touch"
          id="sepa-creditor-id"
          name="creditor_id"
          label="Gläubiger-ID"
          placeholder="DE98ZZZ09999999999"
          autoComplete="off"
          spellCheck={false}
          maxLength={40}
          value={creditorId}
          hint="Sie bekommen die Gläubiger-ID von der Bundesbank. Ohne sie gibt es keine Lastschrift."
          error={formErrors.fieldError("creditor_id") ?? undefined}
          onChange={(event) => setCreditorId(event.target.value)}
        />
        {creditor.creditorId !== "" &&
        compactCode(creditorId) !== creditor.creditorId ? (
          <Alert
            type="warning"
            message="Bisherige Mandate gelten nur für die bisherige Gläubiger-ID. Nach dem Wechsel brauchen die Familien ein neues Mandat. Bis dahin bekommen sie eine Rechnung zum Überweisen."
          />
        ) : null}
        <Input
          controlSize="touch"
          id="sepa-prenotification-days"
          name="prenotification_days"
          label="Vorabankündigung in Tagen"
          required
          inputMode="numeric"
          maxLength={2}
          value={days}
          hint="So viele Tage vor der Abbuchung muss die Rechnung bei der Familie sein. Üblich sind 14 Tage. Weniger nur, wenn Sie das mit den Familien vereinbart haben."
          error={formErrors.fieldError("prenotification_days") ?? undefined}
          onChange={(event) => setDays(event.target.value)}
        />
        <div className="space-y-2">
          <FieldLabel id="sepa-creditor-missing-label">
            Wenn eine Familie kein gültiges Mandat hat
          </FieldLabel>
          <div
            role="radiogroup"
            aria-labelledby="sepa-creditor-missing-label"
            className="grid gap-2"
          >
            {MISSING_CHOICES.map((value) => (
              <ChoiceTile
                key={value}
                selected={missing === value}
                className="items-start gap-3 px-4 py-3"
              >
                <Radio
                  name="sepa-creditor-missing"
                  value={value}
                  checked={missing === value}
                  onChange={() => setMissing(value)}
                  className="mt-0.5"
                />
                <span className="flex min-w-0 flex-col gap-1">
                  <span className="font-medium text-gray-900">
                    {MISSING_MANDATE_LABELS[value]}
                    {value === "transfer" ? " (empfohlen)" : ""}
                  </span>
                  <span className="text-sm font-normal text-gray-600">
                    {MISSING_MANDATE_HINTS[value]}
                  </span>
                </span>
              </ChoiceTile>
            ))}
          </div>
        </div>
        <p className="text-sm text-gray-600">
          Das gilt ab dem nächsten Lauf. Ausgestellte Rechnungen bleiben, wie
          sie sind.
        </p>
      </form>
    </FormModal>
  );
}

/** The settings area with the switch of the online mandate (#4696). */
const ONLINE_MANDATE_SETTINGS_PATH =
  "/traeger/einstellungen?bereich=monatsbeitrag";

export function SepaCreditorCard({
  creditor,
  onlineMandate,
  onChanged,
}: Readonly<{
  creditor: SepaCreditor;
  /** The Träger's switch of the mandate in the parents portal; null while unknown. */
  onlineMandate?: boolean | null;
  onChanged: () => Promise<unknown>;
}>) {
  const [editing, setEditing] = useState(false);
  const missingId = creditor.creditorId === "";

  return (
    <SectionCard
      title="Lastschrift"
      titleBadge={
        missingId ? (
          <StatusBadge tone="warning" label="Gläubiger-ID fehlt" />
        ) : null
      }
      description="Damit buchen Sie Beiträge per SEPA-Lastschrift ab. Das Mandat jeder Familie erfassen Sie bei ihrem Vertrag."
      actions={
        <Button
          type="button"
          variant="outline"
          size="md"
          tooltip="Öffnet Gläubiger-ID, Vorabankündigung und Regel ohne Mandat zum Bearbeiten."
          onClick={() => setEditing(true)}
        >
          Ändern
        </Button>
      }
    >
      <DataGrid columns={2}>
        <DataField label="Gläubiger-ID">
          {missingId ? (
            "–"
          ) : (
            <span className="font-mono tracking-wide">
              {creditor.creditorId}
            </span>
          )}
        </DataField>
        <DataField label="Vorabankündigung">
          <span className="tabular-nums">
            {daysLabel(creditor.prenotificationDays)}
          </span>
        </DataField>
        <DataField label="Ohne gültiges Mandat" fullWidth>
          {MISSING_MANDATE_LABELS[creditor.missingMandate]}
        </DataField>
        {onlineMandate === undefined || onlineMandate === null ? null : (
          <DataField label="Mandat in der Eltern-App" fullWidth>
            <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <StatusBadge
                tone={onlineMandate ? "green" : "outline"}
                label={onlineMandate ? "Eingeschaltet" : "Ausgeschaltet"}
              />
              <ButtonLink
                href={traegerPath(ONLINE_MANDATE_SETTINGS_PATH)}
                variant="link"
                size="md"
                tooltip="Öffnet die Einstellung, ob Eltern das Mandat in der App erteilen."
              >
                In den Einstellungen ändern
              </ButtonLink>
            </span>
            <span className="mt-1 block text-sm font-normal text-gray-600">
              {onlineMandate
                ? "Eltern erteilen und widerrufen das Mandat selbst, bei ihrem Vertrag."
                : "Eltern geben das Mandat auf Papier ab. Sie erfassen es beim Vertrag der Familie."}
            </span>
          </DataField>
        )}
      </DataGrid>
      <SepaCreditorModal
        open={editing}
        creditor={creditor}
        onClose={() => setEditing(false)}
        onSaved={() => {
          setEditing(false);
          void onChanged();
        }}
      />
    </SectionCard>
  );
}
