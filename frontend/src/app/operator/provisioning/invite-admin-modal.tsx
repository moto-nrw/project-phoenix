import { useState, useCallback, useEffect, useRef } from "react";
import { Modal } from "~/components/ui/modal";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { Invitation } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";
import {
  FormField,
  FormError,
  DeliveryStatusBadge,
} from "./provisioning-shared";

const logger = createLogger({ component: "InviteAdminModal" });

export function InviteAdminModal({
  isOpen,
  onClose,
  schoolId,
  schoolName,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly schoolId: string | null;
  readonly schoolName: string;
}) {
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteFirstName, setInviteFirstName] = useState("");
  const [inviteLastName, setInviteLastName] = useState("");
  const [inviteSaving, setInviteSaving] = useState(false);
  const [inviteError, setInviteError] = useState("");
  const errorRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (inviteError && errorRef.current) {
      errorRef.current.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  }, [inviteError]);

  const [inviteResult, setInviteResult] = useState<Invitation | null>(null);

  // Reset form when opening
  useEffect(() => {
    if (isOpen) {
      setInviteEmail("");
      setInviteFirstName("");
      setInviteLastName("");
      setInviteError("");
      setInviteResult(null);
    }
  }, [isOpen]);

  const handleInvite = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!schoolId || !inviteEmail.trim()) return;
      setInviteSaving(true);
      setInviteError("");
      try {
        const result = await operatorProvisioningService.inviteSchoolAdmin(
          schoolId,
          {
            email: inviteEmail.trim(),
            ...(inviteFirstName && { first_name: inviteFirstName.trim() }),
            ...(inviteLastName && { last_name: inviteLastName.trim() }),
          },
        );
        setInviteResult(result);
      } catch (error) {
        setInviteError(
          error instanceof Error ? error.message : "Fehler beim Einladen.",
        );
        logger.error("admin_invite_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
      } finally {
        setInviteSaving(false);
      }
    },
    [schoolId, inviteEmail, inviteFirstName, inviteLastName],
  );

  const handleClose = useCallback(() => {
    onClose();
    setInviteResult(null);
  }, [onClose]);

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title={`Admin einladen — ${schoolName}`}
      footer={
        inviteResult ? (
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
              onClick={(e) => void handleInvite(e)}
              disabled={inviteSaving || !inviteEmail.trim()}
              className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {inviteSaving ? "Wird gesendet..." : "Einladung senden"}
            </button>
          </>
        )
      }
    >
      {inviteResult ? (
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
              Einladung erstellt
            </span>
          </div>
          <div className="space-y-2 text-sm text-gray-600">
            <p>
              <span className="font-medium">E-Mail:</span> {inviteResult.email}
            </p>
            <p>
              <span className="font-medium">Status:</span>{" "}
              <DeliveryStatusBadge status={inviteResult.deliveryStatus} />
            </p>
            {inviteResult.emailError && (
              <p className="text-red-600">
                <span className="font-medium">Fehler:</span>{" "}
                {inviteResult.emailError}
              </p>
            )}
          </div>
        </div>
      ) : (
        <form
          onSubmit={(e) => void handleInvite(e)}
          className="space-y-4"
          id="invite-admin-form"
        >
          <FormField label="E-Mail" htmlFor="invite-email" required>
            <input
              id="invite-email"
              type="email"
              autoComplete="email"
              value={inviteEmail}
              onChange={(e) => setInviteEmail(e.target.value)}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              required
            />
          </FormField>
          <div className="grid grid-cols-2 gap-4">
            <FormField label="Vorname" htmlFor="invite-first-name">
              <input
                id="invite-first-name"
                type="text"
                autoComplete="given-name"
                value={inviteFirstName}
                onChange={(e) => setInviteFirstName(e.target.value)}
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              />
            </FormField>
            <FormField label="Nachname" htmlFor="invite-last-name">
              <input
                id="invite-last-name"
                type="text"
                autoComplete="family-name"
                value={inviteLastName}
                onChange={(e) => setInviteLastName(e.target.value)}
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              />
            </FormField>
          </div>
          {inviteError && <FormError ref={errorRef} message={inviteError} />}
        </form>
      )}
    </Modal>
  );
}
