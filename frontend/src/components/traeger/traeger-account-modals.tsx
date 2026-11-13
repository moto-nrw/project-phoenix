"use client";

import { useEffect, useRef, useState } from "react";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { Input } from "~/components/ui/input";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import type { RefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import {
  changeTraegerMemberName,
  changeTraegerRoles,
  inviteTraegerAccount,
  type TraegerMember,
} from "~/lib/traeger-api";
import {
  MEMBER_NAME_REQUIRED,
  TRAEGER_NAME_HINT,
  TRAEGER_NAME_MAX_LENGTH,
} from "~/lib/traeger-person-name";
import type { TraegerRole } from "~/lib/traeger-roles";
import { TraegerRoleChoice } from "./traeger-role-choice";

const logger = createLogger({ component: "TraegerAccountModals" });

const ROLES_REQUIRED = "Bitte wählen Sie mindestens eine Rolle.";

/** What the invitation sends: the address and at least one role. */
export interface TraegerInvitationInput {
  readonly email: string;
  readonly roles: readonly TraegerRole[];
}

const TRAEGER_INVITATION_HINT =
  "Die Person bekommt einen Link per E-Mail. Bis wann er gilt, steht danach bei den offenen Einladungen.";

/**
 * Invites a person into the Träger-Büro by e-mail (#4650). The person gets a
 * link and chooses a password, or confirms the one of their moto account.
 * The operator portal sends the same invitation through its own route
 * (#5144): it passes `invite` and its own texts.
 */
export function InviteTraegerAccountModal({
  isOpen,
  onClose,
  onInvited,
  invite = inviteTraegerAccount,
  title = "Person einladen",
  hint = TRAEGER_INVITATION_HINT,
  successText = "Die Einladung ist verschickt.",
}: Readonly<{
  isOpen: boolean;
  onClose: () => void;
  /** Reloads the list; never rejects (#6652). */
  onInvited: RefreshAfterWrite;
  invite?: (input: TraegerInvitationInput) => Promise<unknown>;
  title?: string;
  hint?: string;
  successText?: string;
}>) {
  const [email, setEmail] = useState("");
  const [roles, setRoles] = useState<TraegerRole[]>([]);
  const save = useFormModalPending(isOpen);
  const saving = save.pending;
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
    await save.run(async (op) => {
      try {
        await invite({ email: email.trim(), roles });
      } catch (err) {
        logger.error("traeger_invitation_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!op.isCurrent()) return;
        void formErrors.show(err, { object: "die Einladung" });
        return;
      }
      if (op.isCurrent()) onClose();
      success(successText);
      await onInvited();
    });
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      pending={save}
      isBackdropDismissDisabled
      title={title}
      size="lg"
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
        <p className="text-sm text-gray-600">{hint}</p>
        <Input
          controlSize="touch"
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

const LAST_LEADER_REASON =
  "Nur diese Person hat die Rolle Leitung. Geben Sie die Rolle zuerst einer anderen Person.";

const LAST_LEADER_LOCK = { leadership: LAST_LEADER_REASON } as const;

/**
 * Changes the roles of a person of the Träger-Büro (#4650). The person's
 * open session picks the new rights up within a few minutes. Leitung stays
 * checked for the only person holding it (#5471); the backend refuses the
 * change as well.
 */
export function ChangeTraegerRolesModal({
  member,
  members,
  onClose,
  onChanged,
}: Readonly<{
  member: TraegerMember | null;
  members: readonly TraegerMember[];
  onClose: () => void;
  /** Reloads the list; never rejects (#6652). */
  onChanged: RefreshAfterWrite;
}>) {
  const isLastLeader =
    member?.roles.includes("leadership") === true &&
    members.filter((other) => other.roles.includes("leadership")).length <= 1;
  const [roles, setRoles] = useState<TraegerRole[]>([]);
  const save = useFormModalPending(member !== null);
  const saving = save.pending;
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
    await save.run(async (op) => {
      try {
        await changeTraegerRoles(member.accountId, roles);
      } catch (err) {
        logger.error("traeger_roles_change_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!op.isCurrent()) return;
        void formErrors.show(err, { object: "die Rollenvergabe" });
        return;
      }
      if (op.isCurrent()) onClose();
      success("Die Rollen sind geändert.");
      await onChanged();
    });
  };

  return (
    <FormModal
      isOpen={member !== null}
      onClose={onClose}
      pending={save}
      title="Rollen ändern"
      size="lg"
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
          Rollen für{" "}
          <span className="font-medium wrap-anywhere">{member?.email}</span>.
          Die Änderung gilt nach wenigen Minuten, spätestens bei der nächsten
          Anmeldung.
        </p>
        <TraegerRoleChoice
          value={roles}
          onChange={setRoles}
          disabled={saving}
          error={formErrors.fieldError("roles")}
          locked={isLastLeader ? LAST_LEADER_LOCK : undefined}
        />
      </form>
    </FormModal>
  );
}

/**
 * Enters or changes the name of a person of the Träger-Büro (#4693). The
 * export protocol and the history of every invoice show it. `onSaved` gets
 * the person as the backend keeps it, so the row shows the name at once.
 */
export function ChangeTraegerMemberNameModal({
  member,
  onClose,
  onSaved,
  onChanged,
}: Readonly<{
  member: TraegerMember | null;
  onClose: () => void;
  onSaved: (member: TraegerMember) => void;
  /** Reloads the list and the history; never rejects (#6652). */
  onChanged: RefreshAfterWrite;
}>) {
  const [name, setName] = useState("");
  const save = useFormModalPending(member !== null);
  const saving = save.pending;
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!member) return;
    setName(member.name);
    clearErrors();
  }, [member, clearErrors]);

  const submit = async () => {
    if (!member) return;
    formErrors.clear();
    const trimmed = name.trim();
    if (!trimmed) {
      formErrors.invalid(MEMBER_NAME_REQUIRED, { name: MEMBER_NAME_REQUIRED });
      return;
    }
    await save.run(async (op) => {
      let saved: TraegerMember;
      try {
        saved = await changeTraegerMemberName(member.accountId, trimmed);
      } catch (err) {
        logger.error("traeger_member_name_change_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!op.isCurrent()) return;
        void formErrors.show(err, { object: "der Name" });
        return;
      }
      onSaved(saved);
      if (op.isCurrent()) onClose();
      success("Der Name ist gespeichert.");
      await onChanged();
    });
  };

  const title = member?.name ? "Namen ändern" : "Namen eintragen";

  return (
    <FormModal
      isOpen={member !== null}
      onClose={onClose}
      pending={save}
      title={title}
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
        id="change-traeger-member-name-form"
        noValidate
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <p className="text-sm text-gray-600">
          Name für{" "}
          <span className="font-medium wrap-anywhere">{member?.email}</span>.
        </p>
        <Input
          controlSize="compact"
          id="change-traeger-member-name"
          label="Name"
          name="name"
          autoComplete="off"
          value={name}
          onChange={(e) => setName(e.target.value)}
          error={formErrors.fieldError("name")}
          hint={TRAEGER_NAME_HINT}
          maxLength={TRAEGER_NAME_MAX_LENGTH}
          disabled={saving}
          required
        />
      </form>
    </FormModal>
  );
}
