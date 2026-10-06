"use client";

import {
  useState,
  useCallback,
  useLayoutEffect,
  useRef,
  type RefObject,
} from "react";
import { Modal } from "./modal";
import { Alert } from "./alert";
import { FormErrorAlert } from "./form-error-alert";
import type { DatabaseFormErrorPath } from "./database/database-form";
import { apiErrorFromResponse, transportFetch } from "~/lib/api-error";
import { EyeIcon, EyeOffIcon, CheckIcon, SpinnerIcon } from "./icons";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "PasswordChange" });

// Legacy texts for owners without `errorPath` (operator settings); the
// shared path shows the catalog text of the code instead (#2517).
const ERROR_MAPPINGS: Array<{
  test: (msg: string) => boolean;
  message: string;
}> = [
  {
    test: (msg) => msg.includes("invalid") && msg.includes("password"),
    message: "Das aktuelle Passwort ist falsch. Bitte versuchen Sie es erneut.",
  },
  {
    test: (msg) => msg.includes("too weak") || msg.includes("complexity"),
    message:
      "Das neue Passwort ist zu schwach. Verwenden Sie mindestens 8 Zeichen mit Groß-/Kleinbuchstaben, Zahlen und Sonderzeichen.",
  },
  {
    test: (msg) => msg.includes("not found"),
    message:
      "Ihr Konto konnte nicht gefunden werden. Bitte melden Sie sich erneut an.",
  },
];

function mapBackendError(backendError: string): string {
  const match = ERROR_MAPPINGS.find((m) => m.test(backendError));
  return (
    match?.message ??
    "Passwortänderung fehlgeschlagen. Bitte versuchen Sie es später erneut."
  );
}

interface PasswordToggleProps {
  readonly show: boolean;
  readonly onToggle: () => void;
}

