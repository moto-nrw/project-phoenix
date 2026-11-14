"use client";

// Einstellungen im Schul-Portal ("moto schule", #2208): welche Hinweise eine
// Lehrkraft bekommen möchte und auf welchem Gerät (Team-Chat, seit #4792
// auch Krankmeldungen der eigenen Klassen). Dieselben Karten wie im
// OGS- und im Elternportal, gebunden an die school-Session. Die Bereiche stehen
// in der Seitenleiste; offen ist, was die Adresse nennt (#5750). Seit #6955
// zeigt ein eigener Bereich die Geräte, die sich die Lehrkraft beim Anmelden
// gemerkt hat, und entfernt sie wieder.

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
      {section.key === "notifications" && (
        <>
          <NotificationPreferencesSection
            portal="school"
            description="Wählen Sie zuerst die Themen. Dann erlauben Sie die Hinweise auf diesem Gerät."
          />
          <PushNotificationSection portal="school" />
        </>
      )}
      {section.key === "devices" && <TrustedDevicesSection scope="school" />}
      {section.key === "privacy" && <SchoolAnalyticsConsentSection />}
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
