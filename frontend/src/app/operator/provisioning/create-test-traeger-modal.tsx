import { useCallback, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { CustomSelect } from "~/components/ui/custom-select";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import { generateSlug, isValidSlug } from "~/lib/operator/provisioning-helpers";
import { FormField } from "./provisioning-shared";

const logger = createLogger({ component: "CreateTestTraegerModal" });

const SCHOOL_COUNTS = ["1", "2", "3", "4", "5"] as const;
type SchoolCount = (typeof SCHOOL_COUNTS)[number];

/**
 * Sets up the test system of a new Träger (#4658) in one step: a test Träger
 * with hidden test schools, then the invented sample children of every test
 * school. When the children fail, the Träger exists already; its page offers
 * the sample children again.
 */
export function CreateTestTraegerModal({
  isOpen,
  onClose,
  onCreated,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreated: (slug: string) => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugManual, setSlugManual] = useState(false);
  const [schoolCount, setSchoolCount] = useState<SchoolCount>("3");
  const [contactEmail, setContactEmail] = useState("");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, invalid, clear: clearError } = formErrors;
  const { success: toastSuccess, warning: toastWarning } = useToast();
  const complete =
    name.trim() !== "" && slug.trim() !== "" && contactEmail.trim() !== "";

  const reset = useCallback(() => {
    setName("");
    setSlug("");
    setSlugManual(false);
    setSchoolCount("3");
    setContactEmail("");
    clearError();
  }, [clearError]);

  const handleClose = useCallback(() => {
    onClose();
    reset();
  }, [onClose, reset]);

  const handleSubmit = useCallback(
    async (event: React.FormEvent) => {
      event.preventDefault();
      if (!complete) return;
      if (!isValidSlug(slug.trim())) {
        invalid("Bitte prüfen Sie die markierten Felder.", {
          slug: "Slug darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten.",
        });
        return;
      }
      setSaving(true);
      clearError();
      let created: Awaited<
        ReturnType<typeof operatorProvisioningService.createTestTraeger>
      >;
      try {
        created = await operatorProvisioningService.createTestTraeger({
          name: name.trim(),
          slug: slug.trim(),
          school_count: Number(schoolCount),
          contact_email: contactEmail.trim(),
        });
      } catch (error) {
        logger.error("test_traeger_create_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        void showError(error, { object: "das Anlegen des Test-Trägers" });
        setSaving(false);
        return;
      }
      try {
        const children = await operatorProvisioningService.createSampleChildren(
          created.organization.id,
        );
        toastSuccess(
          `Der Test-Träger ist angelegt: ${created.schools.length} Test-Schulen mit ${children.createdCount} Beispielkindern.`,
        );
      } catch (error) {
        // The Träger exists already: say so and point to the retry on its
        // page instead of offering the same order again.
        logger.warn("test_traeger_sample_children_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        toastWarning(
          "Der Test-Träger ist angelegt, die Beispielkinder fehlen noch. Legen Sie sie auf der Seite des Trägers an.",
        );
      }
      setSaving(false);
      handleClose();
      await onCreated(created.organization.slug);
    },
    [
      clearError,
      complete,
      contactEmail,
      handleClose,
      invalid,
      name,
      onCreated,
      schoolCount,
      showError,
      slug,
      toastSuccess,
      toastWarning,
    ],
  );

  return (
    <FormModal
      isOpen={isOpen}
      onClose={handleClose}
      title="Test-Träger anlegen"
      size="md"
      closeDisabled={saving}
      footer={
        <>
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={handleClose}
            disabled={saving}
          >
            Abbrechen
          </Button>
          <Button
            type="submit"
            form="create-test-traeger-form"
            variant="primary"
            size="md"
            disabled={saving || !complete}
          >
            {saving ? "Wird angelegt..." : "Anlegen"}
          </Button>
        </>
      }
    >
      <form
        ref={formRef}
        id="create-test-traeger-form"
        onSubmit={(event) => void handleSubmit(event)}
        className="space-y-4"
      >
        <FormErrorAlert message={formErrors.error} />
        <p className="text-sm text-gray-600">
          Legt einen Test-Träger mit Test-Schulen und erfundenen Beispielkindern
          an. Der neue Träger probiert damit alles aus, ohne echte Daten.
          Test-Schulen stehen nicht in der öffentlichen Schulliste.
        </p>
        <Input
          id="test-traeger-name"
          name="name"
          label="Name"
          value={name}
          onChange={(event) => {
            setName(event.target.value);
            if (!slugManual) setSlug(generateSlug(event.target.value));
          }}
          error={formErrors.fieldError("name")}
          maxLength={200}
          required
        />
        <Input
          id="test-traeger-slug"
          name="slug"
          label="Slug"
          value={slug}
          onChange={(event) => {
            setSlugManual(true);
            setSlug(event.target.value);
          }}
          error={formErrors.fieldError("slug")}
          maxLength={56}
          className="font-mono"
          required
        />
        <p className="-mt-2 text-xs text-gray-500">
          Die Test-Schulen bekommen die Subdomains „{slug.trim() || "slug"}
          -test-1“ und so weiter.
        </p>
        <FormField
          label="Anzahl der Test-Schulen"
          htmlFor="test-traeger-school-count"
        >
          <CustomSelect
            id="test-traeger-school-count"
            ariaLabel="Anzahl der Test-Schulen"
            value={schoolCount}
            options={SCHOOL_COUNTS.map((count) => ({
              value: count,
              label: count,
            }))}
            onChange={(next) => setSchoolCount(next as SchoolCount)}
          />
        </FormField>
        <Input
          id="test-traeger-contact-email"
          name="contact_email"
          type="email"
          label="Kontaktadresse der Test-Schulen"
          value={contactEmail}
          onChange={(event) => setContactEmail(event.target.value)}
          error={formErrors.fieldError("contact_email")}
          maxLength={255}
          required
        />
      </form>
    </FormModal>
  );
}
