"use client";

import { useCallback } from "react";
import { operatorProvisioningKeys } from "~/lib/swr/portal-keys";
import { useSWRAuth } from "~/lib/swr";
import { SettingsCategory } from "~/components/settings/settings-category";
import { displayTabLabel } from "~/components/settings/settings-filter";
import { Alert } from "~/components/ui/alert";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import {
  fetchOperatorTraegerSettingsSchema,
  resetOperatorTraegerSettingValue,
  setOperatorTraegerSettingValue,
} from "~/lib/operator/operator-settings-api";
import { settingsConcept } from "~/lib/concepts/settings";
import type { ResolvedSetting, SchemaTab } from "~/lib/settings-api";
import {
  DEBTOR_NUMBER_ON_CONFIRM_KEY,
  PAYMENT_TEXT_KEY,
  TRAEGER_HOLIDAY_TAB,
  paymentTextPreview,
} from "~/lib/traeger-payment-text";

const logger = createLogger({ component: "OperatorTraegerSettingsSection" });

function tabItem(tab: SchemaTab, key: string): ResolvedSetting | undefined {
  return tab.categories
    .flatMap((category) => category.items)
    .find((item) => item.key === key);
}

/**
 * Settings of a Träger in the operator dashboard (#5748): the fields, texts
 * and checks of `/traeger/einstellungen`, for setting up a Träger before
 * anyone of its office has signed in. The backend names the operator in the
 * Träger's change log and in the operator audit.
 */
export function TraegerSettingsSection({
  organizationId,
}: {
  readonly organizationId: string;
}) {
  const { data, error, isLoading, mutate } = useSWRAuth(
    operatorProvisioningKeys.orgTraegerSettings(organizationId),
    () => fetchOperatorTraegerSettingsSchema(organizationId),
    { keepPreviousData: false },
  );
  const loadError = useSwrLoadError(
    error,
    "die Einstellungen des Trägers",
    () => void mutate(),
  );
  // The catch stays silent: SWR keeps the failed reload's error, and
  // useSwrLoadError shows it with a retry in place of the tab.
  const refresh = useCallback(
    () => void mutate().catch(() => undefined),
    [mutate],
  );

  // Save and reset throw the ApiError of a failed request; the field shows
  // it on the shared error path.
  const handleSave = useCallback(
    async (key: string, value: unknown) => {
      await setOperatorTraegerSettingValue(organizationId, key, value);
      logger.info("operator_traeger_setting_saved", {
        organization_id: organizationId,
        key,
      });
      refresh();
    },
    [organizationId, refresh],
  );
  const handleReset = useCallback(
    async (key: string) => {
      await resetOperatorTraegerSettingValue(organizationId, key);
      logger.info("operator_traeger_setting_reset", {
        organization_id: organizationId,
        key,
      });
      refresh();
    },
    [organizationId, refresh],
  );

  if (loadError) {
    return <LoadErrorAlert error={loadError} />;
  }
  if (isLoading || data === undefined) {
    return (
      <SectionCard title="Einstellungen des Trägers">
        <Skeleton shape="inner" className="h-20 w-full" />
      </SectionCard>
    );
  }
  if (data.tabs.length === 0) {
    return (
      <SectionCard title="Einstellungen des Trägers">
        <p className="text-sm text-gray-600">
          Für Träger gibt es keine Einstellungen.
        </p>
      </SectionCard>
    );
  }

  return (
    <div className="space-y-8">
      <Alert
        type="info"
        message="Sie ändern hier die Werte des Trägers für alle seine Schulen, so wie im Träger-Portal. moto speichert zu jeder Änderung, dass Sie sie als Operator gemacht haben."
      />
      {data.tabs.map((tab) => {
        const paymentText =
          tab.key === TRAEGER_HOLIDAY_TAB
            ? tabItem(tab, PAYMENT_TEXT_KEY)
            : undefined;
        return (
          <SectionCard
            key={tab.key}
            bare
            title={displayTabLabel(tab)}
            concept={settingsConcept}
          >
            <div className="space-y-6">
              {tab.categories.map((category) => (
                <SettingsCategory
                  key={category.key}
                  category={category}
                  onSave={handleSave}
                  onReset={handleReset}
                  onSchemaRefresh={refresh}
                  audience="traeger"
                />
              ))}
              {paymentText ? (
                // The payment sentence with example values, as on the
                // Träger portal; a display only.
                <SectionCard
                  title="Vorschau des Zahlungstexts"
                  description="So steht der Text im Schreiben, mit Beispielwerten. Bei 0 € steht dort: „Sie müssen für diese Betreuung nichts bezahlen.“"
                >
                  <figure aria-label="Vorschau des Zahlungstexts">
                    <p className="font-semibold wrap-anywhere whitespace-pre-line text-gray-900">
                      {paymentTextPreview(
                        typeof paymentText.value === "string"
                          ? paymentText.value
                          : "",
                        tabItem(tab, DEBTOR_NUMBER_ON_CONFIRM_KEY)?.value ===
                          true,
                      )}
                    </p>
                  </figure>
                </SectionCard>
              ) : null}
            </div>
          </SectionCard>
        );
      })}
    </div>
  );
}
