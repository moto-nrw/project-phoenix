"use client";

// Status einer Ferienanmeldung und was das Träger-Büro damit tun kann
// (#4663): bestätigen, stornieren, einfrieren und wieder öffnen. Die Karte
// sagt, warum eine Aktion gerade fehlt. Das Bestätigen hält den Ferienbeitrag
// fest, auf Wunsch mit der Befreiung bei Leistungsbezug (#4664) oder mit
// der Kostenübernahme durch einen Kostenträger (#5627). Fragt der
// Träger die Debitorennummer ab, ist sie beim Bestätigen Pflicht (#5621).
// Bei einer bestätigten Anmeldung trägt das Büro die Nummer hier nach oder
// ändert sie, bis die Rechnung ausgestellt ist; danach hält die Rechnung sie
// fest.

import { useEffect, useRef, useState, type ReactNode } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldLabel } from "~/components/ui/field-label";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import NavigationLink from "~/components/ui/navigation-link";
import { ConfirmationModal } from "~/components/ui/modal";
import { Radio } from "~/components/ui/radio";
import { SectionCard } from "~/components/ui/section-card";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import {
  useRefreshAfterWrite,
  type RefreshAfterWrite,
} from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import { traegerPath } from "~/lib/traeger-url";
import {
  cancelHolidayRegistration,
  changeHolidayDebtorNumber,
  confirmHolidayRegistration,
  type HolidayContribution,
} from "~/lib/traeger-contributions-api";
import {
  changeHolidayRegistration,
  type CostBearerChoice,
  type HolidayRegistrationAction,
  type HolidayRegistrationDetail,
} from "~/lib/traeger-holiday-registrations-api";

const logger = createLogger({ component: "HolidayRegistrationStatusCard" });

function statusText(registration: HolidayRegistrationDetail): string {
  if (registration.status === "cancelled") {
    return "Diese Anmeldung ist storniert. Sie lässt sich nicht mehr ändern.";
  }
  const confirmed =
    registration.status === "confirmed"
      ? "Die Anmeldung ist bestätigt."
      : "Die Anmeldung ist eingegangen und noch nicht bestätigt.";
  const frozen = registration.frozen
    ? " Sie ist eingefroren. Stornieren und Umbuchen gehen erst nach „Wieder öffnen“."
    : " Nach dem Anmeldeschluss wird sie von selbst eingefroren.";
  return confirmed + frozen;
}

/** What the card says about the parents' statement on benefits. */
function benefitsText(
  registration: HolidayRegistrationDetail,
  contribution: HolidayContribution | null,
  canManageBillingSettings: boolean,
): ReactNode {
  if (!registration.benefitsDeclared) return null;
  if (contribution?.exemptionAvailable) {
    return "Die Eltern geben an, dass sie Leistungen beziehen. Bitte prüfen Sie den Nachweis.";
  }
  if (!canManageBillingSettings) {
    return "Die Eltern geben an, dass sie Leistungen beziehen. Am ersten gebuchten Tag gilt keine Befreiungsregel. Die Rolle „Abrechnung“ legt sie an.";
  }
  return (
    <>
      Die Eltern geben an, dass sie Leistungen beziehen. Am ersten gebuchten Tag
      gilt keine Befreiungsregel. Sie legen sie unter{" "}
      <NavigationLink
        href={traegerPath("/traeger/einstellungen/abrechnung")}
        className="font-medium underline"
      >
        Abrechnungsvorgaben
      </NavigationLink>{" "}
      an.
    </>
  );
}

/** The hint under the debtor number, at the confirmation and at a change. */
const DEBTOR_NUMBER_HINT =
  "Die Nummer der Familie bei der Kasse der Gemeinde. Die Familie gibt sie bei der Überweisung an.";

/** The longest debtor number the backend keeps (#5621). */
const DEBTOR_NUMBER_MAX_LENGTH = 35;

/** What the card says about the debtor number, before and after the invoice. */
function debtorNumberText(set: boolean, invoiced: boolean): string {
  const missing = set
    ? ""
    : " Ohne Nummer steht im Schreiben die Rechnungsnummer als Verwendungszweck.";
  if (!invoiced) {
    return `Sie steht im Schreiben als Verwendungszweck. Ändern geht, bis die Rechnung ausgestellt ist.${missing}`;
  }
  return set
    ? "Die Rechnung ist ausgestellt. Sie behält diese Nummer. Ändern geht nicht mehr."
    : `Die Rechnung ist ohne Debitorennummer ausgestellt. Eintragen geht nicht mehr.${missing}`;
}

