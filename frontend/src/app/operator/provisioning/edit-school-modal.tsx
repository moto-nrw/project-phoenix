import { useState, useCallback, useEffect, useRef } from "react";
import { Modal } from "~/components/ui/modal";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  operatorProvisioningService,
  revalidateTenantCache,
} from "~/lib/operator/provisioning-api";
import { isValidSlug } from "~/lib/operator/provisioning-helpers";
import type { Organization, School } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";
import { CustomSelect } from "~/components/ui/custom-select";
import {
  FormField,
  FieldWarning,
  VisibilityToggle,
} from "./provisioning-shared";

const logger = createLogger({ component: "EditSchoolModal" });

export function EditSchoolModal({
  isOpen,
  onClose,
  school,
  organizations,
  onUpdated,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly school: School | null;
  readonly organizations: Organization[] | undefined;
  readonly onUpdated: () => Promise<void>;
}) {
  const [schoolOrgId, setSchoolOrgId] = useState("");
  const [schoolName, setSchoolName] = useState("");
  const [schoolSlug, setSchoolSlug] = useState("");
  const [schoolSubdomain, setSchoolSubdomain] = useState("");
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

  useEffect(() => {
    if (isOpen && school) {
      setSchoolOrgId(school.organizationId);
      setSchoolName(school.name);
      setSchoolSlug(school.slug);
      setSchoolSubdomain(school.subdomain);
      setSchoolAddress(school.address ?? "");
      setSchoolCity(school.city ?? "");
      setSchoolZip(school.zip ?? "");
      setSchoolPhone(school.phone ?? "");
      setSchoolEmail(school.email ?? "");
      setSchoolHidden(school.hidden);
      clearError();
    }
  }, [isOpen, school, clearError]);

  const handleUpdate = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!school) return;
      // CustomSelect's `required` is ARIA-only — it does not join native form
      // constraint validation, so an Enter-submit with a cleared organization
      // lands here and must produce a visible error instead of a silent return.
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
        const oldSubdomain = school.subdomain;
        const newSubdomain = schoolSubdomain.trim();
        await operatorProvisioningService.updateSchool(school.id, {
          organization_id: parseInt(schoolOrgId, 10),
          name: schoolName.trim(),
          slug: schoolSlug.trim(),
          subdomain: newSubdomain,
          address: schoolAddress?.trim() ?? "",
          city: schoolCity?.trim() ?? "",
          zip: schoolZip?.trim() ?? "",
          phone: schoolPhone?.trim() ?? "",
          email: schoolEmail?.trim() ?? "",
          active: school.active,
          hidden: schoolHidden,
        });
        if (oldSubdomain !== newSubdomain || school.hidden !== schoolHidden) {
          await revalidateTenantCache([oldSubdomain, newSubdomain]);
        }
        onClose();
        await onUpdated();
      } catch (error) {
        logger.error("school_update_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        void showError(error, { object: "die Änderung an der Schule" });
      } finally {
        setSchoolSaving(false);
      }
    },
    [
      school,
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
      title="Schule bearbeiten"
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
            disabled={
              schoolSaving ||
              !schoolOrgId ||
              !schoolName.trim() ||
              !schoolSlug.trim() ||
              !schoolSubdomain.trim()
            }
            className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {schoolSaving ? "Wird gespeichert..." : "Speichern"}
          </button>
        </>
      }
    >
      <form
        ref={formRef}
        onSubmit={(e) => void handleUpdate(e)}
        className="space-y-4"
        id="edit-school-form"
      >
        <FormErrorAlert message={formErrors.error} />
        <FormField
          label="Träger"
          htmlFor="edit-school-org"
          required
          error={formErrors.fieldError("organization_id")}
        >
          <CustomSelect
            id="edit-school-org"
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
          {school && schoolOrgId !== school.organizationId && (
            <FieldWarning message="Trägerwechsel kann die Slug-Eindeutigkeit in der neuen Organisation beeinflussen." />
          )}
        </FormField>
        <FormField
          label="Name"
          htmlFor="edit-school-name"
          required
          error={formErrors.fieldError("name")}
        >
          <input
            id="edit-school-name"
            name="name"
            type="text"
            value={schoolName}
            onChange={(e) => setSchoolName(e.target.value)}
            maxLength={255}
            className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
            required
          />
        </FormField>
        <div className="grid grid-cols-2 gap-4">
          <FormField
            label="Slug"
            htmlFor="edit-school-slug"
            required
            error={formErrors.fieldError("slug")}
          >
            <input
              id="edit-school-slug"
              name="slug"
              type="text"
              value={schoolSlug}
              onChange={(e) => setSchoolSlug(e.target.value)}
              maxLength={100}
              className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 font-mono text-sm focus:ring-2 focus:outline-none"
              required
            />
            {school && schoolSlug !== school.slug && (
              <FieldWarning message="Slug-Änderungen können bestehende Verweise ungültig machen." />
            )}
          </FormField>
          <FormField
            label="Subdomain"
            htmlFor="edit-school-subdomain"
            required
            error={formErrors.fieldError("subdomain")}
          >
            <input
              id="edit-school-subdomain"
              name="subdomain"
              type="text"
              value={schoolSubdomain}
              onChange={(e) => setSchoolSubdomain(e.target.value)}
              maxLength={63}
              className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 font-mono text-sm focus:ring-2 focus:outline-none"
              required
            />
            {school && schoolSubdomain !== school.subdomain && (
              <FieldWarning message="Subdomain-Änderungen erfordern, dass alle Benutzer die neue Adresse verwenden. Die alte Adresse wird nach kurzer Zeit nicht mehr erreichbar sein." />
            )}
          </FormField>
        </div>

        <div className="border-t border-gray-100 pt-4">
          <p className="mb-3 text-xs font-medium text-gray-500 uppercase">
            Kontaktdaten
          </p>
          <div className="space-y-3">
            <FormField
              label="Adresse"
              htmlFor="edit-school-address"
              error={formErrors.fieldError("address")}
            >
              <input
                id="edit-school-address"
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
                htmlFor="edit-school-zip"
                error={formErrors.fieldError("zip")}
              >
                <input
                  id="edit-school-zip"
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
                htmlFor="edit-school-city"
                error={formErrors.fieldError("city")}
              >
                <input
                  id="edit-school-city"
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
                htmlFor="edit-school-phone"
                error={formErrors.fieldError("phone")}
              >
                <input
                  id="edit-school-phone"
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
                htmlFor="edit-school-email"
                error={formErrors.fieldError("email")}
              >
                <input
                  id="edit-school-email"
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
