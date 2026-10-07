import { useSyncExternalStore } from "react";

// Tageszeit-Gruß, geteilt von Dashboard und Klassenansicht. getHours() liest
// die lokale Uhr der Person — für einen Gruß die richtige Referenz. Bewusst
// KEIN toLocaleString-Parsing: de-DE liefert "14 Uhr", Number() davon ist NaN
// und der Gruß bliebe für immer "Guten Abend".
export function getTimeBasedGreeting(now: Date = new Date()): string {
  const hour = now.getHours();
  if (hour < 12) return "Guten Morgen";
  if (hour < 17) return "Guten Tag";
  return "Guten Abend";
}

/** Gruß, solange nur der Server rendert: passt zu jeder Tageszeit. */
const SERVER_GREETING = "Hallo";

const subscribeToNothing = () => () => undefined;

/**
 * Der Tageszeit-Gruß für Seiten, die der Server vorrendert. Der Server kennt
 * die Uhr der Person nicht; ein dort berechneter Gruß widerspricht dem
 * Browser, sobald beide Uhren in verschiedenen Tageszeiten stehen, und React
 * verwirft die Seite mit einem Hydration-Fehler (#3764). Deshalb rendert der
 * Server "Hallo", und der Browser setzt den Tageszeit-Gruß direkt nach der
 * Hydration ein. Ohne Hydration (Wechsel innerhalb der App) steht der
 * Tageszeit-Gruß sofort da.
 */
export function useTimeBasedGreeting(): string {
  return useSyncExternalStore(
    subscribeToNothing,
    () => getTimeBasedGreeting(),
    () => SERVER_GREETING,
  );
}
