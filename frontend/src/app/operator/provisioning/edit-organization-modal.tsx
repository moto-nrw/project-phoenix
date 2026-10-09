import { useState, useCallback, useEffect, useRef } from "react";
import { Modal } from "~/components/ui/modal";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import { isValidSlug } from "~/lib/operator/provisioning-helpers";
import type { Organization } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";
import { FormField, FieldWarning } from "./provisioning-shared";

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
          <button
            type="button"
            onClick={onClose}
            className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={(e) => void handleUpdate(e)}
            disabled={orgSaving || !orgName.trim() || !orgSlug.trim()}
            className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {orgSaving ? "Wird gespeichert..." : "Speichern"}
          </button>
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
        <FormField
          label="Name"
          htmlFor="edit-org-name"
          required
          error={formErrors.fieldError("name")}
        >
          <input
            id="edit-org-name"
            name="name"
            type="text"
            value={orgName}
            onChange={(e) => setOrgName(e.target.value)}
            maxLength={255}
            className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
            required
          />
        </FormField>
        <FormField
          label="Slug"
          htmlFor="edit-org-slug"
          required
          error={formErrors.fieldError("slug")}
        >
          <input
            id="edit-org-slug"
            name="slug"
            type="text"
            value={orgSlug}
            onChange={(e) => setOrgSlug(e.target.value)}
            maxLength={255}
            className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 font-mono text-sm focus:ring-2 focus:outline-none"
            required
          />
          <FieldWarning message="Slug-Änderungen können bestehende Verweise ungültig machen." />
        </FormField>
      </form>
    </Modal>
  );
}
