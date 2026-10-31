"use client";

// Einstellungen des Trägers (#4652, ADR 0086): Werte, die für alle Schulen
// des Trägers gelten. Eine Schule kann abweichen, wo die Einstellung das
// erlaubt; Schul-Abweichungen ändert nur die Schule selbst.

import { useCallback, useState } from "react";
import { SettingsCategory } from "~/components/settings/settings-category";
import { displayTabLabel } from "~/components/settings/settings-filter";
import { Alert } from "~/components/ui/alert";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { createLogger } from "~/lib/logger";
import type { SettingsSchema } from "~/lib/settings-api";
import {
  clearTraegerSettingValue,
  saveTraegerSettingValue,
} from "~/lib/traeger-api";
import { useTraegerSettingsSchema } from "~/components/traeger/use-traeger-resources";
import { TRAEGER_BRANDING_TAB } from "~/lib/traeger-branding";

const logger = createLogger({ component: "TraegerSettingsPage" });

function hasWritableSetting(schema: SettingsSchema): boolean {
  return schema.tabs.some((tab) =>
    tab.categories.some((category) =>
      category.items.some((item) => item.writable),
    ),
  );
}

export default function TraegerSettingsPage() {
  const {
    data: schema,
    isLoading: loading,
    loadError,
    mutate,
  } = useTraegerSettingsSchema();
  const [selectedTab, setSelectedTab] = useState<string | null>(null);
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

  // The branding has its own page "Briefkopf" with the preview (#4653).
  const tabs = (schema?.tabs ?? []).filter(
    (tab) => tab.key !== TRAEGER_BRANDING_TAB,
  );

  const activeTab =
    tabs.find((tab) => tab.key === selectedTab) ?? tabs[0] ?? null;
  const fromTraeger = tabs
    .flatMap((tab) => tab.categories)
    .flatMap((category) => category.items)
    .filter((item) => item.source === "traeger").length;
  return (
    <TenantPage
      title="Einstellungen"
      info="Hier legen Sie Werte für alle Schulen Ihres Trägers fest. Wo es erlaubt ist, kann eine Schule einen eigenen Wert eintragen."
      stats={
        tabs.length > 0 ? (
          <TenantPageStats
            items={[
              {
                value: tabs.length,
                label: tabs.length === 1 ? "Bereich" : "Bereiche",
              },
              { value: fromTraeger, label: "vom Träger festgelegt" },
            ]}
          />
        ) : null
      }
      statsLoading={loading && !schema}
      tabs={
        activeTab
          ? {
              value: activeTab.key,
              onChange: setSelectedTab,
              label: "Einstellungsbereiche",
              items: tabs.map((tab) => ({
                value: tab.key,
                label: displayTabLabel(tab),
              })),
            }
          : undefined
      }
      error={loadError}
      loading={loading && !schema}
      empty={
        !loading && schema && tabs.length === 0
          ? {
              title: "Es gibt noch keine Einstellungen für den Träger.",
              description:
                "Die Schulen legen ihre Einstellungen bis dahin selbst fest.",
            }
          : null
      }
    >
      {schema && tabs.length > 0 && !hasWritableSetting(schema) ? (
        <Alert
          type="info"
          message="Sie können die Werte nur ansehen. Ändern kann, wer im Träger-Büro das Recht dazu hat."
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
    </TenantPage>
  );
}
