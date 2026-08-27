"use client";

import { NextIntlClientProvider } from "next-intl";
import deMessages from "~/i18n/messages/de.json";
import { DEFAULT_LOCALE } from "~/i18n/locales";

// Sidebar + MobileBottomNav (rendered by AppShell) call
// useTranslations("parentNav") to label the parent-portal *preview* nav, which
// throws without a NextIntlClientProvider. The parents portal carries the full
// provider from its layout. The German-only staff + operator shells get this
// minimal German parentNav-only context instead — enough to satisfy the hook
// without shipping the rest of the parent message catalog into those portals'
// bundles. Keep this only around shells that are NOT parent-localized.
// `PwaInstallHint` haengt im selben Shell-Baum und liest
// `pwaInstallHint` — ohne diesen Eintrag wirft die Tenant-Seite zur Laufzeit
// (#2614), sichtbar als leere Seite hinter dem Dev-Overlay.
const NAV_MESSAGES = {
  parentNav: deMessages.parentNav,
  pwaInstallHint: deMessages.pwaInstallHint,
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
