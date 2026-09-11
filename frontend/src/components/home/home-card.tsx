import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { MotoConceptKey } from "~/lib/moto-concepts";

/**
 * Gemeinsame Bauteile jeder Karte der Startseite. Sie liegen bewusst neben
 * dem Verteiler (`home-block-content.tsx`), nicht darin: der Verteiler
 * importiert jede Karte, und die Karten brauchen diese Bauteile. Läge beides
 * in einer Datei, entstünde ein Importkreis.
 */

/**
 * Der Körper jeder Karte der Startseite.
 *
 * Er scrollt nicht und blendet nichts aus: jede Karte zeigt so viele Zeilen,
 * wie ganz hineinpassen, und nennt den Rest als Zahl mit Weg
 * (`useHomeCardRows`). Ein Verlauf am unteren Rand sah aus, als liefe der
 * Inhalt unter der Karte weiter, auch wo nichts fehlte. `-mx-1 px-1` gibt
 * dem Schatten von Knöpfen und Zeilen den Platz, den ihm `overflow-hidden`
 * sonst am Rand abschneidet.
 */
export const HOME_CARD_BODY =
  "mt-4 -mx-1 -mb-2 min-h-0 flex-1 overflow-hidden px-1 pb-2";

/**
 * Symbolfläche aller Karten der Startseite: ein Kasten, eine Größe. Vorher
 * trugen die einen ihr Symbol im grauen Kasten und die anderen nackt — auf
 * einer Fläche nebeneinander fällt genau das auf.
 */
export function HomeCardIcon({
  concept,
}: {
  readonly concept: MotoConceptKey;
}) {
  return (
    <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-50 shadow-sm">
      <MotoConceptIcon concept={concept} size={20} />
    </span>
  );
}
