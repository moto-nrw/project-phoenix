import { useState, useCallback, useRef } from "react";
import { Modal } from "~/components/ui/modal";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import { generateSlug, isValidSlug } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";
import { FormField } from "./provisioning-shared";

const logger = createLogger({ component: "CreateOrganizationModal" });

export function CreateOrganizationModal({
  isOpen,
  onClose,
  onCreated,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreated: () => Promise<void>;
}) {
  const [orgName, setOrgName] = useState("");
  const [orgSlug, setOrgSlug] = useState("");
  const [orgSlugManual, setOrgSlugManual] = useState(false);
  const [orgSaving, setOrgSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, invalid, clear: clearError } = formErrors;

  const resetForm = useCallback(() => {
    setOrgName("");
    setOrgSlug("");
    setOrgSlugManual(false);
    clearError();
  }, [clearError]);

  const handleOrgNameChange = useCallback(
    (value: string) => {
      setOrgName(value);
      if (!orgSlugManual) {
        setOrgSlug(generateSlug(value));
      }
    },
    [orgSlugManual],
  );

  const handleClose = useCallback(() => {
    onClose();
    resetForm();
  }, [onClose, resetForm]);

  const handleCreate = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!orgName.trim() || !orgSlug.trim()) return;
      if (!isValidSlug(orgSlug)) {
        const hint =
          "Slug darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten.";
        invalid("Bitte prüfen Sie die markierten Felder.", { slug: hint });
        return;
      }
      setOrgSaving(true);
      clearError();
      try {
        await operatorProvisioningService.createOrganization({
          name: orgName.trim(),
          slug: orgSlug.trim(),
        });
        handleClose();
        await onCreated();
      } catch (error) {
        logger.error("organization_create_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        void showError(error, { object: "das Anlegen des Trägers" });
      } finally {
        setOrgSaving(false);
      }
    },
    [orgName, orgSlug, onCreated, handleClose, invalid, clearError, showError],
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title="Neuer Träger"
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
            type="button"
            onClick={(e) => void handleCreate(e)}
            disabled={orgSaving || !orgName.trim() || !orgSlug.trim()}
            className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {orgSaving ? "Wird erstellt..." : "Erstellen"}
          </button>
        </>
      }
    >
      <form
        ref={formRef}
        onSubmit={(e) => void handleCreate(e)}
        className="space-y-4"
        id="create-org-form"
      >
        <FormErrorAlert message={formErrors.error} />
        <FormField
          label="Name"
          htmlFor="org-name"
          required
          error={formErrors.fieldError("name")}
        >
          <input
            id="org-name"
            name="name"
            type="text"
            value={orgName}
            onChange={(e) => handleOrgNameChange(e.target.value)}
            maxLength={255}
            className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
            required
          />
        </FormField>
        <FormField
          label="Slug"
          htmlFor="org-slug"
          required
          error={formErrors.fieldError("slug")}
        >
          <input
            id="org-slug"
            name="slug"
            type="text"
            value={orgSlug}
            onChange={(e) => {
              setOrgSlugManual(true);
              setOrgSlug(e.target.value);
            }}
            maxLength={255}
            className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 font-mono text-sm focus:ring-2 focus:outline-none"
            required
          />
          <p className="mt-1 text-xs text-gray-500">
            URL-freundlicher Bezeichner (z.B. &quot;stadt-koeln&quot;)
          </p>
        </FormField>
      </form>
    </Modal>
  );
}
