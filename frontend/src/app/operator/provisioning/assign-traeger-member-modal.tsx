import { useCallback, useEffect, useRef, useState } from "react";
import { Modal } from "~/components/ui/modal";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import {
  TRAEGER_ROLES,
  TRAEGER_ROLE_LABELS,
  type TraegerRole,
} from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";
import { FormField, SelectWithChevron } from "./provisioning-shared";

const logger = createLogger({ component: "AssignTraegerMemberModal" });

/**
 * Assigns an existing account to the Träger-Büro with one Träger role
 * (#4649). The person signs in at the Träger-Portal with the same account;
 * an e-mail without an account answers `identity.account_not_found`.
 */
export function AssignTraegerMemberModal({
  isOpen,
  onClose,
  organizationId,
  organizationName,
  onAssigned,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly organizationId: string;
  readonly organizationName: string;
  readonly onAssigned: () => Promise<unknown>;
}) {
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<TraegerRole>("administration");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, clear: clearError } = formErrors;
  const { success: toastSuccess } = useToast();

  useEffect(() => {
    if (isOpen) {
      setEmail("");
      setRole("administration");
      clearError();
    }
  }, [isOpen, clearError]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!email.trim()) return;
      setSaving(true);
      clearError();
      try {
        await operatorProvisioningService.assignTraegerMember(organizationId, {
          email: email.trim(),
          role,
        });
        onClose();
        toastSuccess(`Die Person ist dem Träger-Büro zugeordnet.`);
        await onAssigned();
      } catch (error) {
        logger.error("traeger_member_assign_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        void showError(error, { object: "die Zuordnung zum Träger-Büro" });
      } finally {
        setSaving(false);
      }
    },
    [
      email,
      role,
      organizationId,
      onClose,
      onAssigned,
      clearError,
      showError,
      toastSuccess,
    ],
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Person dem Träger-Büro zuordnen"
      footer={
        <>
          <button
            type="button"
            onClick={onClose}
            className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
          >
            Abbrechen
          </button>
          <button
            type="submit"
            form="assign-traeger-member-form"
            disabled={saving || !email.trim()}
            className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {saving ? "Wird zugeordnet..." : "Zuordnen"}
          </button>
        </>
      }
    >
      <form
        ref={formRef}
        id="assign-traeger-member-form"
        onSubmit={(e) => void handleSubmit(e)}
        className="space-y-4"
      >
        <p className="text-sm text-gray-600">
          Die Person meldet sich mit ihrem bestehenden moto-Konto im
          Träger-Portal von {organizationName} an.
        </p>
        <FormErrorAlert message={formErrors.error} />
        <FormField
          label="E-Mail des Kontos"
          htmlFor="assign-traeger-member-email"
          required
          error={formErrors.fieldError("email")}
        >
          <input
            id="assign-traeger-member-email"
            name="email"
            type="email"
            autoComplete="off"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            maxLength={255}
            className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
            required
          />
        </FormField>
        <FormField
          label="Rolle"
          htmlFor="assign-traeger-member-role"
          required
          error={formErrors.fieldError("role")}
        >
          <SelectWithChevron
            id="assign-traeger-member-role"
            name="role"
            value={role}
            onChange={(e) => setRole(e.target.value as TraegerRole)}
            required
          >
            {TRAEGER_ROLES.map((option) => (
              <option key={option} value={option}>
                {TRAEGER_ROLE_LABELS[option]}
              </option>
            ))}
          </SelectWithChevron>
        </FormField>
      </form>
    </Modal>
  );
}
