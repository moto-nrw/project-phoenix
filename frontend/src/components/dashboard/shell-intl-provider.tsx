"use client";

import { NextIntlClientProvider } from "next-intl";
import deMessages from "~/i18n/messages/de.json";
import { DEFAULT_LOCALE } from "~/i18n/locales";

// Shared shell components call next-intl hooks, which throw without a
// NextIntlClientProvider. The parents portal carries the full provider from its
// layout. The German-only staff + operator shells get only the namespaces used
// by their shell instead of shipping the complete parent message catalog.
//
// Besides the shell itself it carries the namespaces of the shared settings
// cards, which the staff profile page and the school settings page (#2208)
// mount: without them every label there renders as its message key.
const SHELL_MESSAGES = {
  parentNav: deMessages.parentNav,
  pwaInstallHint: deMessages.pwaInstallHint,
  notificationPreferences: deMessages.notificationPreferences,
  pushNotifications: deMessages.pushNotifications,
  parentNotificationSetup: deMessages.parentNotificationSetup,
};

export function ShellIntlProvider({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <NextIntlClientProvider
      locale={DEFAULT_LOCALE}
      messages={SHELL_MESSAGES}
      timeZone="Europe/Berlin"
    >
      {children}
    </NextIntlClientProvider>
  );
}
