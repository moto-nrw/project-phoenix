"use client";

import { Suspense } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import {
  SkeletonRegion,
  PageHeaderSkeleton,
  FormSkeleton,
} from "~/components/ui/page-skeletons";
import { SettingsLayout } from "~/components/shared/settings-layout";
import { useSettingsTabs } from "~/components/settings/settings-page";

const settingsLoadingFallback = (
  <SkeletonRegion label="Einstellungen werden geladen…">
    <PageHeaderSkeleton search={false} />
    <FormSkeleton />
  </SkeletonRegion>
);

function SettingsContent() {
  const { data: session, status } = useSession({
    required: true,
  });

  const settingsTabs = useSettingsTabs();

  if (status === "loading") {
    return settingsLoadingFallback;
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
    <Suspense fallback={settingsLoadingFallback}>
      <SettingsLayout
        tabs={settingsTabs.tabs}
        renderTab={settingsTabs.renderTab}
      />
    </Suspense>
  );
}

export default function SettingsPage() {
  return (
    <Suspense fallback={settingsLoadingFallback}>
      <SettingsContent />
    </Suspense>
  );
}
