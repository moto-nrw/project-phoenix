"use client";

import { NextIntlClientProvider } from "next-intl";
import deMessages from "~/i18n/messages/de.json";
import { DEFAULT_LOCALE } from "~/i18n/locales";

// Sidebar + MobileBottomNav (rendered by AppShell) call
// useTranslations("parentNav") to label the parent-portal *preview* nav, which
// throws without a NextIntlClientProvider. The parents portal carries the full
// provider from its layout. The German-only staff + school + operator shells
// get this minimal German context instead — enough to satisfy the hooks
// without shipping the rest of the parent message catalog into those portals'
// bundles. Keep this only around shells that are NOT parent-localized.
//
// Besides the nav labels it carries the namespaces of the shared settings
// cards, which the staff profile page and the school settings page (#2208)
// mount: without them every label there renders as its message key.
const NAV_MESSAGES = {
  parentNav: deMessages.parentNav,
  notificationPreferences: deMessages.notificationPreferences,
  pushNotifications: deMessages.pushNotifications,
  parentNotificationSetup: deMessages.parentNotificationSetup,
};

export function ShellNavIntlProvider({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <NextIntlClientProvider locale={DEFAULT_LOCALE} messages={NAV_MESSAGES}>
      {children}
    </NextIntlClientProvider>
  );
}