/**
 * The debtor number of a confirmed registration (#5621): shown, and edited in
 * place until the invoice is issued. `editing` belongs to the card, which
 * hides its own button while the field is open.
 */
function DebtorNumberField({
  periodId,
  registrationId,
  debtorNumber,
  required,
  invoiced,
  editing,
  onEditingChange,
  onChanged,
}: Readonly<{
  periodId: string;
  registrationId: string;
  debtorNumber: string;
  required: boolean;
  invoiced: boolean;
  editing: boolean;
  onEditingChange: (editing: boolean) => void;
  /** Reloads the registration; never rejects (#6652). */
  onChanged: RefreshAfterWrite;
}>) {
  const [value, setValue] = useState(debtorNumber);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const fieldRef = useRef<HTMLInputElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!editing) return;
    setValue(debtorNumber);
    clearErrors();
    fieldRef.current?.focus();
  }, [editing, debtorNumber, clearErrors]);

  const submit = async () => {
    if (saving) return;
    formErrors.clear();
    if (required && !value.trim()) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", {
        debtor_number: "Bitte geben Sie die Debitorennummer ein.",
      });
      return;
    }
    setSaving(true);
    try {
      await changeHolidayDebtorNumber(periodId, registrationId, value.trim());
    } catch (err) {
      logger.error("holiday_debtor_number_change_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Debitorennummer" });
      setSaving(false);
      return;
    }
    setSaving(false);
    success("Die Debitorennummer ist gespeichert.");
    onEditingChange(false);
    await onChanged();
  };

  if (editing && !invoiced) {
    return (
      <form
        ref={formRef}
        noValidate
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <FormErrorAlert message={formErrors.error} />
        <div className="sm:max-w-md">
          <Input
            ref={fieldRef}
            id="holiday-debtor-number"
            name="debtor_number"
            label="Debitorennummer"
            required={required}
            autoComplete="off"
            controlSize="compact"
            maxLength={DEBTOR_NUMBER_MAX_LENGTH}
            value={value}
            aria-describedby="holiday-debtor-number-hint"
            error={formErrors.fieldError("debtor_number") ?? undefined}
            onChange={(event) => setValue(event.target.value)}
          />
          <p
            id="holiday-debtor-number-hint"
            className="mt-1 text-xs text-gray-600"
          >
            {DEBTOR_NUMBER_HINT}
          </p>
        </div>
        <EditActions
          onCancel={() => onEditingChange(false)}
          saving={saving}
          saveType="submit"
        />
      </form>
    );
  }

  return (
    <div className="space-y-2">
      <DataGrid columns={2}>
        <DataField label="Debitorennummer">
          {debtorNumber ? (
            <span className="font-mono wrap-anywhere">{debtorNumber}</span>
          ) : (
            "Noch keine Nummer."
          )}
        </DataField>
      </DataGrid>
      <p>{debtorNumberText(Boolean(debtorNumber), invoiced)}</p>
    </div>
  );
}

type Payer = "family" | "exemption" | "cost_bearer";

const NO_COST_BEARERS: readonly CostBearerChoice[] = [];

/**
 * Who pays the contribution a confirmation freezes (#5627): the family,
 * nobody (the exemption) or a cost bearer of the Träger. Exemption and cost
 * takeover exclude each other; each option shows only when the Träger has
 * an exemption rule or an active cost bearer.
 */
