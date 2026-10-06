"use client";

import { useRef } from "react";

import { PasswordChangeModal } from "~/components/ui/password-change-modal";
import { useApiFormError } from "~/contexts/ToastContext";
import { credentialError } from "./credential-error";

const CREDENTIAL_REFUSALS = ["identity.current_password_wrong"] as const;

/** Keeps a refused current password in the dialog instead of logging out. */
function refusedCurrentPassword(error: unknown): unknown {
  return credentialError(error, CREDENTIAL_REFUSALS);
}

/**
 * "Passwort ändern" on the shared error path (#2517): the kit modal may not
 * import contexts, so this owner hands it the form error path. A wrong
 * current password answers 401 with its own code; that marks the field
 * instead of ending the session. Any other 401 still leads to the login.
 */
export function TenantPasswordChangeModal({
  isOpen,
  onClose,
  onSuccess,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSuccess?: () => void;
}) {
  const formRef = useRef<HTMLFormElement>(null);
  const errorPath = useApiFormError(formRef);
  return (
    <PasswordChangeModal
      isOpen={isOpen}
      onClose={onClose}
      onSuccess={onSuccess}
      errorPath={errorPath}
      formRef={formRef}
      mapError={refusedCurrentPassword}
    />
  );
}
