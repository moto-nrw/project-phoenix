"use client";

import { createContext, useContext, type ReactNode } from "react";

/**
 * Sprache der Fläche für die Standardtexte der Kit-Bausteine (#6744). Die
 * Bausteine bringen deutsche Erklärungen mit; auf einer übersetzten Fläche
 * (Elternportal, öffentliche Anmeldung) lassen sie sie weg, statt Deutsch in
 * einen türkischen Bildschirm zu mischen. Dort gibt der Aufrufer den
 * übersetzten Satz über die Prop des Bausteins.
 */
const KitLocaleContext = createContext("de");

export function KitLocaleProvider({
  locale,
  children,
}: Readonly<{ locale: string; children: ReactNode }>) {
  return <KitLocaleContext value={locale}>{children}</KitLocaleContext>;
}

/**
 * Gibt den deutschen Standardsatz eines Bausteins zurück, auf einer
 * übersetzten Fläche `undefined`.
 */
export function useKitDefault(): (germanText: string) => string | undefined {
  const german = useContext(KitLocaleContext).startsWith("de");
  return (germanText) => (german ? germanText : undefined);
}
