"use client";

import {
  useState,
  useCallback,
  useLayoutEffect,
  useRef,
  type RefObject,
} from "react";
import { Modal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import type { DatabaseFormErrorPath } from "~/components/database/database-form";
import { apiErrorFromResponse, transportFetch } from "~/lib/api-error";
import { CheckIcon, SpinnerIcon } from "~/components/ui/icons";
import { Input } from "~/components/ui/input";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "PasswordChange" });

/** Default for `mapError`: the error as it came. */
function keepError(error: unknown): unknown {
  return error;
}

interface PasswordChangeModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSuccess?: () => void;
  readonly apiEndpoint?: string;
  /**
   * The owner's shared API error path (`useApiFormError`), handed in by the
   * page (#2517). A refused change shows the
   * catalog text of its code in the dialog and marks the field the backend
   * names (`current_password`, `new_password`).
   */
  readonly errorPath: DatabaseFormErrorPath;
  /** The element `errorPath` searches for a refused field to focus. */
  readonly formRef?: RefObject<HTMLFormElement | null>;
  /**
   * Turns a refused credential check (401 with its code) into an error the
   * path shows in place, instead of a jump to the login screen.
   */
  readonly mapError?: (error: unknown) => unknown;
}

/** Prüfung vor dem Senden. Schlüssel sind die Feldnamen des Backends. */
function passwordChangeCheck(
  currentPassword: string,
  newPassword: string,
  confirmPassword: string,
): { message: string; field: string } | null {
  if (!currentPassword) {
    return {
      message: "Bitte füllen Sie alle Felder aus.",
      field: "current_password",
    };
  }
  if (!newPassword) {
    return {
      message: "Bitte füllen Sie alle Felder aus.",
      field: "new_password",
    };
  }
  if (!confirmPassword) {
    return {
      message: "Bitte füllen Sie alle Felder aus.",
      field: "confirm_password",
    };
  }
  if (newPassword !== confirmPassword) {
    return {
      message: "Die neuen Passwörter stimmen nicht überein.",
      field: "confirm_password",
    };
  }
  if (newPassword.length < 8) {
    return {
      message: "Das neue Passwort muss mindestens 8 Zeichen lang sein.",
      field: "new_password",
    };
  }
  if (currentPassword === newPassword) {
    return {
      message:
        "Das neue Passwort darf nicht mit dem aktuellen Passwort identisch sein.",
      field: "new_password",
    };
  }
  return null;
}

export function PasswordChangeModal({
  isOpen,
  onClose,
  onSuccess,
  apiEndpoint = "/api/auth/password",
  errorPath,
  formRef,
  mapError = keepError,
}: PasswordChangeModalProps) {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [success, setSuccess] = useState(false);
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const clearPathError = errorPath.clear;

  /** Whether the shared path marks this field as refused. */
  const fieldInvalid = (name: string): boolean =>
    Boolean(errorPath.fieldError(name));

  const submit = async () => {
    const check = passwordChangeCheck(
      currentPassword,
      newPassword,
      confirmPassword,
    );
    if (check) {
      errorPath.invalid(check.message, { [check.field]: check.message });
      return;
    }

    errorPath.clear();
    setIsLoading(true);

    try {
      const response = await transportFetch(apiEndpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          currentPassword,
          newPassword,
          confirmPassword,
        }),
      });

      if (!response.ok) {
        throw await apiErrorFromResponse(response, "password change failed");
      }

      setSuccess(true);
      setTimeout(() => {
        onSuccess?.();
        handleClose();
      }, 2000);
    } catch (err) {
      logger.error("password_change_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void errorPath.show(mapError(err), {
        object: "das Ändern des Passworts",
        retry: () => latestSubmitRef.current(),
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    void submit();
  };

  // „Wiederholen“ sendet den Stand, der dann im Formular steht.
  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submit();
  });

  const handleClose = useCallback(() => {
    setCurrentPassword("");
    setNewPassword("");
    setConfirmPassword("");
    clearPathError();
    setSuccess(false);
    onClose();
  }, [onClose, clearPathError]);

  return (
    <Modal isOpen={isOpen} onClose={handleClose} title="Passwort ändern">
      {success ? (
        <div className="py-8 text-center">
          <div className="bg-moto-green-fill mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full">
            <CheckIcon className="h-8 w-8 text-white" />
          </div>
          <h3 className="mb-2 text-lg font-semibold text-gray-900">
            Passwort erfolgreich geändert!
          </h3>
          <p className="text-sm text-gray-600">
            Sie werden in Kürze weitergeleitet...
          </p>
        </div>
      ) : (
        <form
          ref={formRef}
          onSubmit={handleSubmit}
          noValidate
          className="space-y-4"
        >
          <FormErrorAlert message={errorPath.error} />

          <Input
            label="Aktuelles Passwort"
            id="current-password"
            name="current_password"
            aria-invalid={fieldInvalid("current_password")}
            type="password"
            revealable
            value={currentPassword}
            onChange={(e) => setCurrentPassword(e.target.value)}
            placeholder="••••••••"
            required
          />

          <Input
            label="Neues Passwort"
            id="new-password"
            name="new_password"
            aria-invalid={fieldInvalid("new_password")}
            type="password"
            revealable
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            placeholder="••••••••"
            required
          />

          <Input
            label="Neues Passwort bestätigen"
            id="confirm-password"
            name="confirm_password"
            aria-invalid={fieldInvalid("confirm_password")}
            type="password"
            revealable
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            placeholder="••••••••"
            required
          />

          {/* Simple Password Requirements */}
          <div className="border-moto-blue/20 bg-moto-blue-soft rounded-lg border p-3">
            <h5 className="mb-2 text-sm font-medium text-gray-900">
              Passwort-Anforderungen
            </h5>
            <ul className="space-y-1 text-xs text-gray-600">
              <li>• Mindestens 8 Zeichen</li>
              <li>• Groß- und Kleinbuchstaben empfohlen</li>
              <li>• Zahlen und Sonderzeichen empfohlen</li>
            </ul>
          </div>

          {/* Action Buttons */}
          <div className="flex gap-3 pt-4">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={handleClose}
              className="flex-1"
            >
              Abbrechen
            </Button>

            <Button
              type="submit"
              size="md"
              disabled={isLoading}
              className="flex-1"
            >
              {isLoading ? (
                <span className="flex items-center justify-center gap-2">
                  <SpinnerIcon className="h-4 w-4 text-white" />
                  Wird geändert...
                </span>
              ) : (
                "Passwort ändern"
              )}
            </Button>
          </div>
        </form>
      )}
    </Modal>
  );
}
