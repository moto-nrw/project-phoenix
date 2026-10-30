"use client";

import { NextIntlClientProvider } from "next-intl";
import datePicker from "~/i18n/messages/de/datePicker.json";
import mealAllergens from "~/i18n/messages/de/mealAllergens.json";
import mealOrders from "~/i18n/messages/de/mealOrders.json";
import mealParticipantList from "~/i18n/messages/de/mealParticipantList.json";
import notificationPreferences from "~/i18n/messages/de/notificationPreferences.json";
import parentCalendarSubscribe from "~/i18n/messages/de/parentCalendarSubscribe.json";
import parentChild from "~/i18n/messages/de/parentChild.json";
import parentNav from "~/i18n/messages/de/parentNav.json";
import parentNotificationSetup from "~/i18n/messages/de/parentNotificationSetup.json";
import pwaInstallHint from "~/i18n/messages/de/pwaInstallHint.json";
import pushNotifications from "~/i18n/messages/de/pushNotifications.json";
import staffCalendarSubscribe from "~/i18n/messages/de/staffCalendarSubscribe.json";
import surveyForm from "~/i18n/messages/de/surveyForm.json";
import { DEFAULT_LOCALE } from "~/i18n/locales";

// Shared shell components call next-intl hooks, which throw without a
// NextIntlClientProvider. The parents portal carries the full provider from its
// layout. The German-only staff + operator shells get only the namespaces used
// by their shell instead of shipping the complete parent message catalog.
//
// Besides the shell itself it carries the namespaces of staff-facing shared
// components: settings cards, the meal participant list and order week and
// their date picker, the allergen marks of the meal plan and the survey
// questionnaire,
// plus the consent summary shown in student master data. Without these subsets
// every label there renders as its message key.
const SHELL_MESSAGES = {
  datePicker,
  parentNav,
  pwaInstallHint,
  notificationPreferences,
  pushNotifications,
  parentNotificationSetup,
  parentCalendarSubscribe,
  staffCalendarSubscribe,
  mealParticipantList,
  mealOrders,
  mealAllergens,
  // Der Fragebogen einer Umfrage (#4781): Mitarbeitende beantworten dieselbe
  // Komponente wie Eltern.
  surveyForm,
  parentChild: {
    consents: parentChild.consents,
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
