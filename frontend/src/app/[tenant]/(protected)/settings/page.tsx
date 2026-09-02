"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { TenantPage } from "~/components/ui/tenant-page";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { useSettingsTabs } from "~/components/settings/settings-page";
import { useTenantRouter } from "~/lib/tenant-router";

function SettingsContent() {
  const { data: session, status } = useSession({
    required: true,
  });

  const settingsTabs = useSettingsTabs();
  const router = useTenantRouter();
  const searchParams = useSearchParams();
  const { data: schema } = useSettingsSchema();

  const requestedTab = searchParams.get("tab");
  const requestedTabId = requestedTab ? `settings-${requestedTab}` : null;
  const [selectedTab, setSelectedTab] = useState<string | null>(requestedTabId);

  useEffect(() => {
    if (!requestedTabId) return;
    setSelectedTab(requestedTabId);
  }, [requestedTabId]);

  // Alle Bereiche stehen als Reiter da. Was nicht in die Zeile passt, raeumt
  // das Seitengeruest selbst unter „Mehr" -- gemessen, nicht geraten.
  const { tabItems, flatTabItems } = useMemo(() => {
    const schemaItems = (settingsTabs?.tabs ?? []).map((tab) => ({
      value: tab.id,
      label: tab.label,
    }));
    return { tabItems: schemaItems, flatTabItems: schemaItems };
  }, [settingsTabs]);

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

  if (!settingsTabs || flatTabItems.length === 0) {
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
    selectedTab && flatTabItems.some((tab) => tab.value === selectedTab)
      ? selectedTab
      : (flatTabItems[0]?.value ?? "");

  return (
    <TenantPage
      title="Einstellungen"
      stats={`${flatTabItems.length} ${flatTabItems.length === 1 ? "Bereich" : "Bereiche"} · ${overrides} abweichend von der Vorgabe`}
      tabs={{
        value: activeTab,
        onChange: (value) => {
          // Ein Reiter, der auf eine eigene Seite zeigt, navigiert dorthin.
          if (value.startsWith("/")) {
            router.push(value);
            return;
          }
          setSelectedTab(value);
        },
        items: tabItems,
        label: "Einstellungsbereiche",
      }}
    >
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
