import { useState, useCallback, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import {
  operatorProvisioningService,
  revalidateTenantCache,
} from "~/lib/operator/provisioning-api";
import { generateSlug, isValidSlug } from "~/lib/operator/provisioning-helpers";
import type { Organization } from "~/lib/operator/provisioning-helpers";
import { isOperatorApiError } from "~/lib/operator/api-helpers";
import { createLogger } from "~/lib/logger";
import { CustomSelect } from "~/components/ui/custom-select";
import { FormField, FormError, VisibilityToggle } from "./provisioning-shared";

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
  const [schoolError, setSchoolError] = useState("");
  const errorRef = useScrollToError(schoolError);

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
      setSchoolError("");
    }
  }, [isOpen, organizations]);

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
        if (isOperatorApiError(error) && error.status === 409) {
          const msg = error.message.toLowerCase();
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
            error instanceof Error ? error.message : "Fehler beim Erstellen.",
          );
          logger.error("school_create_failed", {
            error: error instanceof Error ? error.message : String(error),
          });
        }
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
        onSubmit={(e) => void handleCreate(e)}
        className="space-y-4"
        id="create-school-form"
      >
        <FormField label="Träger" htmlFor="school-org" required>
          <CustomSelect
            id="school-org"
            ariaLabel="Träger"
            value={schoolOrgId}
            options={
              organizations?.map((org) => ({
                value: org.id,
                label: org.name,
              })) ?? []
            }
            onChange={setSchoolOrgId}
            placeholder="Träger auswählen..."
            required
          />
        </FormField>
        <FormField label="Name" htmlFor="school-name" required>
          <input
            id="school-name"
            type="text"
            value={schoolName}
            onChange={(e) => handleSchoolNameChange(e.target.value)}
            maxLength={255}
            className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
            required
          />
        </FormField>
        <div className="grid grid-cols-2 gap-4">
          <FormField label="Slug" htmlFor="school-slug" required>
            <input
              id="school-slug"
              type="text"
              value={schoolSlug}
              onChange={(e) => {
                setSchoolSlugManual(true);
                setSchoolSlug(e.target.value);
              }}
              maxLength={100}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 font-mono text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              required
            />
          </FormField>
          <FormField label="Subdomain" htmlFor="school-subdomain" required>
            <input
              id="school-subdomain"
              type="text"
              value={schoolSubdomain}
              onChange={(e) => {
                setSchoolSubdomainManual(true);
                setSchoolSubdomain(e.target.value);
              }}
              maxLength={63}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 font-mono text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              required
            />
          </FormField>
        </div>

        <div className="border-t border-gray-100 pt-4">
          <p className="mb-3 text-xs font-medium text-gray-500 uppercase">
            Kontaktdaten (optional)
          </p>
          <div className="space-y-3">
            <FormField label="Adresse" htmlFor="school-address">
              <input
                id="school-address"
                type="text"
                value={schoolAddress}
                onChange={(e) => setSchoolAddress(e.target.value)}
                maxLength={255}
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              />
            </FormField>
            <div className="grid grid-cols-2 gap-4">
              <FormField label="PLZ" htmlFor="school-zip">
                <input
                  id="school-zip"
                  type="text"
                  value={schoolZip}
                  onChange={(e) => setSchoolZip(e.target.value)}
                  maxLength={10}
                  className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                />
              </FormField>
              <FormField label="Stadt" htmlFor="school-city">
                <input
                  id="school-city"
                  type="text"
                  value={schoolCity}
                  onChange={(e) => setSchoolCity(e.target.value)}
                  maxLength={255}
                  className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                />
              </FormField>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <FormField label="Telefon" htmlFor="school-phone">
                <input
                  id="school-phone"
                  type="tel"
                  value={schoolPhone}
                  onChange={(e) => setSchoolPhone(e.target.value)}
                  maxLength={30}
                  className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                />
              </FormField>
              <FormField label="E-Mail" htmlFor="school-email">
                <input
                  id="school-email"
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
