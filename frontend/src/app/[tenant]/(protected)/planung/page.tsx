"use client";

/**
 * /planung — die vereinheitlichte Planungs-Oberfläche (#1886).
 *
 * Betreuungsplan, Dienstplan und Vertretung sind Tabs EINER Seite mit einer
 * gemeinsamen Grundlage (Kalenderzeiträume, Deviations-Pfad,
 * Änderungsprotokoll). Die Tab-Inhalte sind die unverändert extrahierten
 * bisherigen Seiten (BetreuungsplanView / DienstplanView /
 * VertretungsplanView); die alten Routen leiten hierher weiter.
 *
 * Der aktive Tab lebt im URL-Param `?tab=` — alle übrigen Params (week, view,
 * instance, history, …) gehören den Teilansichten und bleiben unangetastet.
 */

import { Suspense, useCallback } from "react";
import { useSearchParams } from "next/navigation";

import { NavigationTabs } from "~/components/ui/page-header/NavigationTabs";
import { DienstplanView } from "~/components/staff/dienstplan-view";
import { BetreuungsplanView } from "~/components/timetable/betreuungsplan-view";
import { VertretungsplanView } from "~/components/timetable/vertretungsplan-view";

type PlanungTab = "betreuung" | "dienstplan" | "vertretung";

const TAB_ITEMS: Array<{ id: PlanungTab; label: string }> = [
  { id: "betreuung", label: "Betreuungsplan" },
  { id: "dienstplan", label: "Dienstplan" },
  { id: "vertretung", label: "Vertretung" },
];

function parseTab(raw: string | null): PlanungTab {
  return raw === "dienstplan" || raw === "vertretung" ? raw : "betreuung";
}

function PlanungContent() {
  const searchParams = useSearchParams();
  const tab = parseTab(searchParams.get("tab"));

  const handleTabChange = useCallback((next: string) => {
    // Beim Tab-Wechsel die view-spezifischen Params verwerfen: `instance`,
    // `history` usw. gehören der jeweiligen Teilansicht; ein Wechsel mit
    // fremden Params würde z. B. im Vertretungs-Tab einen Slide-Over für
    // eine Instanz öffnen, die der Tab gar nicht anzeigt.
    // Next.js synct natives history.replaceState mit useSearchParams —
    // dasselbe Muster nutzen die Teilansichten für ihre eigenen Params.
    const params = new URLSearchParams();
    params.set("tab", next);
    window.history.replaceState(null, "", `?${params.toString()}`);
  }, []);

  return (
    <div className="space-y-4">
      <NavigationTabs
        items={TAB_ITEMS}
        activeTab={tab}
        onTabChange={handleTabChange}
      />
      {tab === "betreuung" ? <BetreuungsplanView /> : null}
      {tab === "dienstplan" ? <DienstplanView /> : null}
      {tab === "vertretung" ? <VertretungsplanView /> : null}
    </div>
  );
}

export default function PlanungPage() {
  return (
    <Suspense fallback={null}>
      <PlanungContent />
    </Suspense>
  );
}
