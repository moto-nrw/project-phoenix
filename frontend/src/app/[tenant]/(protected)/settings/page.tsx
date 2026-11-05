"use client";

import { Suspense, useMemo } from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { Alert } from "~/components/ui/alert";
import { TenantPage } from "~/components/ui/tenant-page";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { activeSettingsSectionId } from "~/components/settings/settings-nav-items";
import { useSettingsTabs } from "~/components/settings/settings-page";

function SettingsContent() {
  const { data: session, status } = useSession({
    required: true,
  });

  const settingsTabs = useSettingsTabs();
  const searchParams = useSearchParams();
  const {
    data: schema,
    isLoading: schemaLoading,
    error: schemaError,
    mutate: reloadSchema,
  } = useSettingsSchema();
  // Ein gescheitertes Laden des Schemas ist ein Ladefehler des Gerüsts mit
  // Katalogtext und Wiederholen, nie "0 Bereiche" oder der Leerzustand
  // (#2517).
  const schemaLoadError = useSwrLoadError(
    schemaError,
    "die Liste der Einstellungen",
    () => reloadSchema(),
  );

  // Die Bereiche stehen in der Seitenleiste (#5750); offen ist, was die Adresse
  // nennt. So bleibt der Bereich beim Neuladen und in einem Link erhalten.
  const sections = settingsTabs?.tabs ?? [];
  const activeTab = settingsTabs
    ? activeSettingsSectionId(settingsTabs, searchParams.get("tab"))
    : "";

  // Statuszeile: wie viele Einstellungen dieses Bereichs von der Vorgabe
  // abweichen. Die handgeschriebenen Bereiche stehen nicht im Schema und haben
  // keine Vorgabe, also auch keine Zahl.
  const overrides = useMemo(() => {
    const tab = (schema?.tabs ?? []).find(
      (candidate) => `settings-${candidate.key}` === activeTab,
    );
    if (!tab) return null;
    return tab.categories.reduce(
      (sum, category) =>
        sum +
        category.items.filter((item) => item.visible && !item.is_default)
          .length,
      0,
    );
  }, [activeTab, schema]);

  if (status === "loading" || schemaLoading) {
    return <TenantPage title="Einstellungen" statsLoading loading />;
  }

  if (!session?.user) {
    redirect("/");
  }

  // Liegt noch ein früher geladenes Schema vor, bleibt die Seite stehen; ohne
  // Schema gibt es weder Bereiche noch Abweichungen zu zählen. Bis der
  // Katalogtext da ist, bleibt das Skelett stehen.
  if (schemaError && !schema) {
    return (
      <TenantPage
        title="Einstellungen"
        statsLoading={!schemaLoadError}
        loading={!schemaLoadError}
        error={schemaLoadError}
      />
    );
  }

  if (!settingsTabs || sections.length === 0) {
    return (
      <TenantPage
        title="Einstellungen"
        stats="0 Bereiche"
        empty={{
          title: "Keine Einstellungen verfügbar.",
          description:
            "Für Ihre Rolle ist hier nichts freigegeben. Wenden Sie sich an Ihre Leitung, wenn Sie Einstellungen ändern müssen.",
        }}
      />
    );
  }

  return (
    <TenantPage
      title={
        sections.find((section) => section.id === activeTab)?.label ??
        "Einstellungen"
      }
      stats={
        overrides === null
          ? undefined
          : `${overrides} abweichend von der Vorgabe`
      }
    >
      {settingsTabs.highlightNeedsNfc ? (
        <div className="mb-4">
          <Alert
            type="info"
            message="Diese Einstellung gibt es nur für Schulen mit NFC-Tablets. Ihre Schule nutzt keine NFC-Tablets."
          />
        </div>
      ) : null}
      {settingsTabs.renderTab(activeTab)}
    </TenantPage>
  );
}

export default function SettingsPage() {
  return (
    <Suspense
      fallback={<TenantPage title="Einstellungen" statsLoading loading />}
    >
      <SettingsContent />
    </Suspense>
  );
}