function PayerChoice({
  payer,
  exemptionAvailable,
  costBearers,
  costBearerId,
  onPayerChange,
  onCostBearerChange,
}: Readonly<{
  payer: Payer;
  exemptionAvailable: boolean;
  costBearers: readonly CostBearerChoice[];
  costBearerId: string | null;
  onPayerChange: (payer: Payer) => void;
  onCostBearerChange: (id: string) => void;
}>) {
  const options: { value: Payer; label: string; description: string }[] = [
    {
      value: "family",
      label: "Die Familie bezahlt",
      description: "Die Familie bekommt eine Rechnung über den Betrag.",
    },
  ];
  if (exemptionAvailable) {
    options.push({
      value: "exemption",
      label: "Von der Zahlung befreien (Leistungsbezug)",
      description: "Dann gilt die Befreiungsregel. Niemand bezahlt den Betrag.",
    });
  }
  if (costBearers.length > 0) {
    options.push({
      value: "cost_bearer",
      label: "Kosten übernimmt ein Kostenträger",
      description:
        "Die Familie bezahlt nichts. Sie fordern den Betrag beim Kostenträger an.",
    });
  }
  return (
    <fieldset className="space-y-2">
      <legend className="mb-2 text-sm font-medium text-gray-700">
        Wer bezahlt den Beitrag?
      </legend>
      <div className="flex flex-col gap-2 sm:max-w-md">
        {options.map((option) => {
          const id = `holiday-registration-payer-${option.value}`;
          return (
            <ChoiceTile
              key={option.value}
              htmlFor={id}
              selected={payer === option.value}
              className="items-start p-3"
            >
              <Radio
                id={id}
                name="payer"
                value={option.value}
                checked={payer === option.value}
                onChange={() => onPayerChange(option.value)}
                className="mt-0.5"
              />
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-medium text-gray-900">
                  {option.label}
                </span>
                <span className="mt-0.5 block text-xs font-normal text-gray-600">
                  {option.description}
                </span>
              </span>
            </ChoiceTile>
          );
        })}
        {payer === "cost_bearer" ? (
          <div>
            <FieldLabel
              id="holiday-registration-cost-bearer-label"
              htmlFor="holiday-registration-cost-bearer"
            >
              Kostenträger
            </FieldLabel>
            <CustomSelect
              id="holiday-registration-cost-bearer"
              name="cost_bearer_id"
              ariaLabelledBy="holiday-registration-cost-bearer-label"
              value={costBearerId ?? ""}
              options={costBearers.map((bearer) => ({
                value: bearer.id,
                label: bearer.name,
              }))}
              onChange={onCostBearerChange}
              wrapLabels
            />
          </div>
        ) : null}
      </div>
    </fieldset>
  );
}

