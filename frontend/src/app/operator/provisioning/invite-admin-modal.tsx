import { useState, useCallback, useEffect, useRef } from "react";
import { CheckCircle2 } from "lucide-react";
import { Checkbox } from "~/components/ui/checkbox";
import { Input } from "~/components/ui/input";
import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { Invitation } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";
import {
  FormField,
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
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, clear: clearError } = formErrors;

  const [inviteResult, setInviteResult] = useState<Invitation | null>(null);

  // Reset form when opening
  useEffect(() => {
    if (isOpen) {
      setInviteEmail("");
      setInviteFirstName("");
      setInviteLastName("");
      setPosition("");
      setCaregiverEnabled(false);
      clearError();
      setInviteResult(null);
    }
  }, [isOpen, clearError]);

  const handleInvite = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!schoolId || !inviteEmail.trim()) return;
      setInviteSaving(true);
      clearError();
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
        logger.error("admin_invite_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        void showError(error, { object: "die Einladung" });
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
      clearError,
      showError,
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
          <Button
            type="button"
            onClick={handleClose}
            variant="primary"
            size="md"
            className="flex-1"
          >
            Schließen
          </Button>
        ) : (
          <>
            <Button
              type="button"
              onClick={handleClose}
              variant="outline"
              size="md"
              className="flex-1"
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              onClick={(e) => void handleInvite(e)}
              disabled={inviteSaving || !inviteEmail.trim()}
              variant="primary"
              size="md"
              className="flex-1"
            >
              {inviteSaving ? "Wird gesendet..." : "Einladung senden"}
            </Button>
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
          ref={formRef}
        >
          <FormErrorAlert message={formErrors.error} />
          <Input
            id="invite-email"
            name="email"
            type="email"
            autoComplete="email"
            value={inviteEmail}
            onChange={(e) => setInviteEmail(e.target.value)}
            maxLength={255}
            label="E-Mail"
            required
            error={formErrors.fieldError("email")}
            controlSize="compact"
          />
          <div className="grid grid-cols-2 gap-4">
            <Input
              id="invite-first-name"
              name="first_name"
              type="text"
              autoComplete="given-name"
              value={inviteFirstName}
              onChange={(e) => setInviteFirstName(e.target.value)}
              maxLength={255}
              label="Vorname"
              error={formErrors.fieldError("first_name")}
              controlSize="compact"
            />
            <Input
              id="invite-last-name"
              name="last_name"
              type="text"
              autoComplete="family-name"
              value={inviteLastName}
              onChange={(e) => setInviteLastName(e.target.value)}
              maxLength={255}
              label="Nachname"
              error={formErrors.fieldError("last_name")}
              controlSize="compact"
            />
          </div>
          <div className="rounded-lg border border-gray-200 px-4 py-3">
            <label
              htmlFor="invite-admin-caregiver-enabled"
              className="flex cursor-pointer items-start gap-3 text-sm font-medium text-gray-900"
            >
              <Checkbox
                id="invite-admin-caregiver-enabled"
                checked={caregiverEnabled}
                onChange={(e) => setCaregiverEnabled(e.target.checked)}
                aria-describedby="invite-admin-caregiver-hint"
                className="mt-0.5"
              />
              Auch in der Betreuung einsetzen
            </label>
            <p
              id="invite-admin-caregiver-hint"
              className="mt-1 pl-8 text-sm text-gray-600"
            >
              Vergibt zusätzlich die Rolle Betreuung und legt das nötige
              Staff-/Teacher-Profil an.
            </p>
          </div>
          {caregiverEnabled && (
            <FormField
              label="Position"
              htmlFor="invite-admin-position"
              error={formErrors.fieldError("position")}
            >
              <SelectWithChevron
                id="invite-admin-position"
                name="position"
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
        </form>
      )}
    </Modal>
  );
}
