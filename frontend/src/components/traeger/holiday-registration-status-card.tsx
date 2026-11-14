"use client";

// Status einer Ferienanmeldung und was das Träger-Büro damit tun kann
// (#4663): bestätigen, stornieren, einfrieren und wieder öffnen. Die Karte
// sagt, warum eine Aktion gerade fehlt. Das Bestätigen hält den Ferienbeitrag
// fest, auf Wunsch mit der Befreiung bei Leistungsbezug (#4664) oder mit
// der Kostenübernahme durch einen Kostenträger (#5627). Fragt der
// Träger die Debitorennummer ab, ist sie beim Bestätigen Pflicht (#5621).
// Bei einer bestätigten Anmeldung trägt das Büro die Nummer hier nach oder
// ändert sie, bis die Rechnung ausgestellt ist; danach hält die Rechnung sie
// fest. Hat die Familie beim Träger schon eine Nummer, steht sie als
// Vorschlag am Feld; weicht die eingetragene ab, fragt ein Dialog, welche
// Nummer gilt (#6692).

import { useEffect, useRef, useState, type ReactNode } from "react";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldLabel } from "~/components/ui/field-label";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import NavigationLink from "~/components/ui/navigation-link";
import { ConfirmationModal } from "~/components/ui/modal";
import { Input } from "~/components/ui/input";
import { Radio } from "~/components/ui/radio";
import { SectionCard } from "~/components/ui/section-card";
import { enrollmentsConcept } from "~/lib/concepts/enrollments";
import {
  DEBTOR_NUMBER_TEXT_CLASS,
  FamilyDebtorNumberConflictDialog,
  FamilyDebtorNumberHint,
} from "~/components/traeger/family-debtor-number";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import {
  DEBTOR_NUMBER_MAX_LENGTH,
  type DebtorNumberFormat,
  NO_DEBTOR_NUMBER_FORMAT,
  debtorNumberFormatError,
  debtorNumberFormatHint,
} from "~/lib/debtor-number-format";
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
  familyDebtorNumberConflictOf,
  type FamilyDebtorNumberChoice,
  type FamilyDebtorNumberConflict,
  type HolidayContribution,
} from "~/lib/traeger-contributions-api";
import {
  changeHolidayRegistration,
  type CostBearerChoice,
  type HolidayRegistrationAction,
  type HolidayRegistrationDetail,
} from "~/lib/traeger-holiday-registrations-api";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";

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
    ? " Sie ist eingefroren. Zum Stornieren oder Umbuchen wählen Sie „Wieder öffnen“."
    : " Nach dem Anmeldeschluss wird sie eingefroren. Zum Stornieren oder Umbuchen wählen Sie dann „Wieder öffnen“.";
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

/**
 * The field hints of a number the form refuses before it asks the backend:
 * none where the Träger wants one (#5621), another format (#6675).
 */
function debtorNumberHints(
  value: string,
  required: boolean,
  format: DebtorNumberFormat,
): Record<string, string> | null {
  if (required && !value.trim()) {
    return { debtor_number: "Bitte geben Sie die Debitorennummer ein." };
  }
  const formatError = debtorNumberFormatError(value, format);
  return formatError ? { debtor_number: formatError } : null;
}

