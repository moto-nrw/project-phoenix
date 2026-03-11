"use client";

import { useState, useCallback, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import {
  isEmailConflict,
  DELIVERY_STATUS_LABELS,
  DELIVERY_STATUS_STYLES,
} from "~/lib/operator/provisioning-helpers";
import type {
  School,
  Invitation,
  DeliveryStatus,
} from "~/lib/operator/provisioning-helpers";
import { isOperatorApiError } from "~/lib/operator/api-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "InviteAdminModal" });

interface InviteAdminModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly school: School | null;
  readonly onInvited: () => void;
}

export function InviteAdminModal({
  isOpen,
  onClose,
  school,
  onInvited,
}: InviteAdminModalProps) {
  const [email, setEmail] = useState("");
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [position, setPosition] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const [emailError, setEmailError] = useState<string | null>(null);
  const [successInvitation, setSuccessInvitation] = useState<Invitation | null>(
    null,
  );

  // Reset form when modal opens
  useEffect(() => {
    if (isOpen) {
      setEmail("");
      setFirstName("");
      setLastName("");
      setPosition("");
      setIsSaving(false);
      setEmailError(null);
      setSuccessInvitation(null);
    }
  }, [isOpen]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!school) return;

      setEmailError(null);
      if (!email.trim()) {
        setEmailError("E-Mail ist erforderlich");
        return;
      }

      setIsSaving(true);
      try {
        const invitation = await operatorProvisioningService.inviteSchoolAdmin(
          school.id,
          {
            email: email.trim().toLowerCase(),
            ...(firstName.trim() && { first_name: firstName.trim() }),
            ...(lastName.trim() && { last_name: lastName.trim() }),
            ...(position.trim() && { position: position.trim() }),
          },
        );
        setSuccessInvitation(invitation);
        onInvited();
      } catch (error) {
        if (isOperatorApiError(error) && error.status === 409) {
          if (isEmailConflict(error.message)) {
            setEmailError("Diese E-Mail-Adresse wurde bereits eingeladen");
          } else {
            setEmailError(error.message);
          }
        } else {
          logger.error("invite_admin_failed", {
            error: error instanceof Error ? error.message : String(error),
          });
        }
      } finally {
        setIsSaving(false);
      }
    },
    [school, email, firstName, lastName, position, onInvited],
  );

  // Success state
  if (successInvitation) {
    const status = successInvitation.deliveryStatus as DeliveryStatus;
    return (
      <Modal
        isOpen={isOpen}
        onClose={onClose}
        title="Einladung gesendet"
        footer={
          <button
            type="button"
            onClick={onClose}
            className="w-full rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800"
          >
            Schließen
          </button>
        }
      >
        <div className="space-y-4 py-2">
          <div className="rounded-xl bg-green-50 p-4">
            <p className="text-sm font-medium text-green-800">
              Einladung erfolgreich erstellt
            </p>
            <p className="mt-1 text-sm text-green-700">
              Eine Einladung wurde an{" "}
              <span className="font-medium">{successInvitation.email}</span>{" "}
              gesendet.
            </p>
          </div>

          <div className="space-y-2 text-sm">
            <div className="flex items-center justify-between">
              <span className="text-gray-500">Zustellstatus</span>
              <span
                className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                  DELIVERY_STATUS_STYLES[status] ?? "bg-gray-100 text-gray-700"
                }`}
              >
                {DELIVERY_STATUS_LABELS[status] ?? status}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-gray-500">Gültig bis</span>
              <span className="text-gray-900">
                {new Date(successInvitation.expiresAt).toLocaleDateString(
                  "de-DE",
                  {
                    day: "2-digit",
                    month: "2-digit",
                    year: "numeric",
                    hour: "2-digit",
                    minute: "2-digit",
                  },
                )}
              </span>
            </div>
          </div>
        </div>
      </Modal>
    );
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={`Admin einladen — ${school?.name ?? ""}`}
      footer={
        <>
          <button
            type="button"
            onClick={onClose}
            className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={(e) => void handleSubmit(e)}
            disabled={isSaving || !email.trim()}
            className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {isSaving ? "Wird gesendet..." : "Einladen"}
          </button>
        </>
      }
    >
      <form
        onSubmit={(e) => void handleSubmit(e)}
        className="space-y-4"
        id="invite-admin-form"
      >
        <div>
          <label
            htmlFor="invite-email"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            E-Mail
          </label>
          <input
            id="invite-email"
            type="email"
            value={email}
            onChange={(e) => {
              setEmail(e.target.value);
              setEmailError(null);
            }}
            className={`w-full rounded-lg border px-3 py-2 text-sm focus:ring-1 focus:outline-none ${
              emailError
                ? "border-red-300 focus:border-red-500 focus:ring-red-500"
                : "border-gray-200 focus:border-blue-500 focus:ring-blue-500"
            }`}
            placeholder="admin@schule.de"
            required
          />
          {emailError && (
            <p className="mt-1 text-xs text-red-600">{emailError}</p>
          )}
        </div>

        <div className="border-t border-gray-100 pt-4">
          <p className="mb-3 text-xs font-medium text-gray-500 uppercase">
            Optional
          </p>
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label
                  htmlFor="invite-first-name"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Vorname
                </label>
                <input
                  id="invite-first-name"
                  type="text"
                  value={firstName}
                  onChange={(e) => setFirstName(e.target.value)}
                  className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                />
              </div>
              <div>
                <label
                  htmlFor="invite-last-name"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Nachname
                </label>
                <input
                  id="invite-last-name"
                  type="text"
                  value={lastName}
                  onChange={(e) => setLastName(e.target.value)}
                  className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                />
              </div>
            </div>

            <div>
              <label
                htmlFor="invite-position"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Position
              </label>
              <input
                id="invite-position"
                type="text"
                value={position}
                onChange={(e) => setPosition(e.target.value)}
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                placeholder="z.B. Schulleitung"
              />
            </div>
          </div>
        </div>
      </form>
    </Modal>
  );
}
