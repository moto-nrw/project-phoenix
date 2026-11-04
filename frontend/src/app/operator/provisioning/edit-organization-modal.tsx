import { useState, useCallback, useEffect, useRef } from "react";
import { Input } from "~/components/ui/input";
import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import { isValidSlug } from "~/lib/operator/provisioning-helpers";
import type { Organization } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";
import { FieldWarning } from "./provisioning-shared";

const logger = createLogger({ component: "EditOrganizationModal" });

export function EditOrganizationModal({
  isOpen,
  onClose,
  organization,
  onUpdated,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly organization: Organization | null;
  readonly onUpdated: () => Promise<void>;
}) {
  const [orgName, setOrgName] = useState("");
  const [orgSlug, setOrgSlug] = useState("");
  const [orgSaving, setOrgSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, invalid, clear: clearError } = formErrors;

  useEffect(() => {
    if (isOpen && organization) {
      setOrgName(organization.name);
      setOrgSlug(organization.slug);
      clearError();
    }
  }, [isOpen, organization, clearError]);

  const handleUpdate = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!organization || !orgName.trim() || !orgSlug.trim()) return;
      if (!isValidSlug(orgSlug)) {
        const hint =
          "Slug darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten.";
        invalid("Bitte prüfen Sie die markierten Felder.", { slug: hint });
        return;
      }
      setOrgSaving(true);
      clearError();
      try {
        await operatorProvisioningService.updateOrganization(organization.id, {
          name: orgName.trim(),
          slug: orgSlug.trim(),
          active: organization.active,
        });
        onClose();
        await onUpdated();
      } catch (error) {
        logger.error("organization_update_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        void showError(error, { object: "die Änderung am Träger" });
      } finally {
        setOrgSaving(false);
      }
    },
    [
      organization,
      orgName,
      orgSlug,
      onClose,
      onUpdated,
      invalid,
      clearError,
      showError,
    ],
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Träger bearbeiten"
      footer={
        <>
          <Button
            type="button"
            onClick={onClose}
            variant="outline"
            size="md"
            className="flex-1"
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            onClick={(e) => void handleUpdate(e)}
            disabled={orgSaving || !orgName.trim() || !orgSlug.trim()}
            variant="primary"
            size="md"
            className="flex-1"
          >
            {orgSaving ? "Wird gespeichert..." : "Speichern"}
          </Button>
        </>
      }
    >
      <form
        ref={formRef}
        onSubmit={(e) => void handleUpdate(e)}
        className="space-y-4"
        id="edit-org-form"
      >
        <FormErrorAlert message={formErrors.error} />
        <Input
          id="edit-org-name"
          name="name"
          type="text"
          value={orgName}
          onChange={(e) => setOrgName(e.target.value)}
          maxLength={255}
          label="Name"
          required
          error={formErrors.fieldError("name")}
          controlSize="compact"
        />
        <div>
          <Input
            id="edit-org-slug"
            name="slug"
            type="text"
            value={orgSlug}
            onChange={(e) => setOrgSlug(e.target.value)}
            maxLength={255}
            label="Slug"
            required
            error={formErrors.fieldError("slug")}
            controlSize="compact"
            className="font-mono"
          />
          <FieldWarning message="Slug-Änderungen können bestehende Verweise ungültig machen." />
        </div>
      </form>
    </Modal>
  );
}
