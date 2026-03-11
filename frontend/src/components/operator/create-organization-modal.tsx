"use client";

import { useState, useCallback, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import {
  generateSlug,
  validateSlug,
  isSlugConflict,
} from "~/lib/operator/provisioning-helpers";
import type { Organization } from "~/lib/operator/provisioning-helpers";
import { isOperatorApiError } from "~/lib/operator/api-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CreateOrganizationModal" });

interface CreateOrganizationModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreated: (org: Organization) => void;
}

export function CreateOrganizationModal({
  isOpen,
  onClose,
  onCreated,
}: CreateOrganizationModalProps) {
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugTouched, setSlugTouched] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [nameError, setNameError] = useState<string | null>(null);
  const [slugError, setSlugError] = useState<string | null>(null);

  // Reset form when modal opens
  useEffect(() => {
    if (isOpen) {
      setName("");
      setSlug("");
      setSlugTouched(false);
      setIsSaving(false);
      setNameError(null);
      setSlugError(null);
    }
  }, [isOpen]);

  // Auto-generate slug from name
  useEffect(() => {
    if (!slugTouched && name) {
      setSlug(generateSlug(name));
    }
  }, [name, slugTouched]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      setNameError(null);
      setSlugError(null);

      if (!name.trim()) {
        setNameError("Name ist erforderlich");
        return;
      }

      const slugValidation = validateSlug(slug);
      if (slugValidation) {
        setSlugError(slugValidation);
        return;
      }

      setIsSaving(true);
      try {
        const org = await operatorProvisioningService.createOrganization({
          name: name.trim(),
          slug,
        });
        onCreated(org);
        onClose();
      } catch (error) {
        if (isOperatorApiError(error) && error.status === 409) {
          if (isSlugConflict(error.message)) {
            setSlugError("Slug ist bereits vergeben");
          } else {
            setSlugError(error.message);
          }
        } else {
          logger.error("organization_create_failed", {
            error: error instanceof Error ? error.message : String(error),
          });
        }
      } finally {
        setIsSaving(false);
      }
    },
    [name, slug, onCreated, onClose],
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Neue Organisation"
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
            disabled={isSaving || !name.trim() || !slug}
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
        id="create-org-form"
      >
        <div>
          <label
            htmlFor="org-name"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Name
          </label>
          <input
            id="org-name"
            type="text"
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              setNameError(null);
            }}
            className={`w-full rounded-lg border px-3 py-2 text-sm focus:ring-1 focus:outline-none ${
              nameError
                ? "border-red-300 focus:border-red-500 focus:ring-red-500"
                : "border-gray-200 focus:border-blue-500 focus:ring-blue-500"
            }`}
            placeholder="z.B. Schulverband Köln"
            required
          />
          {nameError && (
            <p className="mt-1 text-xs text-red-600">{nameError}</p>
          )}
        </div>

        <div>
          <label
            htmlFor="org-slug"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Slug
          </label>
          <input
            id="org-slug"
            type="text"
            value={slug}
            onChange={(e) => {
              setSlug(e.target.value.toLowerCase());
              setSlugTouched(true);
              setSlugError(null);
            }}
            className={`w-full rounded-lg border px-3 py-2 text-sm focus:ring-1 focus:outline-none ${
              slugError
                ? "border-red-300 focus:border-red-500 focus:ring-red-500"
                : "border-gray-200 focus:border-blue-500 focus:ring-blue-500"
            }`}
            placeholder="z.B. schulverband-koeln"
            required
          />
          {slugError ? (
            <p className="mt-1 text-xs text-red-600">{slugError}</p>
          ) : (
            <p className="mt-1 text-xs text-gray-500">
              Wird automatisch aus dem Namen generiert
            </p>
          )}
        </div>
      </form>
    </Modal>
  );
}
