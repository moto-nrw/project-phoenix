"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { ChartLine } from "lucide-react";
import {
  germanAnalyticsConsentCopy,
  useParentAnalyticsConsentCopy,
  type AnalyticsConsentCopy,
} from "~/components/analytics/analytics-consent-copy";
import { BooleanField } from "~/components/settings/fields/boolean-field";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { useApiErrorDisplay, useApiLoadError } from "~/contexts/ToastContext";
import {
  analyticsConsentApplies,
  decideAnalyticsConsent,
  loadAnalyticsConsent,
  useAnalyticsConsent,
  type AnalyticsConsentPortal,
} from "~/lib/analytics-consent";
import { createLogger } from "~/lib/logger";
import { useTenant } from "~/lib/tenant-context";

const logger = createLogger({ component: "AnalyticsConsentSection" });

/**
 * The person's own consent to the usage analytics (#4901), changeable and
 * revocable at any time. A revocation stops the analytics at once, before it
 * is saved. The school's switch still decides whether anything runs; while it
 * is off, the card says so and keeps the person's choice for later.
 */
export function AnalyticsConsentSection({
  portal,
  copy,
  recording = false,
}: Readonly<{
  portal: AnalyticsConsentPortal;
  copy: AnalyticsConsentCopy;
  /** The school's Analyse-Freigabe records sessions; the card says so. */
  recording?: boolean;
}>) {
  const consent = useAnalyticsConsent();
  const [busy, setBusy] = useState(false);
  const { error: loadError, show: showLoadError } = useApiLoadError();
  const { show: showSaveError } = useApiErrorDisplay();
  const latestToggleRef = useRef<(next: boolean) => void>(() => undefined);
  const shownLoadErrorRef = useRef<unknown>(null);
  const failed = consent.portal === portal && consent.status === "error";

  useEffect(() => {
    if (!failed || shownLoadErrorRef.current === consent.error) return;
    shownLoadErrorRef.current = consent.error;
    void showLoadError(consent.error, {
      object: copy.errorObject,
      retry: () => void loadAnalyticsConsent(),
    });
  }, [failed, consent.error, copy.errorObject, showLoadError]);

  const toggle = async (granted: boolean) => {
    setBusy(true);
    try {
      await decideAnalyticsConsent(granted);
    } catch (err) {
      logger.warn("analytics_consent_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showSaveError(err, {
        object: copy.errorObject,
        retry: () => latestToggleRef.current(granted),
      });
    } finally {
      setBusy(false);
    }
  };

  useLayoutEffect(() => {
    latestToggleRef.current = (next) => void toggle(next);
  });

  // Without a portal session of this host (staff-view preview, operator) or
  // without analytics on this deployment there is nothing to decide.
  if (consent.portal !== portal || !analyticsConsentApplies()) return null;

  return (
    <SectionCard
      icon={ChartLine}
      headingLevel={3}
      title={copy.title}
      description={copy.sectionDescription}
    >
      {consent.status === "loading" || (failed && !loadError) ? (
        <Skeleton className="h-10 w-full" />
      ) : failed ? (
        <LoadErrorAlert error={loadError} />
      ) : (
        <div className="space-y-3">
          <div className="flex items-center justify-between gap-3 rounded-xl border border-gray-200/50 bg-gray-50/50 p-3">
            <span className="text-sm text-gray-800">{copy.switchLabel}</span>
            <BooleanField
              value={consent.granted === true}
              onChange={(next) => void toggle(next)}
              disabled={busy}
              ariaLabel={copy.switchLabel}
            />
          </div>
          {consent.schoolAllows ? null : (
            <p className="text-sm text-gray-600">{copy.schoolOff}</p>
          )}
          {recording && copy.recording ? (
            <p className="text-sm text-gray-600">{copy.recording}</p>
          ) : null}
        </div>
      )}
    </SectionCard>
  );
}

const TENANT_COPY = germanAnalyticsConsentCopy("Profil");
const SCHOOL_COPY = germanAnalyticsConsentCopy("Einstellungen");

/** OGS profile: also names the recordings of a school with Analyse-Freigabe. */
export function TenantAnalyticsConsentSection() {
  const { tenant } = useTenant();
  return (
    <AnalyticsConsentSection
      portal="tenant"
      copy={TENANT_COPY}
      recording={
        tenant?.analyticsEnabled === true && tenant.analyticsFreigabe === true
      }
    />
  );
}

export function SchoolAnalyticsConsentSection() {
  return <AnalyticsConsentSection portal="school" copy={SCHOOL_COPY} />;
}

export function ParentAnalyticsConsentSection() {
  return (
    <AnalyticsConsentSection
      portal="parent"
      copy={useParentAnalyticsConsentCopy()}
    />
  );
}
