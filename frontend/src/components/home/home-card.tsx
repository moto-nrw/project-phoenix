import type { ComponentProps } from "react";
import type { MotoConcept } from "~/lib/moto-concepts";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { SectionCard } from "~/components/ui/section-card";

/**
 * Gemeinsame Bauteile jeder Karte der Startseite. Sie liegen bewusst neben
 * dem Verteiler (`home-block-content.tsx`), nicht darin: der Verteiler
 * importiert jede Karte, und die Karten brauchen diese Bauteile. Läge beides
 * in einer Datei, entstünde ein Importkreis.
 */

/**
 * Die Karte der Startseite: volle Höhe im Raster, der Körper füllt den Rest.
 *
 * Der Körper scrollt nicht und blendet nichts aus: jede Karte zeigt so viele
 * Zeilen, wie ganz hineinpassen, und nennt den Rest als Zahl mit Weg
 * (`useHomeCardRows`). Ein Verlauf am unteren Rand sah aus, als liefe der
 * Inhalt unter der Karte weiter, auch wo nichts fehlte. `-mx-1 px-1` gibt
 * dem Schatten von Knöpfen und Zeilen den Platz, den ihm `overflow-hidden`
 * sonst am Rand abschneidet.
 */
export function HomeCard({
  children,
  ...props
}: Pick<
  ComponentProps<typeof SectionCard>,
  "title" | "leading" | "inlineActions" | "actions" | "children"
>) {
  return (
    <SectionCard
      {...props}
      className="flex h-full flex-col"
      bodyClassName="mt-4 flex min-h-0 flex-1 flex-col"
    >
      <div className="-mx-1 -mb-2 min-h-0 flex-1 overflow-hidden px-1 pb-2">
        {children}
      </div>
    </SectionCard>
  );
}

/**
 * Symbolfläche aller Karten der Startseite: ein Kasten, eine Größe. Vorher
 * trugen die einen ihr Symbol im grauen Kasten und die anderen nackt — auf
 * einer Fläche nebeneinander fällt genau das auf.
 */
export function HomeCardIcon({ concept }: { readonly concept: MotoConcept }) {
  return <ConceptIconTile concept={concept} size="section" />;
}
