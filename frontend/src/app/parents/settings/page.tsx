"use client";

/**
 * Parents-portal settings page (#1671).
 *
 * Personal settings belong to the account, not to a content area, so this page
 * is reached from the avatar menu — the same place the staff and operator
 * portals put theirs. Before it existed, the two notification cards sat at the
 * bottom of the dashboard with no entry point of their own: a parent who never
 * scrolled that far never found them, and since "no row means off" that parent
 * silently received nothing at all.
 */

import { useTranslations } from "next-intl";
import { Settings } from "lucide-react";

import { NotificationPreferencesSection } from "~/components/settings/notification-preferences-section";
import { PushNotificationSection } from "~/components/settings/push-notification-section";
import { InfoCard } from "~/components/ui/info-card";

export default function ParentSettingsPage() {
  const t = useTranslations("parentSettings");

  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <InfoCard
        title={t("title")}
        eyebrow={t("eyebrow")}
        headingLevel={1}
        icon={<Settings className="h-5 w-5" aria-hidden="true" />}
      >
        <p className="text-sm leading-6 text-gray-600">{t("description")}</p>
      </InfoCard>

      <NotificationPreferencesSection portal="parent" />
      <PushNotificationSection portal="parent" />
    </div>
  );
}
