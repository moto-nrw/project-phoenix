"use client";

// Status einer Ferienanmeldung und was das Träger-Büro damit tun kann
// (#4663): bestätigen, stornieren, einfrieren und wieder öffnen. Die Karte
// sagt, warum eine Aktion gerade fehlt. Das Bestätigen hält den Ferienbeitrag
// fest, auf Wunsch mit der Befreiung bei Leistungsbezug (#4664). Fragt der
// Träger die Debitorennummer ab, ist sie beim Bestätigen Pflicht (#5621).

import { useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay, useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  confirmHolidayRegistration,
  type HolidayContribution,
} from "~/lib/traeger-contributions-api";
import {
  changeHolidayRegistration,
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
): string | null {
  if (!registration.benefitsDeclared) return null;
  if (contribution?.exemptionAvailable) {
    return "Die Eltern geben an, dass sie Leistungen beziehen. Bitte prüfen Sie den Nachweis.";
  }
  return "Die Eltern geben an, dass sie Leistungen beziehen. Für eine Befreiung braucht es eine Regel unter „Ermäßigungen“.";
}

/** The hint under the debtor number, here and where it is changed. */
export const DEBTOR_NUMBER_HINT =
  "Die Nummer der Familie bei der Kasse der Gemeinde. Die Familie gibt sie bei der Überweisung an.";

/** The longest debtor number the backend keeps (#5621). */
export const DEBTOR_NUMBER_MAX_LENGTH = 35;

export function HolidayRegistrationStatusCard({
  periodId,
  registration,
  contribution,
  previewPending = false,
  debtorNumberRequired = false,
  exemption,
  onExemptionChange,
  onChanged,
  onConfirmed,
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
  onChanged: (registration: HolidayRegistrationDetail) => void;
  /** After a confirmation; the page reads the registration again. */
  onConfirmed: () => Promise<void>;
}>) {
  const [running, setRunning] = useState<
    HolidayRegistrationAction | "confirm" | null
  >(null);
  const [confirmingCancel, setConfirmingCancel] = useState(false);
  const [debtorNumber, setDebtorNumber] = useState("");
  const confirmRef = useRef<HTMLFormElement>(null);
  const confirmErrors = useApiFormError(confirmRef);
  const errorDisplay = useApiErrorDisplay();
  const exemptionAvailable = contribution?.exemptionAvailable ?? false;

  const confirm = async () => {
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
        exemptionAvailable && exemption,
        debtorNumber.trim(),
      );
      await onConfirmed();
    } catch (err) {
      logger.error("holiday_registration_confirm_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void confirmErrors.show(err, { object: "die Anmeldung" });
    } finally {
      setRunning(null);
    }
  };

  const run = async (action: HolidayRegistrationAction) => {
    setRunning(action);
    try {
      onChanged(
        await changeHolidayRegistration(periodId, registration.id, action),
      );
    } catch (err) {
      logger.error("holiday_registration_change_failed", {
        action,
        error: err instanceof Error ? err.message : String(err),
      });
      void errorDisplay.show(err, { object: "die Anmeldung" });
    } finally {
      setRunning(null);
      setConfirmingCancel(false);
    }
  };

  const open = registration.status !== "cancelled";
  return (
    <SectionCard title="Status">
      <div className="space-y-4 text-sm text-gray-700">
        <p>{statusText(registration)}</p>
        {registration.status === "received" ? (
          <p>Eltern bekommen beim Bestätigen noch keine Nachricht von moto.</p>
        ) : null}
        {registration.status === "received" &&
        benefitsText(registration, contribution) ? (
          <p>{benefitsText(registration, contribution)}</p>
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
              <div>
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
        {registration.status === "received" && exemptionAvailable ? (
          <label
            htmlFor="holiday-registration-exemption"
            className="flex items-start gap-3"
          >
            <Checkbox
              id="holiday-registration-exemption"
              name="benefits_exemption"
              checked={exemption}
              onChange={(event) => onExemptionChange(event.target.checked)}
            />
            <span>
              Beim Bestätigen von der Zahlung befreien (Leistungsbezug)
            </span>
          </label>
        ) : null}
        {open ? (
          <div className="flex flex-wrap gap-2">
            {registration.status === "received" ? (
              <Button
                type="button"
                variant="primary"
                size="md"
                isLoading={running === "confirm"}
                loadingText="Bestätigt …"
                disabled={
                  running !== null || contribution === null || previewPending
                }
                onClick={() => confirmRef.current?.requestSubmit()}
              >
                Bestätigen
              </Button>
            ) : null}
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
                  variant="outline"
                  size="md"
                  isLoading={running === "freeze"}
                  loadingText="Friert ein …"
                  disabled={running !== null}
                  onClick={() => void run("freeze")}
                >
                  Einfrieren
                </Button>
                <Button
                  type="button"
                  variant="outline_danger"
                  size="md"
                  disabled={running !== null}
                  onClick={() => setConfirmingCancel(true)}
                >
                  Stornieren
                </Button>
              </>
            )}
          </div>
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
      </ConfirmationModal>
    </SectionCard>
  );
}
