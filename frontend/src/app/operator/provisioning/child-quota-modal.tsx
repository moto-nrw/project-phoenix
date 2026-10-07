import { useCallback, useEffect, useRef, useState } from "react";
import { Modal } from "~/components/ui/modal";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiErrorDisplay, useApiFormError } from "~/contexts/ToastContext";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type {
  SchoolSummary,
  UpdateSchoolRequest,
} from "~/lib/operator/provisioning-helpers";
import {
  CHILD_QUOTA_DEFAULT_BUNDLE_SIZE,
  childQuotaLimit,
  childQuotaWarning,
  validateChildQuotaDraft,
} from "~/lib/operator/child-quota";
import { formatCount } from "~/lib/format-utils";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ChildQuotaModal" });

// Draft field → control name, which is also the backend field name.
const FIELD_NAMES = { bundles: "bundles", bundleSize: "bundle_size" } as const;

// The school update route replaces every school field, so a quota change
// sends the school's current values with it.
function schoolUpdate(
  school: SchoolSummary,
  childQuota: UpdateSchoolRequest["child_quota"],
): UpdateSchoolRequest {
  return {
    organization_id: parseInt(school.organizationId, 10),
    name: school.name,
    slug: school.slug,
    subdomain: school.subdomain,
    address: school.address,
    city: school.city,
    zip: school.zip,
    phone: school.phone,
    email: school.email,
    active: school.active,
    hidden: school.hidden,
    child_quota: childQuota,
  };
}

/**
 * Kinderkontingent of one school (#3568): the moto team books whole bundles
 * of children per contract. The Kontingentzahl stands next to the result, a
 * limit below it warns but still saves, and removing the limit asks first.
 */
