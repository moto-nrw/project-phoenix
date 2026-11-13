"use client";

import type { ReactNode } from "react";
import { useLocale } from "next-intl";
import { KitLocaleProvider } from "./kit-locale";

/**
 * Übernimmt die Sprache von `NextIntlClientProvider` für das Kit. Eigenes
 * Modul, damit `next-intl` nicht über `useKitDefault` in jeden Kit-Baustein
 * und damit in das Bundle jeder Seite wandert.
 */
export function KitLocaleFromIntl({
  children,
}: Readonly<{ children: ReactNode }>) {
  return <KitLocaleProvider locale={useLocale()}>{children}</KitLocaleProvider>;
}
