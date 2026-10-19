"use client";

import { ArrowLeft } from "lucide-react";
import NavigationLink from "~/components/ui/navigation-link";
import { cn } from "~/lib/utils";

/**
 * Der Rückweg einer Unterseite: Pfeil und Ziel als Text, auf jeder Breite.
 * Er sitzt über dem Seitentitel in der Kopfkarte (`TenantPage` mit `back`,
 * `ParentPageHeader` mit `backHref`). Ein echter Link, damit Mittelklick,
 * „in neuem Tab öffnen" und die Wechselsperre ungespeicherter Formulare
 * greifen.
 *
 * `href` ist der fertige Pfad des Portals; im Tenant-Portal löst ihn der
 * Aufrufer über `useTenantAwarePath` auf.
 *
 * Die Beschriftung nennt das Ziel („Zurück zu den Räumen"), nicht nur
 * „Zurück": auf dem Desktop steht der Link neben den Brotkrumen und muss
 * ohne sie verständlich sein.
 */
export function BackLink({
  href,
  label,
  className,
}: Readonly<{ href: string; label: string; className?: string }>) {
  return (
    <NavigationLink
      href={href}
      className={cn(
        // `-ml-2` setzt den Pfeil auf die Fluchtlinie des Titels; die
        // Fläche bleibt für Hover und Fokus trotzdem symmetrisch. Auf dem
        // Telefon 44 px hoch, die Untergrenze einer Touch-Fläche.
        "-ml-2 flex h-8 w-fit items-center gap-1.5 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none max-sm:h-11",
        className,
      )}
    >
      <ArrowLeft className="h-4 w-4 shrink-0" aria-hidden="true" />
      {label}
    </NavigationLink>
  );
}
