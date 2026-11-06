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
import { activeSchoolSettingsSection } from "~/components/school/shell/school-nav-items";

function SchoolSettingsContent() {
  const section = activeSchoolSettingsSection(useSearchParams().get("tab"));
  return (
    <div className="-mt-1.5 w-full space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">
          {section.label}
        </h1>
        {section.key === "notifications" ? (
          <p className="mt-1 text-sm text-gray-500">
            Wählen Sie zuerst die Themen. Dann erlauben Sie die Hinweise auf
            diesem Gerät.
          </p>
        ) : null}
      </div>
      {section.key === "notifications" ? (
        <>
          <NotificationPreferencesSection portal="school" />
          <PushNotificationSection portal="school" />
        </>
      ) : (
        <SchoolAnalyticsConsentSection />
      )}
    </div>
  );
}

export default function SchoolSettingsPage() {
  return (
    <Suspense fallback={null}>
      <SchoolSettingsContent />
    </Suspense>
  );
}
