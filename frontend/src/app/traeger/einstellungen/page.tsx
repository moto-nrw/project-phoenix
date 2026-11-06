"use client";

// Allgemeine Einstellungen des Trägers (#4652, ADR 0086, #5685): Werte, die
// für alle Schulen des Trägers gelten, und die Kontaktdaten. Eine Schule kann
// abweichen, wo die Einstellung das erlaubt; Schul-Abweichungen ändert nur die
// Schule selbst.

import { Suspense, useCallback, useState } from "react";
import { useSearchParams } from "next/navigation";
import { SettingsCategory } from "~/components/settings/settings-category";
import { displayTabLabel } from "~/components/settings/settings-filter";
import { Alert } from "~/components/ui/alert";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { createLogger } from "~/lib/logger";
import type {
  ResolvedSetting,
  SchemaTab,
  SettingsSchema,
} from "~/lib/settings-api";
import {
  clearTraegerSettingValue,
  saveTraegerSettingValue,
} from "~/lib/traeger-api";
import { useTraegerSettingsSchema } from "~/components/traeger/use-traeger-resources";
import { useTraegerMasterData } from "~/components/traeger/use-traeger-master-data";
import { TRAEGER_BRANDING_TAB } from "~/lib/traeger-branding";
import { metadataLoadError } from "~/components/traeger/metadata-load-error";
import { TraegerContactSection } from "~/components/traeger/traeger-contact-section";
import { SectionCard } from "~/components/ui/section-card";
import {
  DEBTOR_NUMBER_ON_CONFIRM_KEY,
  PAYMENT_TEXT_KEY,
  TRAEGER_HOLIDAY_TAB,
  paymentTextPreview,
} from "~/lib/traeger-payment-text";

const logger = createLogger({ component: "TraegerSettingsPage" });

/**
 * Opens a settings area by its key, e.g. `?bereich=dashboard` from the hints
 * of the dashboard (#4675). An unknown key opens the first area.
 */
const AREA_PARAM = "bereich";

/** Der Bereich mit den Kontaktdaten; er liegt nicht im Einstellungs-Schema. */
const CONTACT_TAB = "kontakt";

function tabItem(tab: SchemaTab, key: string): ResolvedSetting | undefined {
  return tab.categories
    .flatMap((category) => category.items)
    .find((item) => item.key === key);
}

function hasWritableSetting(schema: SettingsSchema): boolean {
  return schema.tabs.some((tab) =>
    tab.categories.some((category) =>
      category.items.some((item) => item.writable),
    ),
  );
}

export default function TraegerSettingsPage() {
  // `useSearchParams` braucht eine Suspense-Grenze; bis dahin lädt die Seite.
  return (
    <Suspense fallback={<TenantPage title="Allgemein" loading />}>
      <TraegerSettingsContent />
    </Suspense>
  );
}

function TraegerSettingsContent() {
  const searchParams = useSearchParams();
  const {
    data: schema,
    isLoading: loading,
    loadError,
    mutate,
  } = useTraegerSettingsSchema();
  const [selectedTab, setSelectedTab] = useState<string | null>(
    () => searchParams?.get(AREA_PARAM) ?? null,
  );
  const refresh = useCallback(() => mutate().catch(() => undefined), [mutate]);

  // Save and reset throw the ApiError of a failed request; the field shows
  // it on the shared error path.
  const handleSave = useCallback(
    async (key: string, value: unknown) => {
      await saveTraegerSettingValue(key, value);
      logger.info("traeger_setting_saved", { key });
      void refresh();
    },
    [refresh],
  );

  const handleReset = useCallback(
    async (key: string) => {
      await clearTraegerSettingValue(key);
      logger.info("traeger_setting_reset", { key });
      void refresh();
    },
    [refresh],
  );

  // The branding has its own page with the preview (#4653).
  const tabs = (schema?.tabs ?? []).filter(
    (tab) => tab.key !== TRAEGER_BRANDING_TAB,
  );

  const contactOpen = selectedTab === CONTACT_TAB;
  const contactActive =
    contactOpen || (schema !== undefined && tabs.length === 0);
  const activeTab = contactActive
    ? null
    : (tabs.find((tab) => tab.key === selectedTab) ?? tabs[0] ?? null);
  const contact = useTraegerMasterData("die Kontaktdaten", contactActive);
  // Ein fehlgeschlagenes Nachladen lässt die Kontaktkarte stehen, außer bei einer Ablehnung.
  const contactError = contactActive
    ? metadataLoadError(contact.error, contact.loaded, contact.failure)
    : null;
  // The holiday tab shows the payment sentence with example values (#5622).
  const paymentText =
    activeTab?.key === TRAEGER_HOLIDAY_TAB
      ? tabItem(activeTab, PAYMENT_TEXT_KEY)
      : undefined;
  const fromTraeger = tabs
    .flatMap((tab) => tab.categories)
    .flatMap((category) => category.items)
    .filter((item) => item.source === "traeger").length;
  return (
    <TenantPage
      title="Allgemein"
      info="Hier legen Sie Werte für alle Schulen Ihres Trägers fest. Wo es erlaubt ist, kann eine Schule einen eigenen Wert eintragen."
      stats={
        tabs.length > 0 ? (
          <TenantPageStats
            items={[
              // Die Bereiche der Einstellungen und „Kontakt“.
              { value: tabs.length + 1, label: "Bereiche" },
              { value: fromTraeger, label: "vom Träger festgelegt" },
            ]}
          />
        ) : null
      }
      statsLoading={loading && !schema}
      tabs={
        schema
          ? {
              value: activeTab?.key ?? CONTACT_TAB,
              onChange: setSelectedTab,
              label: "Einstellungsbereiche",
              items: [
                ...tabs.map((tab) => ({
                  value: tab.key,
                  label: displayTabLabel(tab),
                })),
                { value: CONTACT_TAB, label: "Kontakt" },
              ],
            }
          : undefined
      }
      error={loadError ?? contactError}
      loading={
        (loading && !schema) ||
        (contactActive && contact.loading && !contact.loaded)
      }
    >
      {schema && activeTab && !hasWritableSetting(schema) ? (
        <Alert
          type="info"
          message="Sie können die Werte nur ansehen. Ändern kann, wer im Träger-Büro das Recht dazu hat."
        />
      ) : null}
      {schema && contactActive && contact.loaded ? (
        <TraegerContactSection
          contact={contact.masterData.contact}
          onSaved={contact.refresh}
        />
      ) : null}
      {activeTab?.categories.map((category) => (
        <SettingsCategory
          key={category.key}
          category={category}
          onSave={handleSave}
          onReset={handleReset}
          onSchemaRefresh={() => void refresh()}
          audience="traeger"
        />
      ))}
      {activeTab && paymentText ? (
        // The payment sentence with example values, bold as the letter
        // prints it; a display only, the text changes in the field above.
        <SectionCard
          title="Vorschau des Zahlungstexts"
          description="So steht der Text im Schreiben, mit Beispielwerten. Bei 0 € steht dort: „Sie müssen für diese Betreuung nichts bezahlen.“"
        >
          <figure aria-label="Vorschau des Zahlungstexts">
            <p className="font-semibold wrap-anywhere whitespace-pre-line text-gray-900">
              {paymentTextPreview(
                typeof paymentText.value === "string" ? paymentText.value : "",
                tabItem(activeTab, DEBTOR_NUMBER_ON_CONFIRM_KEY)?.value ===
                  true,
              )}
            </p>
          </figure>
        </SectionCard>
      ) : null}
    </TenantPage>
  );
}