export function HolidayRegistrationStatusCard({
  periodId,
  registration,
  contribution,
  previewPending = false,
  debtorNumberRequired = false,
  exemption,
  onExemptionChange,
  costBearers = NO_COST_BEARERS,
  costBearerId = null,
  onCostBearerChange,
  onChanged,
  onReload,
  onDebtorNumberChanged,
  canManageBillingSettings = false,
}: Readonly<{
  periodId: string;
  registration: HolidayRegistrationDetail;
  /** The contribution a confirmation freezes; null while it loads. */
  contribution: HolidayContribution | null;
  /** The preview is computed again; the confirmation waits for it. */
  previewPending?: boolean;
  /** The Träger asks for the debtor number at the confirmation (#5621). */
  debtorNumberRequired?: boolean;
  /** The exemption the confirmation applies; the page shows its preview. */
  exemption: boolean;
  onExemptionChange: (exemption: boolean) => void;
  /** The active cost bearers the confirmation offers (#5627). */
  costBearers?: readonly CostBearerChoice[];
  /** The cost bearer that pays; null while the family pays or is exempt. */
  costBearerId?: string | null;
  onCostBearerChange?: (costBearerId: string | null) => void;
  onChanged: (registration: HolidayRegistrationDetail) => void;
  /**
   * After a confirmation or a cancellation; the page reads the registration
   * and its contribution again. A cancellation can change the sibling group
   * (#5662).
   */
  /** Reads the registration again and rejects when that fails. */
  onReload: () => Promise<unknown>;
  /** After a change of the debtor number; the page reads it again. */
  onDebtorNumberChanged: () => Promise<unknown>;
  /** Das Recht „Abrechnung“ öffnet die Beitragsregeln. */
  canManageBillingSettings?: boolean;
}>) {
  const [running, setRunning] = useState<
    HolidayRegistrationAction | "confirm" | "cancel" | null
  >(null);
  const [confirmingCancel, setConfirmingCancel] = useState(false);
  // A sibling group pays with one number (#5628): its current number is the
  // proposal. It follows the registration and its group, for example after
  // a split, until the Träger-Büro types its own number for this
  // registration; that one stays.
  const [typedDebtorNumber, setTypedDebtorNumber] = useState<{
    readonly registrationId: string;
    readonly value: string;
  } | null>(null);
  const debtorNumber =
    typedDebtorNumber?.registrationId === registration.id
      ? typedDebtorNumber.value
      : (contribution?.siblingDebtorNumber ?? "");
  const setDebtorNumber = (value: string) =>
    setTypedDebtorNumber({ registrationId: registration.id, value });
  const [editingDebtorNumber, setEditingDebtorNumber] = useState(false);
  const confirmRef = useRef<HTMLFormElement>(null);
  const confirmErrors = useApiFormError(confirmRef);
  const errorDisplay = useApiErrorDisplay();
  // A write's reload stands above the card as a load error, never as an
  // error of the confirmation, the cancellation or the debtor number (#6652).
  const afterWrite = useRefreshAfterWrite(
    "die Anzeige der Anmeldung",
    onReload,
  );
  const debtorAfterWrite = useRefreshAfterWrite(
    "die Anzeige der Anmeldung",
    onDebtorNumberChanged,
  );
  const exemptionAvailable = contribution?.exemptionAvailable ?? false;
  const invoiced = contribution?.invoiced ?? false;
  let payer: Payer = "family";
  if (
    costBearerId &&
    costBearers.some((bearer) => bearer.id === costBearerId)
  ) {
    payer = "cost_bearer";
  } else if (exemptionAvailable && exemption) {
    payer = "exemption";
  }
  // Enter in the debtor number submits the form, so the guards of the button
  // hold here too: one confirmation at a time, and only of the amount shown.
  const confirmBlocked =
    running !== null || contribution === null || previewPending;
  const showsDebtorNumber =
    registration.status === "confirmed" &&
    (debtorNumberRequired || Boolean(registration.debtorNumber));

  const confirm = async () => {
    if (confirmBlocked) return;
    confirmErrors.clear();
    if (debtorNumberRequired && !debtorNumber.trim()) {
      confirmErrors.invalid("Bitte prüfen Sie Ihre Angaben.", {
        debtor_number: "Bitte geben Sie die Debitorennummer ein.",
      });
      return;
    }
    setRunning("confirm");
    try {
      await confirmHolidayRegistration(
        periodId,
        registration.id,
        payer === "exemption",
        debtorNumber.trim(),
        payer === "cost_bearer" ? (costBearerId ?? undefined) : undefined,
      );
    } catch (err) {
      logger.error("holiday_registration_confirm_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void confirmErrors.show(err, { object: "die Anmeldung" });
      setRunning(null);
      return;
    }
    setRunning(null);
    await afterWrite.refresh();
  };

  const run = async (action: HolidayRegistrationAction | "cancel") => {
    setRunning(action);
    try {
      if (action === "cancel") {
        // The Elternabrechnung cancels and computes the siblings again
        // (#5662); the page reads both again.
        await cancelHolidayRegistration(periodId, registration.id);
      } else {
        onChanged(
          await changeHolidayRegistration(periodId, registration.id, action),
        );
      }
    } catch (err) {
      logger.error("holiday_registration_change_failed", {
        action,
        error: err instanceof Error ? err.message : String(err),
      });
      void errorDisplay.show(err, { object: "die Anmeldung" });
      setRunning(null);
      setConfirmingCancel(false);
      return;
    }
    setRunning(null);
    setConfirmingCancel(false);
    if (action === "cancel") await afterWrite.refresh();
  };

  const open = registration.status !== "cancelled";
  const choosesPayer =
    registration.status === "received" &&
    (exemptionAvailable || costBearers.length > 0);
  const changePayer = (next: Payer) => {
    if (next === "cost_bearer") {
      onCostBearerChange?.(costBearerId ?? costBearers[0]?.id ?? null);
      return;
    }
    onCostBearerChange?.(null);
    onExemptionChange(next === "exemption");
  };
  // Hauptaktion rechts, wie „Speichern“ in den Formularen (#5684).
  const actions = open ? (
    <div className="flex flex-wrap justify-end gap-2">
      {registration.frozen ? (
        <Button
          type="button"
          variant="outline"
          size="md"
          isLoading={running === "reopen"}
          loadingText="Öffnet …"
          disabled={running !== null}
          onClick={() => void run("reopen")}
        >
          Wieder öffnen
        </Button>
      ) : (
        <>
          <Button
            type="button"
            variant="outline_danger"
            size="md"
            disabled={running !== null}
            onClick={() => setConfirmingCancel(true)}
          >
            Stornieren
          </Button>
          <Button
            type="button"
            variant="outline"
            size="md"
            isLoading={running === "freeze"}
            loadingText="Friert ein …"
            disabled={running !== null}
            onClick={() => void run("freeze")}
          >
            Einfrieren
          </Button>
        </>
      )}
      {showsDebtorNumber && !invoiced && !editingDebtorNumber ? (
        <Button
          type="button"
          variant="outline"
          size="md"
          disabled={running !== null}
          onClick={() => setEditingDebtorNumber(true)}
        >
          {registration.debtorNumber
            ? "Debitorennummer ändern"
            : "Debitorennummer eintragen"}
        </Button>
      ) : null}
      {registration.status === "received" ? (
        <Button
          type="button"
          variant="primary"
          size="md"
          isLoading={running === "confirm"}
          loadingText="Bestätigt …"
          disabled={confirmBlocked}
          onClick={() => confirmRef.current?.requestSubmit()}
        >
          Bestätigen
        </Button>
      ) : null}
    </div>
  ) : undefined;
  return (
    <SectionCard title="Status" footer={actions}>
      <LoadErrorAlert error={afterWrite.error} className="mb-4" />
      <LoadErrorAlert error={debtorAfterWrite.error} className="mb-4" />
      <div className="space-y-4 text-sm text-gray-700">
        <p>{statusText(registration)}</p>
        {registration.status === "received" ? (
          <p>Eltern bekommen beim Bestätigen noch keine Nachricht von moto.</p>
        ) : null}
        {registration.status === "received" &&
        registration.children.some((child) =>
          child.bookings.some((booking) => booking.placement === "pending"),
        ) ? (
          <Alert
            type="warning"
            message={
              <>
                Für ein Kind ist noch ein Platz offen. Bestätigen geht erst,
                wenn das Büro im Reiter{" "}
                <NavigationLink
                  href={traegerPath(`/traeger/ferien/${periodId}/vergabe`)}
                  className="font-medium underline"
                >
                  Vergabe
                </NavigationLink>{" "}
                entschieden hat.
              </>
            }
          />
        ) : null}
        {registration.status === "received" &&
        benefitsText(registration, contribution, canManageBillingSettings) ? (
          <p>
            {benefitsText(registration, contribution, canManageBillingSettings)}
          </p>
        ) : null}
        {registration.status === "received" ? (
          <form
            ref={confirmRef}
            noValidate
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault();
              void confirm();
            }}
          >
            <FormErrorAlert message={confirmErrors.error} />
            {debtorNumberRequired ? (
              <div className="sm:max-w-md">
                <Input
                  id="holiday-registration-debtor-number"
                  name="debtor_number"
                  label="Debitorennummer"
                  required
                  autoComplete="off"
                  controlSize="compact"
                  maxLength={DEBTOR_NUMBER_MAX_LENGTH}
                  value={debtorNumber}
                  aria-describedby="holiday-registration-debtor-number-hint"
                  error={confirmErrors.fieldError("debtor_number") ?? undefined}
                  onChange={(event) => setDebtorNumber(event.target.value)}
                />
                <p
                  id="holiday-registration-debtor-number-hint"
                  className="mt-1 text-xs text-gray-600"
                >
                  {DEBTOR_NUMBER_HINT}
                </p>
              </div>
            ) : null}
          </form>
        ) : null}
        {showsDebtorNumber ? (
          <DebtorNumberField
            periodId={periodId}
            registrationId={registration.id}
            debtorNumber={registration.debtorNumber}
            required={debtorNumberRequired}
            invoiced={invoiced}
            editing={editingDebtorNumber}
            onEditingChange={setEditingDebtorNumber}
            onChanged={debtorAfterWrite.refresh}
          />
        ) : null}
        {choosesPayer ? (
          <PayerChoice
            payer={payer}
            exemptionAvailable={exemptionAvailable}
            costBearers={costBearers}
            costBearerId={costBearerId}
            onPayerChange={changePayer}
            onCostBearerChange={(id) => onCostBearerChange?.(id)}
          />
        ) : null}
      </div>
      <ConfirmationModal
        isOpen={confirmingCancel}
        title="Anmeldung stornieren?"
        confirmText="Stornieren"
        confirmVariant="danger"
        isConfirmLoading={running === "cancel"}
        loadingText="Storniert …"
        onClose={() => setConfirmingCancel(false)}
        onConfirm={() => void run("cancel")}
      >
        <p className="text-sm text-gray-700">
          Die Kinder dieser Anmeldung sind dann für keine Woche mehr angemeldet.
          Eine stornierte Anmeldung lässt sich nicht wieder öffnen. Eltern
          bekommen keine Nachricht von moto.
        </p>
        {contribution?.siblings?.length ? (
          <p className="mt-2 text-sm text-gray-700">
            Die Anmeldung zählt dann nicht mehr zu den Geschwistern. moto
            berechnet den Ferienbeitrag der Geschwister neu.
          </p>
        ) : null}
      </ConfirmationModal>
    </SectionCard>
  );
}
