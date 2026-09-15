"use client";

import { NextIntlClientProvider } from "next-intl";
import deMessages from "~/i18n/messages/de.json";
import { DEFAULT_LOCALE } from "~/i18n/locales";

// Shared shell components call next-intl hooks, which throw without a
// NextIntlClientProvider. The parents portal carries the full provider from its
// layout. The German-only staff + operator shells get only the namespaces used
// by their shell instead of shipping the complete parent message catalog.
//
// Besides the shell itself it carries the namespaces of staff-facing shared
// components: settings cards, the meal participant list and its date picker,
// plus the consent summary shown in student master data. Without these subsets
// every label there renders as its message key.
const SHELL_MESSAGES = {
  datePicker: deMessages.datePicker,
  parentNav: deMessages.parentNav,
  pwaInstallHint: deMessages.pwaInstallHint,
  notificationPreferences: deMessages.notificationPreferences,
  pushNotifications: deMessages.pushNotifications,
  parentNotificationSetup: deMessages.parentNotificationSetup,
  parentCalendarSubscribe: deMessages.parentCalendarSubscribe,
  staffCalendarSubscribe: deMessages.staffCalendarSubscribe,
  mealParticipantList: deMessages.mealParticipantList,
  parentChild: {
    consents: deMessages.parentChild.consents,
  },
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
