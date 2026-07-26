import { useState, useCallback, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import {
  operatorProvisioningService,
  revalidateTenantCache,
} from "~/lib/operator/provisioning-api";
import { isValidSlug } from "~/lib/operator/provisioning-helpers";
import type { Organization, School } from "~/lib/operator/provisioning-helpers";
import { isOperatorApiError } from "~/lib/operator/api-helpers";
import { createLogger } from "~/lib/logger";
import { CustomSelect } from "~/components/ui/custom-select";
import {
  FormField,
  FormError,
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
  const [schoolError, setSchoolError] = useState("");
  const errorRef = useScrollToError(schoolError);

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
      setSchoolError("");
    }
  }, [isOpen, school]);

  const handleUpdate = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!school) return;
      // CustomSelect's `required` is ARIA-only — it does not join native form
      // constraint validation, so an Enter-submit with a cleared organization
      // lands here and must produce a visible error instead of a silent return.
      if (!schoolOrgId) {
        setSchoolError("Bitte wählen Sie einen Träger aus.");
        return;
      }
      if (!schoolName.trim() || !schoolSlug.trim() || !schoolSubdomain.trim())
        return;
      if (!isValidSlug(schoolSlug)) {
        setSchoolError(
          "Slug darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten.",
        );
        return;
      }
      if (!isValidSlug(schoolSubdomain)) {
        setSchoolError(
          "Subdomain darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten.",
        );
        return;
      }
      setSchoolSaving(true);
      setSchoolError("");
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
        if (isOperatorApiError(error) && error.status === 409) {
          const msg = error instanceof Error ? error.message.toLowerCase() : "";
          if (msg.includes("subdomain")) {
            setSchoolError(
              "Eine Schule mit dieser Subdomain existiert bereits.",
            );
          } else {
            setSchoolError(
              "Eine Schule mit diesem Slug existiert bereits in dieser Organisation.",
            );
          }
        } else {
          setSchoolError(
            error instanceof Error ? error.message : "Fehler beim Speichern.",
          );
          logger.error("school_update_failed", {
            error: error instanceof Error ? error.message : String(error),
          });
        }
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
        onSubmit={(e) => void handleUpdate(e)}
        className="space-y-4"
        id="edit-school-form"
      >
        <FormField label="Träger" htmlFor="edit-school-org" required>
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
        <FormField label="Name" htmlFor="edit-school-name" required>
          <input
            id="edit-school-name"
            type="text"
            value={schoolName}
            onChange={(e) => setSchoolName(e.target.value)}
            maxLength={255}
            className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
            required
          />
        </FormField>
        <div className="grid grid-cols-2 gap-4">
          <FormField label="Slug" htmlFor="edit-school-slug" required>
            <input
              id="edit-school-slug"
              type="text"
              value={schoolSlug}
              onChange={(e) => setSchoolSlug(e.target.value)}
              maxLength={100}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 font-mono text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              required
            />
            {school && schoolSlug !== school.slug && (
              <FieldWarning message="Slug-Änderungen können bestehende Verweise ungültig machen." />
            )}
          </FormField>
          <FormField label="Subdomain" htmlFor="edit-school-subdomain" required>
            <input
              id="edit-school-subdomain"
              type="text"
              value={schoolSubdomain}
              onChange={(e) => setSchoolSubdomain(e.target.value)}
              maxLength={63}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 font-mono text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
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
            <FormField label="Adresse" htmlFor="edit-school-address">
              <input
                id="edit-school-address"
                type="text"
                value={schoolAddress}
                onChange={(e) => setSchoolAddress(e.target.value)}
                maxLength={255}
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              />
            </FormField>
            <div className="grid grid-cols-2 gap-4">
              <FormField label="PLZ" htmlFor="edit-school-zip">
                <input
                  id="edit-school-zip"
                  type="text"
                  value={schoolZip}
                  onChange={(e) => setSchoolZip(e.target.value)}
                  maxLength={10}
                  className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                />
              </FormField>
              <FormField label="Stadt" htmlFor="edit-school-city">
                <input
                  id="edit-school-city"
                  type="text"
                  value={schoolCity}
                  onChange={(e) => setSchoolCity(e.target.value)}
                  maxLength={255}
                  className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                />
              </FormField>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <FormField label="Telefon" htmlFor="edit-school-phone">
                <input
                  id="edit-school-phone"
                  type="tel"
                  value={schoolPhone}
                  onChange={(e) => setSchoolPhone(e.target.value)}
                  maxLength={30}
                  className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                />
              </FormField>
              <FormField label="E-Mail" htmlFor="edit-school-email">
                <input
                  id="edit-school-email"
                  type="email"
                  value={schoolEmail}
                  onChange={(e) => setSchoolEmail(e.target.value)}
                  maxLength={255}
                  className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                />
              </FormField>
            </div>
          </div>
        </div>
        <VisibilityToggle
          hidden={schoolHidden}
          onToggle={() => setSchoolHidden(!schoolHidden)}
        />

        {schoolError && <FormError ref={errorRef} message={schoolError} />}
      </form>
    </Modal>
  );
}
