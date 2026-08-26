"use client";

/**
 * Parents-portal settings page (#1671).
 *
 * Personal settings belong to the account, not to a content area, so this page
 * is reached from the account navigation, alongside sign-out. Before it
 * existed, the two notification cards sat at the
 * bottom of the dashboard with no entry point of their own: a parent who never
 * scrolled that far never found them, and since "no row means off" that parent
 * silently received nothing at all.
 */

import { useTranslations } from "next-intl";

import { NotificationPreferencesSection } from "~/components/settings/notification-preferences-section";
import { PushNotificationSection } from "~/components/settings/push-notification-section";
import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";
import { LanguageSwitcher } from "~/components/parent/language-switcher";
import { ParentSection } from "~/components/parent/shell/parent-section";
import { SamsungChromeInstallSection } from "~/components/parent/samsung-chrome-install-section";

export default function ParentSettingsPage() {
  const t = useTranslations("parentSettings");

  return (
    <ParentPage>
      <ParentPageHeader
        kicker={t("eyebrow")}
        title={t("title")}
        description={t("description")}
      />

      <ParentSection
        title={t("languageTitle")}
        description={t("languageDescription")}
        actions={
          <div className="ms-auto shrink-0">
            <LanguageSwitcher />
          </div>
        }
      />

      <SamsungChromeInstallSection />
      <NotificationPreferencesSection portal="parent" />
      <PushNotificationSection portal="parent" />
    </ParentPage>
  );
}
