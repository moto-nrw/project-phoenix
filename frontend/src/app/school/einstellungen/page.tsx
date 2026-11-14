"use client";

// Einstellungen im Schul-Portal ("moto schule", #2208): welche Hinweise eine
// Lehrkraft bekommen möchte und auf welchem Gerät (Team-Chat, seit #4792
// auch Krankmeldungen der eigenen Klassen), und welche Geräte moto schule
// nach dem Code gemerkt hat (#6955). Dieselben Karten wie im
// OGS- und im Elternportal, gebunden an die school-Session. Die Bereiche stehen
// in der Seitenleiste; offen ist, was die Adresse nennt (#5750).

import { useSearchParams } from "next/navigation";
import { Suspense } from "react";
import { NotificationPreferencesSection } from "~/components/settings/notification-preferences-section";
import { PushNotificationSection } from "~/components/settings/push-notification-section";
import { SchoolAnalyticsConsentSection } from "~/components/settings/analytics-consent-section";
import { TrustedDevicesSection } from "~/components/settings/trusted-devices-section";
import { TenantPage } from "~/components/ui/tenant-page";
import { activeSchoolSettingsSection } from "~/components/school/shell/school-nav-items";

function SchoolSettingsContent() {
  const section = activeSchoolSettingsSection(useSearchParams().get("tab"));
  return (
    <TenantPage title={section.label}>
      {section.key === "notifications" ? (
        <>
          <NotificationPreferencesSection
            portal="school"
            description="Wählen Sie zuerst die Themen. Dann erlauben Sie die Hinweise auf diesem Gerät."
          />
          <PushNotificationSection portal="school" />
        </>
      ) : section.key === "devices" ? (
        <TrustedDevicesSection scope="school" />
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
