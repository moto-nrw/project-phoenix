"use client";

import { useState, useCallback } from "react";
import { Modal } from "~/components/ui/modal";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import {
  FormField,
  FormError,
} from "~/app/operator/provisioning/provisioning-shared";
import { isOperatorApiError } from "~/lib/operator/api-helpers";
import { createOperatorInvitation } from "~/lib/operator/invitation-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "InviteOperatorModal" });

interface InviteOperatorModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSuccess: () => void;
}

export function InviteOperatorModal({
  isOpen,
  onClose,
  onSuccess,
}: InviteOperatorModalProps) {
  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const errorRef = useScrollToError(error);

  const resetForm = useCallback(() => {
    setEmail("");
    setDisplayName("");
    setError("");
  }, []);

  const handleClose = useCallback(() => {
    onClose();
    resetForm();
  }, [onClose, resetForm]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!email.trim()) return;
      setSaving(true);
      setError("");
      try {
        await createOperatorInvitation({
          email: email.trim(),
          display_name: displayName.trim() || undefined,
        });
        handleClose();
        onSuccess();
      } catch (err) {
        if (isOperatorApiError(err) && err.status === 409) {
          setError("Diese E-Mail-Adresse wird bereits verwendet.");
        } else {
          setError(
            err instanceof Error ? err.message : "Fehler beim Einladen.",
          );
          logger.error("invite_operator_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
        }
      } finally {
        setSaving(false);
      }
    },
    [email, displayName, handleClose, onSuccess],
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title="Operator einladen"
      footer={
        <>
          <button
            type="button"
            onClick={handleClose}
            className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
          >
            Abbrechen
          </button>
          <button
            type="submit"
            form="invite-operator-form"
            disabled={saving || !email.trim()}
            className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {saving ? "Wird gesendet..." : "Einladung senden"}
          </button>
        </>
      }
    >
      <form
        onSubmit={(e) => void handleSubmit(e)}
        className="space-y-4"
        id="invite-operator-form"
      >
        <FormField label="E-Mail-Adresse" htmlFor="invite-email" required>
          <input
            id="invite-email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            maxLength={255}
            className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
            placeholder="operator@example.com"
            required
          />
        </FormField>
        <FormField label="Anzeigename" htmlFor="invite-display-name">
          <input
            id="invite-display-name"
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            maxLength={100}
            className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
            placeholder="Max Mustermann"
          />
          <p className="mt-1 text-xs text-gray-500">
            Optional (kann beim Annehmen der Einladung gesetzt werden)
          </p>
        </FormField>
        {error && <FormError ref={errorRef} message={error} />}
      </form>
    </Modal>
  );
}
