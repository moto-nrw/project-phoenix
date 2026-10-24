"use client";

import { useState } from "react";
import { Button } from "~/components/ui/button";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Modal } from "~/components/ui/modal";
import { useApiFormError } from "~/contexts/ToastContext";
import type { AnalyticsConsentCopy } from "~/components/analytics/analytics-consent-copy";
import {
  analyticsConsentApplies,
  decideAnalyticsConsent,
  dismissAnalyticsConsentPrompt,
  useAnalyticsConsent,
  type AnalyticsConsentPortal,
} from "~/lib/analytics-consent";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AnalyticsConsentPrompt" });

/**
 * Asks once after login whether the usage analytics may run in this browser
 * (§ 25 TDDDG, #4901). Both answers carry the same weight and nothing is
 * preselected: the focus starts on the title, not on a button. It appears
 * only while the person has not decided and the school lets the analytics
 * run; closing it decides nothing, and nothing is collected until consent.
 */
export function AnalyticsConsentPrompt({
  portal,
  copy,
  recording = false,
}: Readonly<{
  portal: AnalyticsConsentPortal;
  copy: AnalyticsConsentCopy;
  /** The school's Analyse-Freigabe records sessions; the prompt says so. */
  recording?: boolean;
}>) {
  const consent = useAnalyticsConsent();
  const [saving, setSaving] = useState<boolean | null>(null);
  const errors = useApiFormError();
  const open =
    consent.portal === portal &&
    consent.status === "ready" &&
    consent.granted === null &&
    consent.schoolAllows &&
    !consent.dismissed &&
    analyticsConsentApplies();

  const decide = async (granted: boolean) => {
    setSaving(granted);
    errors.clear();
    try {
      await decideAnalyticsConsent(granted);
    } catch (err) {
      logger.warn("analytics_consent_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void errors.show(err, {
        object: copy.errorObject,
        retry: () => void decide(granted),
      });
    } finally {
      setSaving(null);
    }
  };

  return (
    <Modal
      isOpen={open}
      onClose={dismissAnalyticsConsentPrompt}
      title={copy.promptTitle}
      closeLabel={copy.closeLabel}
      isDismissDisabled={saving !== null}
      isBackdropDismissDisabled
      focusTitleOnOpen
      footer={
        <div className="grid w-full grid-cols-2 gap-3">
          <Button
            type="button"
            variant="outline"
            size="md"
            disabled={saving !== null}
            isLoading={saving === false}
            loadingText={copy.saving}
            onClick={() => void decide(false)}
          >
            {copy.refuse}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="md"
            disabled={saving !== null}
            isLoading={saving === true}
            loadingText={copy.saving}
            onClick={() => void decide(true)}
          >
            {copy.allow}
          </Button>
        </div>
      }
    >
      <FormErrorAlert message={errors.error} className="mb-4" />
      <div className="space-y-3 text-sm text-gray-700">
        <p>
          {copy.promptIntro} {copy.promptPrivacy}
        </p>
        {recording && copy.promptRecording ? (
          <p>{copy.promptRecording}</p>
        ) : null}
        <p>
          {copy.promptNoDisadvantage} {copy.promptChangeLater}
        </p>
      </div>
    </Modal>
  );
}
