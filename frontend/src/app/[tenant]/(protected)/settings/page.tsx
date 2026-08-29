"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { useSettingsTabs } from "~/components/settings/settings-page";
import { useTenantRouter } from "~/lib/tenant-router";
import { SETTINGS_REGISTER_TABS } from "~/lib/section-navigation";

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

  // Die Register, die reine Konfiguration sind (Gruppen, Rollen,
  // Berechtigungen, Geräte, Info-Displays, Exporte, Jahrgangswechsel), standen
  // im aufgelösten Bereich „Datenverwaltung". Sie sind jetzt Reiter hier und
  // führen auf ihre bestehenden Routen.
  //
  // Höchstens vier Seitenreiter: die ersten Einstellungsbereiche stehen
  // sichtbar, alles Seltenere (weitere Bereiche und die Register) bündelt ein
  // Reiter „Verwaltung" mit Menü. Mehr Reiter wären eine Werkzeugleiste,
  // keine Orientierung.
  const { tabItems, flatTabItems } = useMemo(() => {
    const schemaItems = (settingsTabs?.tabs ?? []).map((tab) => ({
      value: tab.id,
      label: tab.label,
    }));
    const registerItems = SETTINGS_REGISTER_TABS.map((tab) => ({
      value: tab.href,
      label: tab.label,
    }));
    const visibleSchemaItems = schemaItems.slice(0, 3);
    const menuItems = [...schemaItems.slice(3), ...registerItems];
    const items = [
      ...visibleSchemaItems,
      ...(menuItems.length > 0
        ? [{ value: "verwaltung", label: "Verwaltung", menu: menuItems }]
        : []),
    ];
    return {
      tabItems: items,
      flatTabItems: [...schemaItems, ...registerItems],
    };
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
      {/* Bauart „Einstellungen": die einzige Flaeche, die automatisch
          speichert. Das Verhalten steht einmal ruhig auf einer eigenen
          Flaeche und nicht als Banner ueber jeder Karte — und nicht als
          freier Text auf dem Grund. */}
      <SectionCard className="px-5 py-3">
        <p className="text-sm leading-5 text-gray-500">
          Änderungen werden sofort gespeichert.
        </p>
      </SectionCard>
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
