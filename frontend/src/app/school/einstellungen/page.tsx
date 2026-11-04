"use client";

// Einstellungen im Schul-Portal ("moto schule", #2208): welche Hinweise eine
// Lehrkraft bekommen möchte und auf welchem Gerät (Team-Chat, seit #4792
// auch Krankmeldungen der eigenen Klassen). Dieselben Karten wie im
// OGS- und im Elternportal, gebunden an die school-Session.

import { NotificationPreferencesSection } from "~/components/settings/notification-preferences-section";
import { PushNotificationSection } from "~/components/settings/push-notification-section";
import { SchoolAnalyticsConsentSection } from "~/components/settings/analytics-consent-section";

export default function SchoolSettingsPage() {
  return (
    <div className="-mt-1.5 w-full space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">
          Einstellungen
        </h1>
        <p className="mt-1 text-sm text-gray-500">
          Wählen Sie zuerst die Themen. Dann erlauben Sie die Hinweise auf
          diesem Gerät.
        </p>
      </div>
      <NotificationPreferencesSection portal="school" />
      <PushNotificationSection portal="school" />
      <SchoolAnalyticsConsentSection />
    </div>
  );
}
