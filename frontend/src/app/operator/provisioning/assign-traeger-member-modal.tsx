import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { TraegerRoleChoice } from "~/components/traeger/traeger-role-choice";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { TraegerRole } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AssignTraegerMemberModal" });

const ROLES_REQUIRED = "Bitte wählen Sie mindestens eine Rolle.";

/**
 * Assigns an existing account to the Träger-Büro with one or more Träger
 * roles (#4649, #5144). The person signs in at the Träger-Portal with the
 * same account; an e-mail without an account answers
 * `identity.account_not_found`, so the text points to the invitation.
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
  const [roles, setRoles] = useState<TraegerRole[]>([]);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, clear: clearError, invalid } = formErrors;
  const { success: toastSuccess } = useToast();

  useEffect(() => {
    if (isOpen) {
      setEmail("");
      setRoles([]);
      clearError();
    }
  }, [isOpen, clearError]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!email.trim()) return;
      clearError();
      if (roles.length === 0) {
        invalid(ROLES_REQUIRED, { roles: ROLES_REQUIRED });
        return;
      }
      setSaving(true);
      try {
        await operatorProvisioningService.assignTraegerMember(organizationId, {
          email: email.trim(),
          roles,
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
      roles,
      organizationId,
      onClose,
      onAssigned,
      clearError,
      invalid,
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
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            Abbrechen
          </Button>
          <Button
            type="submit"
            form="assign-traeger-member-form"
            variant="primary"
            size="md"
            disabled={saving || !email.trim()}
          >
            {saving ? "Wird zugeordnet..." : "Zuordnen"}
          </Button>
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
          Nur für Personen mit moto-Konto. Sie melden sich mit diesem Konto im
          Träger-Portal von {organizationName} an. Ohne Konto: „Person
          einladen“.
        </p>
        <FormErrorAlert message={formErrors.error} />
        <Input
          id="assign-traeger-member-email"
          label="E-Mail des Kontos"
          name="email"
          error={formErrors.fieldError("email")}
          type="email"
          autoComplete="off"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          maxLength={255}
          required
        />
        <TraegerRoleChoice
          value={roles}
          onChange={setRoles}
          disabled={saving}
          error={formErrors.fieldError("roles")}
        />
      </form>
    </Modal>
  );
}
