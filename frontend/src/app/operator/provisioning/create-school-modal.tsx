import { useState, useCallback, useEffect, useRef } from "react";
import { Modal } from "~/components/ui/modal";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  operatorProvisioningService,
  revalidateTenantCache,
} from "~/lib/operator/provisioning-api";
import { generateSlug, isValidSlug } from "~/lib/operator/provisioning-helpers";
import type { Organization } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";
import { CustomSelect } from "~/components/ui/custom-select";
import { FormField, VisibilityToggle } from "./provisioning-shared";

const logger = createLogger({ component: "CreateSchoolModal" });

export function CreateSchoolModal({
  isOpen,
  onClose,
  organizations,
  onCreated,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly organizations: Organization[] | undefined;
  readonly onCreated: () => Promise<void>;
}) {
  const [schoolOrgId, setSchoolOrgId] = useState("");
  const [schoolName, setSchoolName] = useState("");
  const [schoolSlug, setSchoolSlug] = useState("");
  const [schoolSlugManual, setSchoolSlugManual] = useState(false);
  const [schoolSubdomain, setSchoolSubdomain] = useState("");
  const [schoolSubdomainManual, setSchoolSubdomainManual] = useState(false);
  const [schoolAddress, setSchoolAddress] = useState("");
  const [schoolCity, setSchoolCity] = useState("");
  const [schoolZip, setSchoolZip] = useState("");
  const [schoolPhone, setSchoolPhone] = useState("");
  const [schoolEmail, setSchoolEmail] = useState("");
  const [schoolHidden, setSchoolHidden] = useState(false);
  const [schoolSaving, setSchoolSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, invalid, clear: clearError } = formErrors;

  // Reset form and pre-select org when opening
  useEffect(() => {
    if (isOpen) {
      setSchoolOrgId(organizations?.length === 1 ? organizations[0]!.id : "");
      setSchoolName("");
      setSchoolSlug("");
      setSchoolSlugManual(false);
      setSchoolSubdomain("");
      setSchoolSubdomainManual(false);
      setSchoolAddress("");
      setSchoolCity("");
      setSchoolZip("");
      setSchoolPhone("");
      setSchoolEmail("");
      setSchoolHidden(false);
      clearError();
    }
  }, [isOpen, organizations, clearError]);

  const handleSchoolNameChange = useCallback(
    (value: string) => {
      setSchoolName(value);
      const slug = generateSlug(value);
      if (!schoolSlugManual) {
        setSchoolSlug(slug);
      }
      if (!schoolSubdomainManual) {
        setSchoolSubdomain(slug);
      }
    },
    [schoolSlugManual, schoolSubdomainManual],
  );

  const handleCreate = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      // CustomSelect's `required` is ARIA-only — it does not join native form
      // constraint validation, so an Enter-submit with no organization lands
      // here and must produce a visible error instead of a silent return.
      if (!schoolOrgId) {
        const hint = "Bitte wählen Sie einen Träger aus.";
        invalid("Bitte prüfen Sie die markierten Felder.", {
          organization_id: hint,
        });
        return;
      }
      if (!schoolName.trim() || !schoolSlug.trim() || !schoolSubdomain.trim())
        return;
      if (!isValidSlug(schoolSlug)) {
        const hint =
          "Slug darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten.";
        invalid("Bitte prüfen Sie die markierten Felder.", { slug: hint });
        return;
      }
      if (!isValidSlug(schoolSubdomain)) {
        const hint =
          "Subdomain darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten.";
        invalid("Bitte prüfen Sie die markierten Felder.", { subdomain: hint });
        return;
      }
      setSchoolSaving(true);
      clearError();
      try {
        await operatorProvisioningService.createSchool({
          organization_id: parseInt(schoolOrgId, 10),
          name: schoolName.trim(),
          slug: schoolSlug.trim(),
          subdomain: schoolSubdomain.trim(),
          ...(schoolAddress && { address: schoolAddress.trim() }),
          ...(schoolCity && { city: schoolCity.trim() }),
          ...(schoolZip && { zip: schoolZip.trim() }),
          ...(schoolPhone && { phone: schoolPhone.trim() }),
          ...(schoolEmail && { email: schoolEmail.trim() }),
          ...(schoolHidden && { hidden: true }),
        });
        onClose();
        await onCreated();
        await revalidateTenantCache([schoolSubdomain.trim()]);
      } catch (error) {
        logger.error("school_create_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        void showError(error, { object: "das Anlegen der Schule" });
      } finally {
        setSchoolSaving(false);
      }
    },
    [
      schoolOrgId,
      schoolName,
      schoolSlug,
      schoolSubdomain,
      schoolAddress,
      schoolCity,
      schoolZip,
      schoolPhone,
      schoolEmail,
      schoolHidden,
      onClose,
      onCreated,
      invalid,
      clearError,
      showError,
    ],
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Neue Schule"
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
            onClick={(e) => void handleCreate(e)}
            disabled={
              schoolSaving ||
              !schoolOrgId ||
              !schoolName.trim() ||
              !schoolSlug.trim() ||
              !schoolSubdomain.trim()
            }
            className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {schoolSaving ? "Wird erstellt..." : "Erstellen"}
          </button>
        </>
      }
    >
      <form
        ref={formRef}
        onSubmit={(e) => void handleCreate(e)}
        className="space-y-4"
        id="create-school-form"
      >
        <FormErrorAlert message={formErrors.error} />
        <FormField
          label="Träger"
          htmlFor="school-org"
          required
          error={formErrors.fieldError("organization_id")}
        >
          <CustomSelect
            id="school-org"
            ariaLabel="Träger"
            value={schoolOrgId}
            options={[
              { value: "", label: "Träger auswählen..." },
              ...(organizations?.map((org) => ({
                value: org.id,
                label: org.name,
              })) ?? []),
            ]}
            onChange={setSchoolOrgId}
            placeholder="Träger auswählen..."
            required
          />
        </FormField>
        <FormField
          label="Name"
          htmlFor="school-name"
          required
          error={formErrors.fieldError("name")}
        >
          <input
            id="school-name"
            name="name"
            type="text"
            value={schoolName}
            onChange={(e) => handleSchoolNameChange(e.target.value)}
            maxLength={255}
            className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
            required
          />
        </FormField>
        <div className="grid grid-cols-2 gap-4">
          <FormField
            label="Slug"
            htmlFor="school-slug"
            required
            error={formErrors.fieldError("slug")}
          >
            <input
              id="school-slug"
              name="slug"
              type="text"
              value={schoolSlug}
              onChange={(e) => {
                setSchoolSlugManual(true);
                setSchoolSlug(e.target.value);
              }}
              maxLength={100}
              className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 font-mono text-sm focus:ring-2 focus:outline-none"
              required
            />
          </FormField>
          <FormField
            label="Subdomain"
            htmlFor="school-subdomain"
            required
            error={formErrors.fieldError("subdomain")}
          >
            <input
              id="school-subdomain"
              name="subdomain"
              type="text"
              value={schoolSubdomain}
              onChange={(e) => {
                setSchoolSubdomainManual(true);
                setSchoolSubdomain(e.target.value);
              }}
              maxLength={63}
              className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 font-mono text-sm focus:ring-2 focus:outline-none"
              required
            />
          </FormField>
        </div>

        <div className="border-t border-gray-100 pt-4">
          <p className="mb-3 text-xs font-medium text-gray-500 uppercase">
            Kontaktdaten (optional)
          </p>
          <div className="space-y-3">
            <FormField
              label="Adresse"
              htmlFor="school-address"
              error={formErrors.fieldError("address")}
            >
              <input
                id="school-address"
                name="address"
                type="text"
                value={schoolAddress}
                onChange={(e) => setSchoolAddress(e.target.value)}
                maxLength={255}
                className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
              />
            </FormField>
            <div className="grid grid-cols-2 gap-4">
              <FormField
                label="PLZ"
                htmlFor="school-zip"
                error={formErrors.fieldError("zip")}
              >
                <input
                  id="school-zip"
                  name="zip"
                  type="text"
                  value={schoolZip}
                  onChange={(e) => setSchoolZip(e.target.value)}
                  maxLength={10}
                  className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
                />
              </FormField>
              <FormField
                label="Stadt"
                htmlFor="school-city"
                error={formErrors.fieldError("city")}
              >
                <input
                  id="school-city"
                  name="city"
                  type="text"
                  value={schoolCity}
                  onChange={(e) => setSchoolCity(e.target.value)}
                  maxLength={255}
                  className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
                />
              </FormField>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <FormField
                label="Telefon"
                htmlFor="school-phone"
                error={formErrors.fieldError("phone")}
              >
                <input
                  id="school-phone"
                  name="phone"
                  type="tel"
                  value={schoolPhone}
                  onChange={(e) => setSchoolPhone(e.target.value)}
                  maxLength={30}
                  className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
                />
              </FormField>
              <FormField
                label="E-Mail"
                htmlFor="school-email"
                error={formErrors.fieldError("email")}
              >
                <input
                  id="school-email"
                  name="email"
                  type="email"
                  value={schoolEmail}
                  onChange={(e) => setSchoolEmail(e.target.value)}
                  maxLength={255}
                  className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
                />
              </FormField>
            </div>
          </div>
        </div>
        <VisibilityToggle
          hidden={schoolHidden}
          onToggle={() => setSchoolHidden(!schoolHidden)}
        />
      </form>
    </Modal>
  );
}
