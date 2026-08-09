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
