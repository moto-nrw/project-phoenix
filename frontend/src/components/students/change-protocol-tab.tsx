"use client";

/**
 * Reiter „Änderungsprotokoll“ der Kinderkartei (#2437): dieselbe Historie wie
 * im Anfragen-Modul, auf dieses Kind gefiltert. Entschiedene Anfragen aller
 * Arten und Direkt-Korrekturen der Verwaltung, chronologisch, mit vorher →
 * nachher, wer, wann und Begründung — damit am Kind sichtbar ist, warum es
 * heute anders läuft als letzte Woche.
 *
 * Anmeldungsänderungen fehlen bewusst: sie hängen an einer eigenen
 * Berechtigung, kennen den Kind-Filter nicht und betreffen die Zeit vor der
 * Übernahme in die Kinderkartei.
 */

import { SectionCard } from "~/components/ui/section-card";
import { AggregatedRequestList } from "~/components/students/aggregated-request-list";

export function StudentChangeProtocolTab({
  studentId,
}: {
  readonly studentId: string;
}) {
  return (
    <SectionCard
      kicker="Kinderkartei"
      title="Änderungsprotokoll"
      description="Was sich an Buchungen, Betreuungszeiten, Stammdaten und Entschuldigungen dieses Kindes geändert hat"
    >
      <AggregatedRequestList
        view="history"
        filters={{
          search: "",
          studentId,
          includeEnrollment: false,
          types: [],
          statuses: [],
        }}
      />
    </SectionCard>
  );
}