// Extracted component to avoid re-creating on each parent render
function PasswordToggle({ show, onToggle }: PasswordToggleProps) {
  return (
    <button
      type="button"
      aria-label={show ? "Passwort ausblenden" : "Passwort anzeigen"}
      onClick={onToggle}
      className="absolute top-1/2 right-3 -translate-y-1/2 p-1 text-gray-500 hover:text-gray-700"
    >
      {show ? <EyeOffIcon /> : <EyeIcon />}
    </button>
  );
}

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
   * The owner's shared API error path (`useApiFormError`), handed in because
   * the kit may not import contexts (#2517). With it, a refused change shows
   * the catalog text of its code in the dialog and marks the field the
   * backend names (`current_password`, `new_password`).
   */
  readonly errorPath?: DatabaseFormErrorPath;
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
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [errorFieldName, setErrorFieldName] = useState<string | null>(null);
  const errorRef = useScrollToError(error);
  const [showCurrentPassword, setShowCurrentPassword] = useState(false);
  const [showNewPassword, setShowNewPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const clearPathError = errorPath?.clear;

  /** The refusal of a field, from the shared path or the legacy state. */
  const fieldInvalid = (name: string): boolean =>
    errorPath ? Boolean(errorPath.fieldError(name)) : errorFieldName === name;

  const submit = async () => {
    setError(null);
    setErrorFieldName(null);

    const check = passwordChangeCheck(
      currentPassword,
      newPassword,
      confirmPassword,
    );
    if (check) {
      if (errorPath) {
        errorPath.invalid(check.message, { [check.field]: check.message });
      } else {
        setError(check.message);
        setErrorFieldName(check.field);
      }
      return;
    }

    errorPath?.clear();
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
        if (errorPath) {
          throw await apiErrorFromResponse(response, "password change failed");
        }
        const data = (await response.json()) as { error?: string };
        throw new Error(mapBackendError(data.error ?? ""));
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
      if (errorPath) {
        void errorPath.show(mapError(err), {
          object: "das Ändern des Passworts",
          retry: () => latestSubmitRef.current(),
        });
      } else {
        setError(
          err instanceof Error
            ? err.message
            : "Ein unerwarteter Fehler ist aufgetreten.",
        );
      }
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
    setError(null);
    setErrorFieldName(null);
    clearPathError?.();
    setSuccess(false);
    setShowCurrentPassword(false);
    setShowNewPassword(false);
    setShowConfirmPassword(false);
    onClose();
  }, [onClose, clearPathError]);

  return (
    <Modal isOpen={isOpen} onClose={handleClose} title="Passwort ändern">
      {success ? (
        <div className="py-8 text-center">
          <div className="bg-moto-green mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full">
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
          {errorPath ? (
            <FormErrorAlert message={errorPath.error} />
          ) : (
            error && (
              <div ref={errorRef}>
                <Alert type="error" message={error} />
              </div>
            )
          )}

          {/* Current Password */}
          <div>
            <label
              htmlFor="current-password"
              className={`mb-1 block text-sm font-medium ${fieldInvalid("current_password") ? "text-moto-red" : "text-gray-700"}`}
            >
              Aktuelles Passwort
            </label>
            <div className="relative">
              <input
                id="current-password"
                name="current_password"
                aria-invalid={fieldInvalid("current_password")}
                type={showCurrentPassword ? "text" : "password"}
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                placeholder="••••••••"
                required
                className={`block w-full rounded-lg border ${fieldInvalid("current_password") ? "border-moto-red/40" : "border-gray-200"} focus:border-moto-blue focus:ring-moto-blue bg-white px-4 py-3 pr-12 text-base text-gray-900 transition-colors placeholder:text-gray-400 focus:ring-1`}
              />
              <PasswordToggle
                show={showCurrentPassword}
                onToggle={() => setShowCurrentPassword(!showCurrentPassword)}
              />
            </div>
          </div>

          {/* New Password */}
          <div>
            <label
              htmlFor="new-password"
              className={`mb-1 block text-sm font-medium ${fieldInvalid("new_password") ? "text-moto-red" : "text-gray-700"}`}
            >
              Neues Passwort
            </label>
            <div className="relative">
              <input
                id="new-password"
                name="new_password"
                aria-invalid={fieldInvalid("new_password")}
                type={showNewPassword ? "text" : "password"}
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="••••••••"
                required
                className={`block w-full rounded-lg border ${fieldInvalid("new_password") ? "border-moto-red/40" : "border-gray-200"} focus:border-moto-blue focus:ring-moto-blue bg-white px-4 py-3 pr-12 text-base text-gray-900 transition-colors placeholder:text-gray-400 focus:ring-1`}
              />
              <PasswordToggle
                show={showNewPassword}
                onToggle={() => setShowNewPassword(!showNewPassword)}
              />
            </div>
          </div>

          {/* Confirm Password */}
          <div>
            <label
              htmlFor="confirm-password"
              className={`mb-1 block text-sm font-medium ${fieldInvalid("confirm_password") ? "text-moto-red" : "text-gray-700"}`}
            >
              Neues Passwort bestätigen
            </label>
            <div className="relative">
              <input
                id="confirm-password"
                name="confirm_password"
                aria-invalid={fieldInvalid("confirm_password")}
                type={showConfirmPassword ? "text" : "password"}
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="••••••••"
                required
                className={`block w-full rounded-lg border ${fieldInvalid("confirm_password") ? "border-moto-red/40" : "border-gray-200"} focus:border-moto-blue focus:ring-moto-blue bg-white px-4 py-3 pr-12 text-base text-gray-900 transition-colors placeholder:text-gray-400 focus:ring-1`}
              />
              <PasswordToggle
                show={showConfirmPassword}
                onToggle={() => setShowConfirmPassword(!showConfirmPassword)}
              />
            </div>
          </div>

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
            <button
              type="button"
              onClick={handleClose}
              className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100"
            >
              Abbrechen
            </button>

            <button
              type="submit"
              disabled={isLoading}
              className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-gray-700 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:scale-100"
            >
              {isLoading ? (
                <span className="flex items-center justify-center gap-2">
                  <SpinnerIcon className="h-4 w-4 text-white" />
                  Wird geändert...
                </span>
              ) : (
                "Passwort ändern"
              )}
            </button>
          </div>
        </form>
      )}
    </Modal>
  );
}