export function ChildQuotaModal({
  isOpen,
  onClose,
  school,
  loadCurrentSchool,
  onUpdated,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly school: SchoolSummary;
  /**
   * Reads the school fresh right before saving. The update route replaces
   * every school field, so saving from the copy the page loaded earlier
   * would undo a rename or status change made in the meantime.
   */
  readonly loadCurrentSchool: () => Promise<SchoolSummary>;
  readonly onUpdated: () => Promise<void>;
}) {
  const [bundles, setBundles] = useState("");
  const [bundleSize, setBundleSize] = useState("");
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, invalid, clear: clearError } = formErrors;
  const { show: showRefreshError } = useApiErrorDisplay();
  const [saving, setSaving] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState(false);

  const currentLimit = childQuotaLimit({
    bundles: school.childQuotaBundles,
    bundleSize: school.childQuotaBundleSize,
  });

  useEffect(() => {
    if (!isOpen) return;
    setBundles(school.childQuotaBundles?.toString() ?? "");
    setBundleSize(
      (
        school.childQuotaBundleSize ?? CHILD_QUOTA_DEFAULT_BUNDLE_SIZE
      ).toString(),
    );
    clearError();
    setConfirmRemove(false);
  }, [
    isOpen,
    school.childQuotaBundles,
    school.childQuotaBundleSize,
    clearError,
  ]);

  const draft = validateChildQuotaDraft({ bundles, bundleSize });
  const warning = draft.ok
    ? childQuotaWarning(draft.limit, school.childQuotaCount)
    : null;

  const save = useCallback(
    async (childQuota: UpdateSchoolRequest["child_quota"]) => {
      setSaving(true);
      clearError();
      try {
        const current = await loadCurrentSchool();
        await operatorProvisioningService.updateSchool(
          current.id,
          schoolUpdate(current, childQuota),
        );
      } catch (err) {
        logger.error("child_quota_update_failed", {
          school_id: school.id,
          error: err instanceof Error ? err.message : String(err),
        });
        void showError(err, { object: "die Änderung am Kinderkontingent" });
        setSaving(false);
        return;
      }

      try {
        await onUpdated();
      } catch (err) {
        logger.error("child_quota_refresh_failed", {
          school_id: school.id,
          error: err instanceof Error ? err.message : String(err),
        });
        // Saved, but the page still shows the old figures.
        void showRefreshError(err, {
          object: "die Liste der Schulen",
          retry: () => void onUpdated(),
        });
      } finally {
        setSaving(false);
      }
      onClose();
    },
    [
      loadCurrentSchool,
      onClose,
      onUpdated,
      school.id,
      clearError,
      showError,
      showRefreshError,
    ],
  );

  const handleSubmit = useCallback(
    (event?: React.SyntheticEvent) => {
      event?.preventDefault();
      const result = validateChildQuotaDraft({ bundles, bundleSize });
      if (!result.ok) {
        invalid("Bitte prüfen Sie die markierten Felder.", {
          [FIELD_NAMES[result.field]]: result.error,
        });
        return;
      }
      void save({ bundles: result.bundles, bundle_size: result.bundleSize });
    },
    [bundleSize, bundles, save, invalid],
  );

  if (confirmRemove) {
    return (
      <ConfirmDeleteModal
        isOpen={isOpen}
        title="Kinderkontingent entfernen"
        description={
          <p>
            Danach gilt für <span className="font-medium">{school.name}</span>{" "}
            keine Grenze mehr. Die Schule kann wieder beliebig viele Kinder
            aufnehmen.
          </p>
        }
        gate={{ mode: "twoStep", firstStepLabel: "Entfernen" }}
        confirmLabel="Kinderkontingent entfernen"
        loadingLabel="Wird entfernt…"
        onConfirm={() => save(null)}
        onClose={() => {
          clearError();
          setConfirmRemove(false);
        }}
        loading={saving}
        error={formErrors.error}
      />
    );
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Kinderkontingent"
      footer={
        <>
          {currentLimit !== null && (
            <Button
              type="button"
              variant="outline_danger"
              size="md"
              onClick={() => {
                clearError();
                setConfirmRemove(true);
              }}
              disabled={saving}
              className="mr-auto"
            >
              Entfernen
            </Button>
          )}
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={saving}
          >
            Abbrechen
          </Button>
          <Button
            type="submit"
            form="child-quota-form"
            variant="primary"
            size="md"
            disabled={saving}
          >
            {saving ? "Wird gespeichert…" : "Speichern"}
          </Button>
        </>
      }
    >
      <form
        ref={formRef}
        id="child-quota-form"
        onSubmit={handleSubmit}
        noValidate
        className="space-y-4"
      >
        <FormErrorAlert message={formErrors.error} />
        <p className="text-sm text-gray-600">
          So viele Kinder darf {school.name} laut Vertrag verwalten. Gebucht
          wird in ganzen Bundles.
        </p>
        <div className="grid grid-cols-2 gap-4">
          <Input
            id="child-quota-bundles"
            name="bundles"
            label="Anzahl Bundles"
            inputMode="numeric"
            autoComplete="off"
            value={bundles}
            onChange={(event) => setBundles(event.target.value)}
            error={formErrors.fieldError("bundles")}
            controlSize="compact"
          />
          <Input
            id="child-quota-bundle-size"
            name="bundle_size"
            label="Kinder pro Bundle"
            inputMode="numeric"
            autoComplete="off"
            value={bundleSize}
            onChange={(event) => setBundleSize(event.target.value)}
            error={formErrors.fieldError("bundle_size")}
            controlSize="compact"
          />
        </div>
        <dl className="grid grid-cols-2 gap-4 rounded-lg bg-gray-50 px-4 py-3">
          <div>
            <dt className="text-xs text-gray-500">Kinderkontingent</dt>
            <dd
              className="text-base font-semibold text-gray-900"
              data-testid="child-quota-limit"
            >
              {draft.ok ? `${formatCount(draft.limit)} Kinder` : "–"}
            </dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500">Kontingentzahl</dt>
            <dd
              className="text-base font-semibold text-gray-900"
              data-testid="child-quota-count"
            >
              {formatCount(school.childQuotaCount)} Kinder
            </dd>
          </div>
        </dl>
        {warning && <Alert type="warning" message={warning} />}
      </form>
    </Modal>
  );
}
