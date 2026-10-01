"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { Alert } from "~/components/ui/alert";
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
  const { data: schema, isLoading: schemaLoading } = useSettingsSchema();

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
      (schema?.tabs ?? [])
        .filter((tab) =>
          flatTabItems.some((item) => item.value === `settings-${tab.key}`),
        )
        .reduce(
          (sum, tab) =>
            sum +
            tab.categories.reduce(
              (inner, category) =>
                inner +
                category.items.filter(
                  (item) => item.visible && !item.is_default,
                ).length,
              0,
            ),
          0,
        ),
    [flatTabItems, schema],
  );

  if (status === "loading" || schemaLoading) {
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

  // A deep link names a setting; when its tab is not named or not shown, the
  // tab that holds the setting wins over the first tab.
  const isShown = (tabId: string | null): tabId is string =>
    tabId !== null && flatTabItems.some((tab) => tab.value === tabId);
  let activeTab = flatTabItems[0]?.value ?? "";
  if (isShown(selectedTab)) {
    activeTab = selectedTab;
  } else if (isShown(settingsTabs.highlightTabId)) {
    activeTab = settingsTabs.highlightTabId;
  }

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
