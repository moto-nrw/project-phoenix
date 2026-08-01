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

import { NotificationPreferencesSection } from "~/components/settings/notification-preferences-section";
import { PushNotificationSection } from "~/components/settings/push-notification-section";

export default function ParentSettingsPage() {
  const t = useTranslations("parentSettings");

  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <header className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {t("eyebrow")}
        </p>
        <h1 className="mt-1 text-2xl font-semibold text-balance text-gray-900">
          {t("title")}
        </h1>
        <p className="mt-1 text-sm leading-6 text-gray-600">
          {t("description")}
        </p>
      </header>

      <NotificationPreferencesSection portal="parent" />
      <PushNotificationSection portal="parent" />
    </div>
  );
}
