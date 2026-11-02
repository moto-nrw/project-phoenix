import { useState, useCallback, useRef } from "react";
import { Input } from "~/components/ui/input";
import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import { generateSlug, isValidSlug } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";

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
            onClick={(e) => void handleCreate(e)}
            disabled={orgSaving || !orgName.trim() || !orgSlug.trim()}
            variant="primary"
            size="md"
            className="flex-1"
          >
            {orgSaving ? "Wird erstellt..." : "Erstellen"}
          </Button>
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
        <Input
          id="org-name"
          name="name"
          type="text"
          value={orgName}
          onChange={(e) => handleOrgNameChange(e.target.value)}
          maxLength={255}
          label="Name"
          required
          error={formErrors.fieldError("name")}
          controlSize="compact"
        />
        <div>
          <Input
            id="org-slug"
            name="slug"
            type="text"
            value={orgSlug}
            onChange={(e) => {
              setOrgSlugManual(true);
              setOrgSlug(e.target.value);
            }}
            maxLength={255}
            label="Slug"
            required
            error={formErrors.fieldError("slug")}
            controlSize="compact"
            className="font-mono"
          />
          <p className="mt-1 text-xs text-gray-500">
            URL-freundlicher Bezeichner (z.B. &quot;stadt-koeln&quot;)
          </p>
        </div>
      </form>
    </Modal>
  );
}
