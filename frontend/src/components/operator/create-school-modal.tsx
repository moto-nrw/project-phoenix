"use client";

import { useState, useCallback, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import {
  generateSlug,
  validateSlug,
  validateSubdomain,
  isSlugConflict,
  isSubdomainConflict,
} from "~/lib/operator/provisioning-helpers";
import type { Organization, School } from "~/lib/operator/provisioning-helpers";
import { isOperatorApiError } from "~/lib/operator/api-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CreateSchoolModal" });

interface CreateSchoolModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreated: (school: School) => void;
  readonly organizations: Organization[];
  readonly onCreateOrganization: () => void;
}

export function CreateSchoolModal({
  isOpen,
  onClose,
  onCreated,
  organizations,
  onCreateOrganization,
}: CreateSchoolModalProps) {
  const [organizationId, setOrganizationId] = useState("");
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugTouched, setSlugTouched] = useState(false);
  const [subdomain, setSubdomain] = useState("");
  const [subdomainTouched, setSubdomainTouched] = useState(false);
  const [address, setAddress] = useState("");
  const [city, setCity] = useState("");
  const [zip, setZip] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  // Reset form when modal opens
  useEffect(() => {
    if (isOpen) {
      setOrganizationId(
        organizations.length === 1 ? (organizations[0]?.id ?? "") : "",
      );
      setName("");
      setSlug("");
      setSlugTouched(false);
      setSubdomain("");
      setSubdomainTouched(false);
      setAddress("");
      setCity("");
      setZip("");
      setPhone("");
      setEmail("");
      setIsSaving(false);
      setErrors({});
    }
  }, [isOpen, organizations]);

  // Auto-generate slug from name
  useEffect(() => {
    if (!slugTouched && name) {
      setSlug(generateSlug(name));
    }
  }, [name, slugTouched]);

  // Auto-generate subdomain from slug
  useEffect(() => {
    if (!subdomainTouched && slug) {
      setSubdomain(slug.slice(0, 63));
    }
  }, [slug, subdomainTouched]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      const newErrors: Record<string, string> = {};

      if (!organizationId) {
        newErrors.organizationId = "Organisation ist erforderlich";
      }
      if (!name.trim()) {
        newErrors.name = "Name ist erforderlich";
      }

      const slugValidation = validateSlug(slug);
      if (slugValidation) newErrors.slug = slugValidation;

      const subdomainValidation = validateSubdomain(subdomain);
      if (subdomainValidation) newErrors.subdomain = subdomainValidation;

      if (Object.keys(newErrors).length > 0) {
        setErrors(newErrors);
        return;
      }

      setIsSaving(true);
      setErrors({});
      try {
        const school = await operatorProvisioningService.createSchool({
          organization_id: parseInt(organizationId, 10),
          name: name.trim(),
          slug,
          subdomain,
          ...(address.trim() && { address: address.trim() }),
          ...(city.trim() && { city: city.trim() }),
          ...(zip.trim() && { zip: zip.trim() }),
          ...(phone.trim() && { phone: phone.trim() }),
          ...(email.trim() && { email: email.trim().toLowerCase() }),
        });
        onCreated(school);
        onClose();
      } catch (error) {
        if (isOperatorApiError(error) && error.status === 409) {
          if (isSubdomainConflict(error.message)) {
            setErrors({ subdomain: "Subdomain ist bereits vergeben" });
          } else if (isSlugConflict(error.message)) {
            setErrors({ slug: "Slug ist bereits vergeben" });
          } else {
            setErrors({ slug: error.message });
          }
        } else {
          logger.error("school_create_failed", {
            error: error instanceof Error ? error.message : String(error),
          });
        }
      } finally {
        setIsSaving(false);
      }
    },
    [
      organizationId,
      name,
      slug,
      subdomain,
      address,
      city,
      zip,
      phone,
      email,
      onCreated,
      onClose,
    ],
  );

  // No organizations → show message
  if (isOpen && organizations.length === 0) {
    return (
      <Modal isOpen={isOpen} onClose={onClose} title="Neue Schule">
        <div className="flex flex-col items-center gap-3 py-6 text-center">
          <p className="text-sm text-gray-600">
            Es muss zuerst eine Organisation erstellt werden, bevor eine Schule
            angelegt werden kann.
          </p>
          <button
            type="button"
            onClick={() => {
              onClose();
              onCreateOrganization();
            }}
            className="mt-2 rounded-full bg-gray-900 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
          >
            Organisation erstellen
          </button>
        </div>
      </Modal>
    );
  }

  const fieldClass = (fieldName: string) =>
    `w-full rounded-lg border px-3 py-2 text-sm focus:ring-1 focus:outline-none ${
      errors[fieldName]
        ? "border-red-300 focus:border-red-500 focus:ring-red-500"
        : "border-gray-200 focus:border-blue-500 focus:ring-blue-500"
    }`;

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
            onClick={(e) => void handleSubmit(e)}
            disabled={
              isSaving || !name.trim() || !slug || !subdomain || !organizationId
            }
            className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {isSaving ? "Wird erstellt..." : "Erstellen"}
          </button>
        </>
      }
    >
      <form
        onSubmit={(e) => void handleSubmit(e)}
        className="space-y-4"
        id="create-school-form"
      >
        {/* Organization selector */}
        <div>
          <label
            htmlFor="school-org"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Organisation
          </label>
          <select
            id="school-org"
            value={organizationId}
            onChange={(e) => {
              setOrganizationId(e.target.value);
              setErrors((prev) => {
                const next = { ...prev };
                delete next.organizationId;
                return next;
              });
            }}
            className={fieldClass("organizationId")}
            required
          >
            <option value="">Organisation auswählen</option>
            {organizations.map((org) => (
              <option key={org.id} value={org.id}>
                {org.name}
              </option>
            ))}
          </select>
          {errors.organizationId && (
            <p className="mt-1 text-xs text-red-600">{errors.organizationId}</p>
          )}
        </div>

        {/* Name */}
        <div>
          <label
            htmlFor="school-name"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Schulname
          </label>
          <input
            id="school-name"
            type="text"
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              setErrors((prev) => {
                const next = { ...prev };
                delete next.name;
                return next;
              });
            }}
            className={fieldClass("name")}
            placeholder="z.B. GGS Mühlenberg"
            required
          />
          {errors.name && (
            <p className="mt-1 text-xs text-red-600">{errors.name}</p>
          )}
        </div>

        {/* Slug */}
        <div>
          <label
            htmlFor="school-slug"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Slug
          </label>
          <input
            id="school-slug"
            type="text"
            value={slug}
            onChange={(e) => {
              setSlug(e.target.value.toLowerCase());
              setSlugTouched(true);
              setErrors((prev) => {
                const next = { ...prev };
                delete next.slug;
                return next;
              });
            }}
            className={fieldClass("slug")}
            placeholder="z.B. ggs-muehlenberg"
            required
          />
          {errors.slug ? (
            <p className="mt-1 text-xs text-red-600">{errors.slug}</p>
          ) : (
            <p className="mt-1 text-xs text-gray-500">
              Wird automatisch aus dem Namen generiert
            </p>
          )}
        </div>

        {/* Subdomain */}
        <div>
          <label
            htmlFor="school-subdomain"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Subdomain
          </label>
          <div className="flex items-center gap-2">
            <input
              id="school-subdomain"
              type="text"
              value={subdomain}
              onChange={(e) => {
                setSubdomain(e.target.value.toLowerCase());
                setSubdomainTouched(true);
                setErrors((prev) => {
                  const next = { ...prev };
                  delete next.subdomain;
                  return next;
                });
              }}
              className={fieldClass("subdomain")}
              placeholder="z.B. ggs-muehlenberg"
              required
            />
            <span className="shrink-0 text-sm text-gray-500">.moto-app.de</span>
          </div>
          {errors.subdomain ? (
            <p className="mt-1 text-xs text-red-600">{errors.subdomain}</p>
          ) : (
            <p className="mt-1 text-xs text-gray-500">
              Wird automatisch aus dem Slug generiert
            </p>
          )}
        </div>

        {/* Optional fields */}
        <div className="border-t border-gray-100 pt-4">
          <p className="mb-3 text-xs font-medium text-gray-500 uppercase">
            Optional
          </p>
          <div className="space-y-3">
            <div>
              <label
                htmlFor="school-address"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Adresse
              </label>
              <input
                id="school-address"
                type="text"
                value={address}
                onChange={(e) => setAddress(e.target.value)}
                className={fieldClass("address")}
                placeholder="Musterstraße 1"
              />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label
                  htmlFor="school-zip"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  PLZ
                </label>
                <input
                  id="school-zip"
                  type="text"
                  value={zip}
                  onChange={(e) => setZip(e.target.value)}
                  className={fieldClass("zip")}
                  placeholder="50667"
                />
              </div>
              <div>
                <label
                  htmlFor="school-city"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Stadt
                </label>
                <input
                  id="school-city"
                  type="text"
                  value={city}
                  onChange={(e) => setCity(e.target.value)}
                  className={fieldClass("city")}
                  placeholder="Köln"
                />
              </div>
            </div>

            <div>
              <label
                htmlFor="school-phone"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Telefon
              </label>
              <input
                id="school-phone"
                type="tel"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                className={fieldClass("phone")}
                placeholder="0221 12345"
              />
            </div>

            <div>
              <label
                htmlFor="school-email"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                E-Mail
              </label>
              <input
                id="school-email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className={fieldClass("email")}
                placeholder="info@schule.de"
              />
            </div>
          </div>
        </div>
      </form>
    </Modal>
  );
}
