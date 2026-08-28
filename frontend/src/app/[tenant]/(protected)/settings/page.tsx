"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { TenantPage } from "~/components/ui/tenant-page";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { useSettingsTabs } from "~/components/settings/settings-page";

function SettingsContent() {
  const { data: session, status } = useSession({
    required: true,
  });

  const settingsTabs = useSettingsTabs();
  const searchParams = useSearchParams();
  const { data: schema } = useSettingsSchema();

  const requestedTab = searchParams.get("tab");
  const requestedTabId = requestedTab ? `settings-${requestedTab}` : null;
  const [selectedTab, setSelectedTab] = useState<string | null>(requestedTabId);

  useEffect(() => {
    if (!requestedTabId) return;
    setSelectedTab(requestedTabId);
  }, [requestedTabId]);

  const tabItems = useMemo(
    () =>
      (settingsTabs?.tabs ?? []).map((tab) => ({
        value: tab.id,
        label: tab.label,
      })),
    [settingsTabs],
  );

  // Statuszeile: wie viele Bereiche es gibt und wie viele Einstellungen von
  // der Vorgabe abweichen — beides aus dem ohnehin geladenen Schema.
  const overrides = useMemo(
    () =>
      (schema?.tabs ?? []).reduce(
        (sum, tab) =>
          sum +
          tab.categories.reduce(
            (inner, category) =>
              inner +
              category.items.filter((item) => item.visible && !item.is_default)
                .length,
            0,
          ),
        0,
      ),
    [schema],
  );

  if (status === "loading") {
    return <TenantPage title="Einstellungen" statsLoading loading />;
  }

  if (!session?.user) {
    redirect("/");
  }

  if (!settingsTabs || tabItems.length === 0) {
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

  const activeTab =
    selectedTab && tabItems.some((tab) => tab.value === selectedTab)
      ? selectedTab
      : (tabItems[0]?.value ?? "");

  return (
    <TenantPage
      title="Einstellungen"
      stats={`${tabItems.length} ${tabItems.length === 1 ? "Bereich" : "Bereiche"} · ${overrides} abweichend von der Vorgabe`}
      tabs={{
        value: activeTab,
        onChange: setSelectedTab,
        items: tabItems,
        label: "Einstellungsbereiche",
      }}
    >
      {/* Bauart „Einstellungen": die einzige Flaeche, die automatisch
          speichert. Das Verhalten steht einmal ruhig im Kopf der Flaeche und
          nicht als Banner ueber jeder Karte. */}
      <p className="text-sm leading-5 text-gray-500">
        Änderungen werden sofort gespeichert.
      </p>
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