/** What the card says about the debtor number, before and after the invoice. */
function debtorNumberText(set: boolean, invoiced: boolean): string {
  const missing = set
    ? ""
    : " Ohne Nummer steht im Schreiben die Rechnungsnummer als Verwendungszweck.";
  if (!invoiced) {
    return `Sie steht im Schreiben als Verwendungszweck. Ändern können Sie sie, bis die Rechnung ausgestellt ist.${missing}`;
  }
  return set
    ? "Die Rechnung ist ausgestellt. Die Debitorennummer lässt sich nicht mehr ändern."
    : `Die Rechnung ist ohne Debitorennummer ausgestellt. Eine Nummer lässt sich nicht mehr eintragen.${missing}`;
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
  familyNumber,
  required,
  format,
  invoiced,
  editing,
  onEditingChange,
  onChanged,
}: Readonly<{
  periodId: string;
  registrationId: string;
  debtorNumber: string;
  /** The number the family has at the Träger (#6692); "" for none. */
  familyNumber: string;
  required: boolean;
  /** The format of the Träger's numbers (#6675). */
  format: DebtorNumberFormat;
  invoiced: boolean;
  editing: boolean;
  onEditingChange: (editing: boolean) => void;
  /** Reloads the registration; never rejects (#6652). */
  onChanged: RefreshAfterWrite;
}>) {
  const [value, setValue] = useState(debtorNumber);
  const [saving, setSaving] = useState(false);
  const [conflict, setConflict] = useState<FamilyDebtorNumberConflict | null>(
    null,
  );
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

  const submit = async (choice?: FamilyDebtorNumberChoice) => {
    if (saving) return;
    formErrors.clear();
    const hints = debtorNumberHints(value, required, format);
    if (hints) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", hints);
      return;
    }
    setSaving(true);
    try {
      await changeHolidayDebtorNumber(
        periodId,
        registrationId,
        value.trim(),
        choice,
      );
      setConflict(null);
    } catch (err) {
      // A number that differs from the family's needs a choice (#6692).
      const differs = familyDebtorNumberConflictOf(err);
      if (differs) {
        // The request is over: the dialog's choices must work.
        setSaving(false);
        setConflict(differs);
        return;
      }
      setConflict(null);
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
        <div className="sm:max-w-xl">
          <Input
            ref={fieldRef}
            id="holiday-debtor-number"
            required={required}
            name="debtor_number"
            label="Debitorennummer"
            autoComplete="off"
            controlSize="touch"
            inputMode={format.digitsOnly ? "numeric" : undefined}
            maxLength={DEBTOR_NUMBER_MAX_LENGTH}
            value={value}
            hint={`${debtorNumberFormatHint(format)} ${DEBTOR_NUMBER_HINT}`.trim()}
            aria-describedby={
              familyNumber && value.trim() !== familyNumber
                ? "holiday-debtor-number-family"
                : undefined
            }
            error={formErrors.fieldError("debtor_number") ?? undefined}
            onChange={(event) => setValue(event.target.value)}
          />
          <FamilyDebtorNumberHint
            id="holiday-debtor-number-family"
            familyNumber={familyNumber}
            value={value}
            onAdopt={(number) => {
              setValue(number);
              fieldRef.current?.focus();
            }}
          />
        </div>
        <EditActions
          onCancel={() => onEditingChange(false)}
          saving={saving}
          saveType="submit"
        />
        <FamilyDebtorNumberConflictDialog
          conflict={conflict}
          running={saving}
          onChoose={(choice) => void submit(choice)}
          onClose={() => setConflict(null)}
        />
      </form>
    );
  }

  return (
    <div className="space-y-2">
      <DataGrid columns={2}>
        <DataField label="Debitorennummer">
          {debtorNumber ? (
            <span className={`wrap-anywhere ${DEBTOR_NUMBER_TEXT_CLASS}`}>
              {debtorNumber}
            </span>
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
      <div className="flex flex-col gap-2 sm:max-w-xl">
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
                <span className="mt-0.5 block text-sm font-normal text-gray-600">
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
  debtorNumberFormat = NO_DEBTOR_NUMBER_FORMAT,
  familyDebtorNumber = "",
  exemption,
  onExemptionChange,
  costBearers = NO_COST_BEARERS,
  costBearerId = null,
  onCostBearerChange,
  onChanged,
  onReload,
  onDebtorNumberChanged,
  canManageBillingSettings = false,
  billingHref = null,
}: Readonly<{
  periodId: string;
  registration: HolidayRegistrationDetail;
  /** The contribution a confirmation freezes; null while it loads. */
  contribution: HolidayContribution | null;
  /** The preview is computed again; the confirmation waits for it. */
  previewPending?: boolean;
  /** The Träger asks for the debtor number at the confirmation (#5621). */
  debtorNumberRequired?: boolean;
  /** The format of the Träger's debtor numbers (#6675). */
  debtorNumberFormat?: DebtorNumberFormat;
  /**
   * The number the family already has at the Träger (#6692), proposed at the
   * field; "" for none.
   */
  familyDebtorNumber?: string;
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
  /** Where the invoice of the period stands; null without the right. */
  billingHref?: string | null;
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
  const enteredDebtorNumber =
    typedDebtorNumber?.registrationId === registration.id
      ? typedDebtorNumber.value
      : null;
  const debtorNumber =
    enteredDebtorNumber ?? contribution?.siblingDebtorNumber ?? "";
  const setDebtorNumber = (value: string) =>
    setTypedDebtorNumber({ registrationId: registration.id, value });
  const [editingDebtorNumber, setEditingDebtorNumber] = useState(false);
  const [conflict, setConflict] = useState<FamilyDebtorNumberConflict | null>(
    null,
  );
  const confirmFieldRef = useRef<HTMLInputElement>(null);
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
    (debtorNumberRequired ||
      Boolean(registration.debtorNumber) ||
      Boolean(familyDebtorNumber));
  // The confirmation asks for the number when the Träger wants it, and
  // offers it when the family already has one (#6692).
  const asksDebtorNumber = debtorNumberRequired || Boolean(familyDebtorNumber);

  const confirm = async (choice?: FamilyDebtorNumberChoice) => {
    if (confirmBlocked) return;
    confirmErrors.clear();
    const hints = debtorNumberHints(
      debtorNumber,
      debtorNumberRequired,
      enteredDebtorNumber === null
        ? NO_DEBTOR_NUMBER_FORMAT
        : debtorNumberFormat,
    );
    if (hints) {
      confirmErrors.invalid("Bitte prüfen Sie Ihre Angaben.", hints);
      return;
    }
    setRunning("confirm");
    try {
      await confirmHolidayRegistration(
        periodId,
        registration.id,
        payer === "exemption",
        // An untouched sibling number is inherited under the backend's locks.
        enteredDebtorNumber?.trim() ?? "",
        payer === "cost_bearer" ? (costBearerId ?? undefined) : undefined,
        choice,
      );
      setConflict(null);
    } catch (err) {
      // A number that differs from the family's needs a choice (#6692).
      const differs = familyDebtorNumberConflictOf(err);
      if (differs) {
        // The request is over: the dialog's choices must work.
        setRunning(null);
        setConflict(differs);
        return;
      }
      setConflict(null);
      logger.error("holiday_registration_confirm_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void confirmErrors.show(err, { object: "die Anmeldung" });
      setRunning(null);
      return;
    }
    await afterWrite.refresh();
    setRunning(null);
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
  const duplicateChildren = registration.children.filter(
    (child) => child.duplicateRegistrationIds.length > 0,
  );
  const duplicateNames = duplicateChildren.map((child) =>
    `${child.firstName} ${child.lastName}`.trim(),
  );
  const duplicateIds = [
    ...new Set(
      duplicateChildren.flatMap((child) => child.duplicateRegistrationIds),
    ),
  ];
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
          tooltip="Öffnet die Anmeldung wieder, damit Sie stornieren oder umbuchen können."
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
            tooltip="Storniert die Anmeldung, Nachrückende bekommen eine E-Mail. moto fragt vorher nach."
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
            tooltip="Sperrt die Anmeldung sofort gegen Stornieren und Umbuchen."
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
          tooltip="Öffnet das Feld für die Debitorennummer dieser Anmeldung."
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
          tooltip="Bestätigt die Anmeldung und schickt den Eltern sofort die Zusage per E-Mail."
          onClick={() => confirmRef.current?.requestSubmit()}
        >
          Bestätigen
        </Button>
      ) : null}
    </div>
  ) : undefined;
  return (
    <SectionCard
      leading={<ConceptIconTile concept={enrollmentsConcept} />}
      title="Status"
      footer={actions}
    >
      <LoadErrorAlert error={afterWrite.error} className="mb-4" />
      <LoadErrorAlert error={debtorAfterWrite.error} className="mb-4" />
      <div className="space-y-4 text-sm text-gray-700">
        <p>{statusText(registration)}</p>
        {/* The children's cards sit far below on a phone: the status names a
            possible double registration first, in one line; the child's card
            holds the way to the other registration. */}
        {duplicateIds.length > 0 ? (
          <Alert
            type="warning"
            message={`Vielleicht doppelt angemeldet: ${duplicateNames.join(", ")}. Die andere Anmeldung öffnen Sie beim Kind.`}
          />
        ) : null}
        {registration.status === "received" ? (
          <p>
            Beim Bestätigen bekommen die Eltern eine E-Mail mit den gebuchten
            Wochen und dem{" "}
            <NavigationLink
              href={traegerPath(`/traeger/ferien/${periodId}`)}
              className="font-medium underline"
            >
              Text der Annahme-Mail
            </NavigationLink>
            .
          </p>
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
            {asksDebtorNumber ? (
              <div className="sm:max-w-xl">
                <Input
                  ref={confirmFieldRef}
                  id="holiday-registration-debtor-number"
                  required={debtorNumberRequired}
                  name="debtor_number"
                  label="Debitorennummer"
                  autoComplete="off"
                  controlSize="touch"
                  inputMode={
                    debtorNumberFormat.digitsOnly ? "numeric" : undefined
                  }
                  maxLength={DEBTOR_NUMBER_MAX_LENGTH}
                  value={debtorNumber}
                  hint={`${debtorNumberFormatHint(debtorNumberFormat)} ${DEBTOR_NUMBER_HINT}`.trim()}
                  aria-describedby={
                    familyDebtorNumber &&
                    debtorNumber.trim() !== familyDebtorNumber
                      ? "holiday-registration-debtor-number-family"
                      : undefined
                  }
                  error={confirmErrors.fieldError("debtor_number") ?? undefined}
                  onChange={(event) => setDebtorNumber(event.target.value)}
                />
                <FamilyDebtorNumberHint
                  id="holiday-registration-debtor-number-family"
                  familyNumber={familyDebtorNumber}
                  value={debtorNumber}
                  onAdopt={(number) => {
                    setDebtorNumber(number);
                    confirmFieldRef.current?.focus();
                  }}
                />
              </div>
            ) : null}
          </form>
        ) : null}
        {invoiced && billingHref ? (
          <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
            <p>Die Rechnung dieser Anmeldung steht in der Abrechnung.</p>
            <ButtonLink
              href={billingHref}
              variant="outline"
              size="compact"
              tooltip="Öffnet die Abrechnung mit der Rechnung dieser Anmeldung."
            >
              Zur Abrechnung
            </ButtonLink>
          </div>
        ) : null}
        {showsDebtorNumber ? (
          <DebtorNumberField
            periodId={periodId}
            registrationId={registration.id}
            debtorNumber={registration.debtorNumber}
            familyNumber={familyDebtorNumber}
            required={debtorNumberRequired}
            format={debtorNumberFormat}
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
      <FamilyDebtorNumberConflictDialog
        conflict={conflict}
        running={running === "confirm"}
        onChoose={(choice) => void confirm(choice)}
        onClose={() => setConflict(null)}
      />
      <ConfirmationModal
        isOpen={confirmingCancel}
        title="Anmeldung stornieren?"
        confirmText="Anmeldung stornieren"
        confirmVariant="danger"
        isConfirmLoading={running === "cancel"}
        loadingText="Storniert …"
        onClose={() => setConfirmingCancel(false)}
        onConfirm={() => void run("cancel")}
      >
        {/* Names what is cancelled, so the decision does not rely on the
            blurred page behind the dialog. */}
        <p className="text-sm font-medium text-gray-900">
          Anmeldung Nr. {registration.id} von{" "}
          {`${registration.guardianFirstName} ${registration.guardianLastName}`.trim()}
          {registration.children.length > 0
            ? `: ${registration.children
                .map((child) => `${child.firstName} ${child.lastName}`.trim())
                .join(", ")}`
            : ""}
        </p>
        {/* One consequence per line, so the irreversible step reads at a
            glance; the way to the invoice stays on the page. */}
        <ul className="mt-2 list-disc space-y-1 pl-5 text-sm text-gray-700">
          <li>
            Die Kinder sind dann für keine Woche mehr angemeldet. Das lässt sich
            nicht rückgängig machen.
          </li>
          <li>Eltern bekommen keine Nachricht von moto.</li>
          {contribution?.siblings?.length ? (
            <li>
              Die Anmeldung zählt dann nicht mehr zu den Geschwistern. moto
              berechnet den Ferienbeitrag der Geschwister neu.
            </li>
          ) : null}
          {contribution?.siblings?.length && contribution.siblingsInvoiced ? (
            <li>
              Die schon ausgestellte Rechnung der Geschwister bleibt, wie sie
              ist. Den neuen Betrag zeigt dann ihre Anmeldung unter „Rechnung
              prüfen“.
            </li>
          ) : null}
          {invoiced ? (
            <li>
              Die Rechnung dieser Anmeldung bleibt bestehen. Soll die Familie
              nicht zahlen, stornieren Sie sie danach in der Abrechnung.
            </li>
          ) : null}
        </ul>
      </ConfirmationModal>
    </SectionCard>
  );
}
