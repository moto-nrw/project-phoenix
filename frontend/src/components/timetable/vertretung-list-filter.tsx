"use client";

import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";

/**
 * Anzeigemodus der Störungslisten. Der Typ wohnt HIER und nicht in
 * vertretung-day-list.tsx: die Liste rendert den Filter, ein Typ-Import in die
 * Gegenrichtung wäre ein Import-Zyklus (depcruise no-circular). Die Liste
 * exportiert ihn unter ihrem angestammten Namen weiter, damit die Aufrufer
 * unverändert bleiben.
 */
export type VertretungDayListMode = "stoerungen" | "ganzer-tag";

/**
 * Der Filter "Nur Störungen | Ganzer Tag" als Kopfzeile DER Liste, die er
 * filtert (#2031). Vorher stand er in der Kopfleiste der Seite, optisch
 * identisch mit dem Ansichts-Umschalter "Tag | Woche" direkt darüber: zwei
 * gleich aussehende Pillenleisten mit völlig verschiedener Wirkung. Jetzt sitzt
 * er auf der Fläche, die er verändert, und trägt die Unterstrich-Variante,
 * damit man ihn nicht mehr mit dem Ansichtswechsel verwechselt.
 */
interface VertretungListFilterProps {
  readonly mode: VertretungDayListMode;
  readonly onModeChange: (mode: VertretungDayListMode) => void;
  /** Beschriftung der Alles-Option: "Ganzer Tag" bzw. "Ganze Woche". */
  readonly allLabel: string;
}

export function VertretungListFilter({
  mode,
  onModeChange,
  allLabel,
}: VertretungListFilterProps) {
  return (
    // Kein eigener Rahmen und kein vertikales Padding am Wrapper: die
    // Unterstrich-Variante bringt ihre Trennlinie selbst mit. Zwei Linien
    // wenige Pixel übereinander sahen gequetscht aus. Das Padding sitzt an der
    // TabsList, damit die Linie über die volle Kartenbreite läuft, die
    // Beschriftungen aber auf der Textkante der Zeilen darunter stehen.
    <div className="shrink-0">
      <Tabs
        value={mode}
        onValueChange={(v) => onModeChange(v as VertretungDayListMode)}
      >
        <TabsList variant="line" className="px-3 pt-3">
          <TabsTrigger value="stoerungen">Nur Störungen</TabsTrigger>
          <TabsTrigger value="ganzer-tag">{allLabel}</TabsTrigger>
        </TabsList>
      </Tabs>
    </div>
  );
}
