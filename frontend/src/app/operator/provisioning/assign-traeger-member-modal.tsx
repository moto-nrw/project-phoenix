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

const logger = createLogger({ component: "TraegerMemberModal" });

const ROLES_REQUIRED = "Bitte wählen Sie mindestens eine Rolle.";

/** What differs between assigning an existing account and inviting. */
interface TraegerMemberMode {
  readonly formId: string;
  readonly title: string;
  readonly emailLabel: string;
  readonly submitLabel: string;
  readonly savingLabel: string;
  readonly successText: string;
  readonly errorObject: string;
  readonly logEvent: string;
  readonly explanation: (organizationName: string) => string;
  readonly send: (
    organizationId: string,
    data: { email: string; roles: TraegerRole[] },
  ) => Promise<unknown>;
}

const ASSIGN: TraegerMemberMode = {
  formId: "assign-traeger-member-form",
  title: "Person dem Träger-Büro zuordnen",
  emailLabel: "E-Mail des Kontos",
  submitLabel: "Zuordnen",
  savingLabel: "Wird zugeordnet...",
  successText: "Die Person ist dem Träger-Büro zugeordnet.",
  errorObject: "die Zuordnung zum Träger-Büro",
  logEvent: "traeger_member_assign_failed",
  explanation: (name) =>
    `Nur für Personen mit moto-Konto. Sie melden sich mit diesem Konto im Träger-Portal von ${name} an. Ohne Konto: „Person einladen“.`,
  send: (organizationId, data) =>
    operatorProvisioningService.assignTraegerMember(organizationId, data),
};

const INVITE: TraegerMemberMode = {
  formId: "invite-traeger-member-form",
  title: "Person ins Träger-Büro einladen",
  emailLabel: "E-Mail-Adresse",
  submitLabel: "Einladung senden",
  savingLabel: "Wird gesendet...",
  successText:
    "Die Einladung ist verschickt. Das Träger-Büro sieht sie bei den offenen Einladungen.",
  errorObject: "die Einladung ins Träger-Büro",
  logEvent: "traeger_member_invite_failed",
  explanation: (name) =>
    `Die Person bekommt eine E-Mail mit einem Link zum Träger-Portal von ${name}. Dort legt sie ein Passwort fest oder meldet sich mit ihrem moto-Konto an. Ein Schulkonto braucht sie nicht.`,
  send: (organizationId, data) =>
    operatorProvisioningService.inviteTraegerMember(organizationId, data),
};

interface TraegerMemberModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly organizationId: string;
  readonly organizationName: string;
  readonly onDone: () => Promise<unknown>;
}

/**
 * Adds a person to the Träger-Büro with one or more Träger roles (#4649,
 * #5144). The first person usually needs Verwaltung, so it is preselected.
 */
function TraegerMemberModal({
  mode,
  isOpen,
  onClose,
  organizationId,
  organizationName,
  onDone,
}: TraegerMemberModalProps & { readonly mode: TraegerMemberMode }) {
  const [email, setEmail] = useState("");
  const [roles, setRoles] = useState<TraegerRole[]>(["administration"]);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, clear: clearError, invalid } = formErrors;
  const { success: toastSuccess } = useToast();

  useEffect(() => {
    if (isOpen) {
      setEmail("");
      setRoles(["administration"]);
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
        await mode.send(organizationId, { email: email.trim(), roles });
        onClose();
        toastSuccess(mode.successText);
        await onDone();
      } catch (error) {
        logger.error(mode.logEvent, {
          error: error instanceof Error ? error.message : String(error),
        });
        void showError(error, { object: mode.errorObject });
      } finally {
        setSaving(false);
      }
    },
    [
      mode,
      email,
      roles,
      organizationId,
      onClose,
      onDone,
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
      title={mode.title}
      footer={
        <>
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            Abbrechen
          </Button>
          <Button
            type="submit"
            form={mode.formId}
            variant="primary"
            size="md"
            disabled={saving || !email.trim()}
          >
            {saving ? mode.savingLabel : mode.submitLabel}
          </Button>
        </>
      }
    >
      <form
        ref={formRef}
        id={mode.formId}
        onSubmit={(e) => void handleSubmit(e)}
        className="space-y-4"
      >
        <p className="text-sm text-gray-600">
          {mode.explanation(organizationName)}
        </p>
        <FormErrorAlert message={formErrors.error} />
        <Input
          id={`${mode.formId}-email`}
          label={mode.emailLabel}
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

/**
 * Assigns an existing account to the Träger-Büro (#4649). The person signs
 * in at the Träger-Portal with the same account; an e-mail without an
 * account answers `identity.account_not_found`.
 */
export function AssignTraegerMemberModal(props: TraegerMemberModalProps) {
  return <TraegerMemberModal mode={ASSIGN} {...props} />;
}

/**
 * Invites a person into the Träger-Büro with the Träger-Büro's own
 * invitation (#5144): the mail links to the Träger-Portal, where the person
 * sets a password or confirms an existing moto account.
 */
export function InviteTraegerMemberModal(props: TraegerMemberModalProps) {
  return <TraegerMemberModal mode={INVITE} {...props} />;
}
