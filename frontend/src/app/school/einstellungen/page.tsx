"use client";

// Einstellungen im Schul-Portal ("moto schule", #2208): welche Hinweise eine
// Lehrkraft bekommen möchte und auf welchem Gerät (Team-Chat, seit #4792
// auch Krankmeldungen der eigenen Klassen). Dieselben Karten wie im
// OGS- und im Elternportal, gebunden an die school-Session. Die Bereiche stehen
// in der Seitenleiste; offen ist, was die Adresse nennt (#5750).

import { useSearchParams } from "next/navigation";
import { Suspense } from "react";
import { NotificationPreferencesSection } from "~/components/settings/notification-preferences-section";
import { PushNotificationSection } from "~/components/settings/push-notification-section";
import { SchoolAnalyticsConsentSection } from "~/components/settings/analytics-consent-section";
import { TenantPage } from "~/components/ui/tenant-page";
import { activeSchoolSettingsSection } from "~/components/school/shell/school-nav-items";

function SchoolSettingsContent() {
  const section = activeSchoolSettingsSection(useSearchParams().get("tab"));
  return (
    <TenantPage
      title={section.label}
      info={
        section.key === "notifications" ? (
          <p>
            Wählen Sie zuerst die Themen. Dann erlauben Sie die Hinweise auf
            diesem Gerät.
          </p>
        ) : undefined
      }
    >
      {section.key === "notifications" ? (
        <>
          <NotificationPreferencesSection portal="school" />
          <PushNotificationSection portal="school" />
        </>
      ) : (
        <SchoolAnalyticsConsentSection />
      )}
    </TenantPage>
  );
}

export default function SchoolSettingsPage() {
  return (
    <Suspense fallback={null}>
      <SchoolSettingsContent />
    </Suspense>
  );
}
