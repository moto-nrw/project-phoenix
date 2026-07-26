"use client";

import { Suspense } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { Loading } from "~/components/ui/loading";
import { SettingsLayout } from "~/components/shared/settings-layout";
import { useSettingsTabs } from "~/components/settings/settings-page";

function SettingsContent() {
  const { data: session, status } = useSession({
    required: true,
  });

  const settingsTabs = useSettingsTabs();

  if (status === "loading") {
    return <Loading fullPage={false} />;
  }

  if (!session?.user) {
    redirect("/");
  }

  if (!settingsTabs) {
    return (
      <div className="flex items-center justify-center py-12 text-sm text-gray-500">
        Keine Einstellungen verfügbar.
      </div>
    );
  }

  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <SettingsLayout
        tabs={settingsTabs.tabs}
        renderTab={settingsTabs.renderTab}
      />
    </Suspense>
  );
}

export default function SettingsPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <SettingsContent />
    </Suspense>
  );
}
