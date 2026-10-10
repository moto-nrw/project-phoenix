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

import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";

import { NotificationPreferencesSection } from "~/components/settings/notification-preferences-section";
import { PushNotificationSection } from "~/components/settings/push-notification-section";
import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";
import { LanguageSwitcher } from "~/components/parent/language-switcher";
import { ParentSection } from "~/components/parent/shell/parent-section";
import { SamsungChromeInstructions } from "~/components/tenant/pwa-install-hint";
import { ButtonLink } from "~/components/ui/button";
import { buildHelpHref, HELP_TOPICS } from "~/lib/help-topics";
import { isStandaloneApp } from "~/lib/push-api";
import { isSamsungInternet } from "~/lib/pwa-install-prompt";

export default function ParentSettingsPage() {
  const t = useTranslations("parentSettings");
  const pathname = usePathname();
  const [pageUrl, setPageUrl] = useState<URL | null>(null);
  const installHelpHref = buildHelpHref(
    {
      role: "parent",
      nfcEnabled: false,
      presenceMode: "detailed",
      groupMode: "fixed_groups",
      returnTo: pathname,
    },
    HELP_TOPICS.parentInstallApp,
  );

  useEffect(() => {
    if (isSamsungInternet(window.navigator) && !isStandaloneApp()) {
      setPageUrl(new URL(window.location.href));
    }
  }, []);

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

      <div data-parent-tour-install-app>
        <ParentSection
          title={t("appInstallTitle")}
          description={t("appInstallGenericDescription")}
          concept="devices"
        >
          {pageUrl ? (
            <SamsungChromeInstructions pageUrl={pageUrl} />
          ) : (
            <ButtonLink href={installHelpHref} variant="surface" size="md">
              {t("appInstallGuide")}
            </ButtonLink>
          )}
        </ParentSection>
      </div>
      <div data-parent-tour="notification-topics">
        <NotificationPreferencesSection portal="parent" />
      </div>
      <PushNotificationSection portal="parent" />
    </ParentPage>
  );
}
