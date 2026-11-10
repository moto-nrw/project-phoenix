"use client";

// Änderungswunsch freigeben oder ablehnen (#5979). Die Karte
// „Änderungswunsch der Eltern“ lädt das Formular erst beim Öffnen.

import { useEffect, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DatePicker } from "~/components/ui/date-picker";
import { FieldLabel } from "~/components/ui/field-label";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import type {
  ContractChangeRequest,
  ContractChanges,
} from "~/lib/care-contract-changes";
import { ApiError, wireErrorCode } from "~/lib/api-error";
import { OfferingRequestChanges } from "~/components/students/offering-request-changes";
import { contractChangeReview } from "~/lib/care-contract-changes";
import {
  decideContractChange,
  fetchContractChanges,
} from "~/lib/care-contract-changes-api";
import {
  berlinTodayISO,
  formatDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CareContractChangeModal" });

const FORM_ID = "care-contract-change-form";

/**
 * Approves or rejects the parents' change. An approval books the change
 * from the confirmed day and tells the parents; the contract follows with a
 * new version. A rejection tells the parents the reason.
 */
export function DecideChangeModal({
  isOpen,
  onClose,
  contractId,
  request,
  approve,
  reasonRequired,
  onDecided,
  onConflict,
}: Readonly<{
  isOpen: boolean;
  onClose: () => void;
  contractId: string;
  request: ContractChangeRequest;
  approve: boolean;
  /** The school asks for a reason on an approval too. */
  reasonRequired: boolean;
  onDecided: (next: ContractChanges) => void;
  onConflict: () => void;
}>) {
  const [previewAttempt, setPreviewAttempt] = useState(0);
  const [preview, setPreview] = useState<{
    date: string;
    request: ContractChangeRequest;
  } | null>(null);
  const today = berlinTodayISO();
  // A wish for a day that has passed takes effect today at the earliest.
  const [effectiveFrom, setEffectiveFrom] = useState(
    request.effectiveFrom < today ? today : request.effectiveFrom,
  );
  const [reason, setReason] = useState("");
  const save = useFormModalPending(isOpen);
  const saving = save.pending;
  const form = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(form);
  const { success } = useToast();
  const { show: showFormError, clear: clearFormError } = formErrors;
  useEffect(() => {
    if (!approve || !effectiveFrom) return;
    let active = true;
    setPreview(null);
    clearFormError();
    void fetchContractChanges(contractId, effectiveFrom)
      .then((next) => {
        if (!active) return;
        const current = next.pending.find((item) => item.id === request.id);
        if (
          !next.decidable ||
          !current ||
          current.version !== request.version
        ) {
          onConflict();
          return;
        }
        setPreview({ date: effectiveFrom, request: current });
      })
      .catch((err: unknown) => {
        if (!active) return;
        if (isChangedRequest(err)) onConflict();
        else
          void showFormError(err, {
            object: "die Vorschau",
            retry: () => setPreviewAttempt((attempt) => attempt + 1),
          });
      });
    return () => {
      active = false;
    };
  }, [
    previewAttempt,
    approve,
    contractId,
    effectiveFrom,
    request.id,
    request.version,
    clearFormError,
    showFormError,
    onConflict,
  ]);
  const needsReason = !approve || reasonRequired;
  const ready =
    (!needsReason || reason.trim() !== "") &&
    (!approve || (effectiveFrom && preview?.date === effectiveFrom));

  const submit = async () => {
    // Ein zweiter Klick während des Speicherns ändert nichts: run läuft nur
    // einmal je Dialog.
    if (!ready) return;
    formErrors.clear();
    await save.run(async (op) => {
      try {
        const next = await decideContractChange(contractId, request.id, {
          approve,
          reason,
          effectiveFrom: approve ? effectiveFrom : undefined,
          expectedVersion: request.version,
        });
        success(
          approve
            ? `Die Änderung ist freigegeben. Sie gilt ab ${formatDate(effectiveFrom)}. Der Vertrag bekommt in wenigen Minuten eine neue Version.`
            : "Der Änderungswunsch ist abgelehnt. Die Eltern bekommen Ihre Begründung.",
        );
        if (op.isCurrent()) onDecided(next);
      } catch (err) {
        logger.error("traeger_care_contract_change_decide_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!op.isCurrent()) return;
        if (isChangedRequest(err)) onConflict();
        else void formErrors.show(err, { object: "der Änderungswunsch" });
      }
    });
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={approve ? "Änderungswunsch freigeben" : "Änderungswunsch ablehnen"}
      size="sm"
      pending={save}
      isBackdropDismissDisabled
      error={formErrors.error}
      footer={
        <div className="flex w-full flex-wrap justify-end gap-3">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={saving}
          >
            Abbrechen
          </Button>
          <Button
            type="submit"
            form={FORM_ID}
            variant={approve ? "success" : "danger"}
            size="md"
            disabled={saving || !ready}
            isLoading={saving}
            loadingText={approve ? "Wird freigegeben …" : "Wird abgelehnt …"}
          >
            {approve ? "Freigeben" : "Ablehnen"}
          </Button>
        </div>
      }
    >
      <form
        id={FORM_ID}
        ref={form}
        noValidate
        className="space-y-5"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        {approve ? (
          <div className="space-y-1.5">
            <FieldLabel htmlFor="care-contract-change-effective-from" required>
              Gilt ab
            </FieldLabel>
            <DatePicker
              id="care-contract-change-effective-from"
              name="effective_from"
              value={effectiveFrom ? parseISODate(effectiveFrom) : undefined}
              onChange={(date) => {
                setPreview(null);
                setEffectiveFrom(date ? toISODate(date) : "");
              }}
              minDate={parseISODate(today)}
              required
              disabled={saving}
              invalid={Boolean(formErrors.fieldError("effective_from"))}
            />
            <p className="text-sm text-gray-500">
              {`Die Eltern wünschen ${formatDate(request.effectiveFrom)}.`}
            </p>
          </div>
        ) : null}
        {approve &&
        effectiveFrom &&
        preview?.date !== effectiveFrom &&
        !formErrors.error ? (
          <Alert
            type="info"
            message="Die Änderungen werden für dieses Datum geprüft."
          />
        ) : null}
        {approve && preview?.date === effectiveFrom ? (
          <OfferingRequestChanges row={contractChangeReview(preview.request)} />
        ) : null}
        <Textarea
          id="care-contract-change-reason"
          label={needsReason ? "Begründung" : "Nachricht an die Eltern"}
          hint={
            approve && reasonRequired
              ? "Die Schule verlangt eine Begründung auch bei einer Freigabe. Die Eltern lesen sie in der Eltern-App."
              : "Die Eltern lesen sie in der Eltern-App."
          }
          required={needsReason}
          autoGrow
          rows={3}
          maxLength={1000}
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          error={formErrors.fieldError("reason")}
          disabled={saving}
        />
        <Alert
          type="info"
          message={
            approve
              ? "Die gebuchte Betreuung ändert sich ab diesem Tag. Die Eltern bekommen eine Nachricht. Der Vertrag bekommt danach eine neue Version, die alte bleibt lesbar."
              : "Die gebuchte Betreuung und der Vertrag bleiben, wie sie sind."
          }
        />
      </form>
    </FormModal>
  );
}

function isChangedRequest(error: unknown): boolean {
  if (!(error instanceof ApiError)) return false;
  const code = wireErrorCode(error.code);
  return (
    code === "students.change_request_stale" ||
    code === "students.change_request_not_pending" ||
    code === "care_contract.change_not_found"
  );
}
