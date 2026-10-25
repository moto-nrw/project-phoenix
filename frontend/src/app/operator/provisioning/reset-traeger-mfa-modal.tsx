"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { TraegerMember } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ResetTraegerMfaModal" });

const MIN_REASON_LENGTH = 3;

/**
 * Resets the second factor of a Träger-Büro member (#4651): the recovery for
 * a person who is locked out or no longer trusts the factor. The factor
 * belongs to the account, so the reset applies to every portal; the running
 * Träger session ends at its next refresh. The reason goes to the operator
 * audit.
 */
export function ResetTraegerMfaModal({
  member,
  onClose,
  organizationId,
  onReset,
}: {
  readonly member: TraegerMember | null;
  readonly onClose: () => void;
  readonly organizationId: string;
  readonly onReset: () => Promise<unknown>;
}) {
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, clear: clearError } = formErrors;
  const { success: toastSuccess } = useToast();

  useEffect(() => {
    if (member) {
      setReason("");
      clearError();
    }
  }, [member, clearError]);

  const reasonValid = reason.trim().length >= MIN_REASON_LENGTH;

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!member || !reasonValid) return;
      setSaving(true);
      clearError();
      try {
        await operatorProvisioningService.resetTraegerMemberMfa(
          organizationId,
          member.accountId,
          reason.trim(),
        );
        onClose();
        toastSuccess("Die Zwei-Faktor-Anmeldung ist zurückgesetzt.");
        await onReset();
      } catch (error) {
        logger.error("traeger_member_mfa_reset_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        void showError(error, {
          object: "das Zurücksetzen der Zwei-Faktor-Anmeldung",
        });
      } finally {
        setSaving(false);
      }
    },
    [
      member,
      reason,
      reasonValid,
      organizationId,
      onClose,
      onReset,
      clearError,
      showError,
      toastSuccess,
    ],
  );

  return (
    <Modal
      isOpen={member !== null}
      onClose={onClose}
      title="Zwei-Faktor-Anmeldung zurücksetzen"
      footer={
        <>
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            Abbrechen
          </Button>
          <Button
            type="submit"
            form="reset-traeger-mfa-form"
            variant="warning"
            size="md"
            disabled={saving || !reasonValid}
          >
            {saving ? "Wird zurückgesetzt..." : "Zurücksetzen"}
          </Button>
        </>
      }
    >
      <form
        ref={formRef}
        id="reset-traeger-mfa-form"
        onSubmit={(e) => void handleSubmit(e)}
        className="space-y-4"
      >
        <p className="text-sm text-gray-600">
          {member?.email} richtet die Zwei-Faktor-Anmeldung bei der nächsten
          Anmeldung neu ein. Das gilt für alle Portale dieses Kontos. Die
          laufende Sitzung im Träger-Portal endet.
        </p>
        <FormErrorAlert message={formErrors.error} />
        <Input
          id="reset-traeger-mfa-reason"
          label="Grund (für das Protokoll)"
          name="reason"
          error={formErrors.fieldError("reason")}
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          maxLength={500}
          required
        />
      </form>
    </Modal>
  );
}
