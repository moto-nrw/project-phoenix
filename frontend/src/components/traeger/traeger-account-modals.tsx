"use client";

import { useEffect, useRef, useState } from "react";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  changeTraegerRoles,
  inviteTraegerAccount,
  type TraegerMember,
} from "~/lib/traeger-api";
import type { TraegerRole } from "~/lib/traeger-roles";
import { TraegerRoleChoice } from "./traeger-role-choice";

const logger = createLogger({ component: "TraegerAccountModals" });

const ROLES_REQUIRED = "Bitte wählen Sie mindestens eine Rolle.";

/**
 * Invites a person into the Träger-Büro by e-mail (#4650). The person gets a
 * link and chooses a password, or confirms the one of their moto account.
 */
export function InviteTraegerAccountModal({
  isOpen,
  onClose,
  onInvited,
}: Readonly<{
  isOpen: boolean;
  onClose: () => void;
  onInvited: () => Promise<unknown>;
}>) {
  const [email, setEmail] = useState("");
  const [roles, setRoles] = useState<TraegerRole[]>([]);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!isOpen) return;
    setEmail("");
    setRoles([]);
    clearErrors();
  }, [isOpen, clearErrors]);

  const submit = async () => {
    formErrors.clear();
    const fields: Record<string, string> = {};
    if (!email.trim()) fields.email = "Bitte geben Sie eine E-Mail-Adresse an.";
    if (roles.length === 0) fields.roles = ROLES_REQUIRED;
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", fields);
      return;
    }
    setSaving(true);
    try {
      await inviteTraegerAccount({ email: email.trim(), roles });
      onClose();
      success("Die Einladung ist verschickt.");
      await onInvited();
    } catch (err) {
      logger.error("traeger_invitation_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Einladung" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title="Person einladen"
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Einladung senden"
          savingLabel="Wird gesendet …"
        />
      }
    >
      <form
        ref={formRef}
        id="invite-traeger-account-form"
        noValidate
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <p className="text-sm text-gray-600">
          Die Person bekommt eine E-Mail mit einem Link zum Träger-Portal. Bis
          wann der Link gilt, steht danach bei den offenen Einladungen.
        </p>
        <Input
          id="invite-traeger-account-email"
          label="E-Mail-Adresse"
          name="email"
          type="email"
          autoComplete="off"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          error={formErrors.fieldError("email")}
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
    </FormModal>
  );
}

/**
 * Changes the roles of a person of the Träger-Büro (#4650). The person's
 * open session picks the new rights up within a few minutes.
 */
export function ChangeTraegerRolesModal({
  member,
  onClose,
  onChanged,
}: Readonly<{
  member: TraegerMember | null;
  onClose: () => void;
  onChanged: () => Promise<unknown>;
}>) {
  const [roles, setRoles] = useState<TraegerRole[]>([]);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!member) return;
    setRoles([...member.roles]);
    clearErrors();
  }, [member, clearErrors]);

  const submit = async () => {
    if (!member) return;
    formErrors.clear();
    if (roles.length === 0) {
      formErrors.invalid(ROLES_REQUIRED, { roles: ROLES_REQUIRED });
      return;
    }
    setSaving(true);
    try {
      await changeTraegerRoles(member.accountId, roles);
      onClose();
      success("Die Rollen sind geändert.");
      await onChanged();
    } catch (err) {
      logger.error("traeger_roles_change_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Rollen" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={member !== null}
      onClose={onClose}
      closeDisabled={saving}
      title="Rollen ändern"
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
        id="change-traeger-roles-form"
        noValidate
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <p className="text-sm text-gray-600">
          Rollen für <span className="font-medium">{member?.email}</span>. Die
          Änderung gilt nach wenigen Minuten, spätestens bei der nächsten
          Anmeldung.
        </p>
        <TraegerRoleChoice
          value={roles}
          onChange={setRoles}
          disabled={saving}
          error={formErrors.fieldError("roles")}
        />
      </form>
    </FormModal>
  );
}
