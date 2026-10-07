"use client";

import { useRef } from "react";

import { PasswordResetModal } from "~/components/ui/password-reset-modal";
import { useApiFormError } from "~/contexts/ToastContext";

/**
 * The staff "Passwort vergessen" dialog on the shared error path (#2517):
 * the kit modal may not import contexts, so this owner hands it the form
 * error path. Errors stay in the open dialog.
 */
export function TenantPasswordResetModal({
  isOpen,
  onClose,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
}) {
  const formRef = useRef<HTMLFormElement>(null);
  const errorPath = useApiFormError(formRef);
  return (
    <PasswordResetModal
      isOpen={isOpen}
      onClose={onClose}
      errorPath={errorPath}
      formRef={formRef}
    />
  );
}
