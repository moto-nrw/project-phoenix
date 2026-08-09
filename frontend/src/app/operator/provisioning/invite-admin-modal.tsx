import { useState, useCallback, useEffect } from "react";
import { CheckCircle2 } from "lucide-react";
import { Modal } from "~/components/ui/modal";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { Invitation } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";
import {
  FormField,
  FormError,
  DeliveryStatusBadge,
  SelectWithChevron,
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
  const [position, setPosition] = useState("");
  const [caregiverEnabled, setCaregiverEnabled] = useState(false);
  const [inviteSaving, setInviteSaving] = useState(false);
  const [inviteError, setInviteError] = useState("");
  const errorRef = useScrollToError(inviteError);

  const [inviteResult, setInviteResult] = useState<Invitation | null>(null);
  const inputClasses =
    "focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none";

  // Reset form when opening
  useEffect(() => {
    if (isOpen) {
      setInviteEmail("");
      setInviteFirstName("");
      setInviteLastName("");
      setPosition("");
      setCaregiverEnabled(false);
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
            ...(caregiverEnabled && { caregiver_enabled: true }),
            ...(caregiverEnabled && position && { position: position.trim() }),
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
    [
      schoolId,
      inviteEmail,
      inviteFirstName,
      inviteLastName,
      caregiverEnabled,
      position,
    ],
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
          <div className="bg-moto-green/10 flex items-center gap-2 rounded-lg px-4 py-3">
            <CheckCircle2
              className="text-moto-green h-5 w-5"
              aria-hidden="true"
            />
            <span className="text-moto-green-strong text-sm font-medium">
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
            {inviteResult.caregiverEnabled && (
              <p>
                <span className="font-medium">Einsatz:</span> Verwaltung +
                Betreuung
              </p>
            )}
            {inviteResult.emailError && (
              <p className="text-moto-red">
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
              maxLength={255}
              className={inputClasses}
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
                maxLength={255}
                className={inputClasses}
              />
            </FormField>
            <FormField label="Nachname" htmlFor="invite-last-name">
              <input
                id="invite-last-name"
                type="text"
                autoComplete="family-name"
                value={inviteLastName}
                onChange={(e) => setInviteLastName(e.target.value)}
                maxLength={255}
                className={inputClasses}
              />
            </FormField>
          </div>
          <div className="flex items-start gap-3 rounded-lg border border-gray-200 px-4 py-3">
            <input
              id="invite-admin-caregiver-enabled"
              type="checkbox"
              checked={caregiverEnabled}
              onChange={(e) => setCaregiverEnabled(e.target.checked)}
              className="text-moto-blue focus:ring-moto-blue mt-0.5 h-4 w-4 rounded border-gray-300"
            />
            <div className="space-y-1">
              <label
                htmlFor="invite-admin-caregiver-enabled"
                className="text-sm font-medium text-gray-900"
              >
                Auch als Betreuer einsetzen
              </label>
              <p className="text-sm text-gray-600">
                Vergibt zusätzlich die Betreuer-Rolle und legt das nötige
                Staff-/Teacher-Profil an.
              </p>
            </div>
          </div>
          {caregiverEnabled && (
            <FormField label="Position" htmlFor="invite-admin-position">
              <SelectWithChevron
                id="invite-admin-position"
                value={position}
                onChange={(e) => setPosition(e.target.value)}
              >
                <option value="">Position auswählen...</option>
                <option value="Pädagogische Fachkraft">
                  Pädagogische Fachkraft
                </option>
                <option value="OGS-Büro">OGS-Büro</option>
                <option value="Extern">Extern</option>
              </SelectWithChevron>
            </FormField>
          )}
          {inviteError && <FormError ref={errorRef} message={inviteError} />}
        </form>
      )}
    </Modal>
  );
}
