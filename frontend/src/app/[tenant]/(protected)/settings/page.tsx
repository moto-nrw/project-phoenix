"use client";

import { Suspense } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import {
  SkeletonRegion,
  PageHeaderSkeleton,
  FormSkeleton,
} from "~/components/ui/page-skeletons";
import { EmptyState } from "~/components/ui/empty-state";
import { PageIntro } from "~/components/ui/page-intro";
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
      <div className="w-full">
        <PageIntro
          title="Einstellungen"
          description="0 Bereiche"
          className="mb-6"
        />
        <EmptyState
          title="Keine Einstellungen verfügbar."
          description="Für Ihre Rolle ist hier nichts freigegeben. Wenden Sie sich an Ihre Leitung, wenn Sie Einstellungen ändern müssen."
        />
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
