import { useState, useCallback, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { SchoolAccount } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";
import { FormField, FormError } from "./provisioning-shared";

const logger = createLogger({ component: "CreateAccountModal" });

export function CreateAccountModal({
  isOpen,
  onClose,
  schoolId,
  schoolName,
  onCreated,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly schoolId: string | null;
  readonly schoolName: string;
  readonly onCreated: () => void;
}) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const errorRef = useScrollToError(error);

  const [result, setResult] = useState<SchoolAccount | null>(null);

  // Reset form when opening
  useEffect(() => {
    if (isOpen) {
      setEmail("");
      setPassword("");
      setConfirmPassword("");
      setError("");
      setResult(null);
    }
  }, [isOpen]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!schoolId || !email.trim() || !password) return;

      if (password !== confirmPassword) {
        setError("Passwörter stimmen nicht überein.");
        return;
      }

      setSaving(true);
      setError("");
      try {
        const created = await operatorProvisioningService.createSchoolAccount(
          schoolId,
          {
            email: email.trim(),
            password,
            confirm_password: confirmPassword,
          },
        );
        setResult(created);
        onCreated();
      } catch (err) {
        setError(
          err instanceof Error
            ? err.message
            : "Fehler beim Erstellen des Kontos.",
        );
        logger.error("account_create_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      } finally {
        setSaving(false);
      }
    },
    [schoolId, email, password, confirmPassword, onCreated],
  );

  const handleClose = useCallback(() => {
    onClose();
    setResult(null);
  }, [onClose]);

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title={`Konto erstellen — ${schoolName}`}
      footer={
        result ? (
          <button
            type="button"
            onClick={handleClose}
            className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800"
          >
            Schließen
          </button>
        ) : (
          <>
            <button
              type="button"
              onClick={handleClose}
              className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
            >
              Abbrechen
            </button>
            <button
              type="button"
              onClick={(e) => void handleSubmit(e)}
              disabled={
                saving || !email.trim() || !password || !confirmPassword
              }
              className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {saving ? "Wird erstellt..." : "Konto erstellen"}
            </button>
          </>
        )
      }
    >
      {result ? (
        <div className="space-y-3">
          <div className="flex items-center gap-2 rounded-lg bg-green-50 px-4 py-3">
            <svg
              className="h-5 w-5 text-green-600"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <span className="text-sm font-medium text-green-800">
              Konto erstellt
            </span>
          </div>
          <div className="space-y-2 text-sm text-gray-600">
            <p>
              <span className="font-medium">E-Mail:</span> {result.email}
            </p>
          </div>
        </div>
      ) : (
        <form
          onSubmit={(e) => void handleSubmit(e)}
          className="space-y-4"
          id="create-account-form"
        >
          <FormField label="E-Mail" htmlFor="create-account-email" required>
            <input
              id="create-account-email"
              type="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              maxLength={255}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              required
            />
          </FormField>
          <FormField
            label="Passwort"
            htmlFor="create-account-password"
            required
          >
            <input
              id="create-account-password"
              type="password"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              maxLength={255}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              required
            />
          </FormField>
          <FormField
            label="Passwort bestätigen"
            htmlFor="create-account-confirm-password"
            required
          >
            <input
              id="create-account-confirm-password"
              type="password"
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              maxLength={255}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              required
            />
          </FormField>
          {error && <FormError ref={errorRef} message={error} />}
        </form>
      )}
    </Modal>
  );
}
