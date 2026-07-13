"use client";

// Eigenständiger Betreuungsplan-Bereich (Planung-Redesign, docs/planung-
// redesign/docs/03). Rendert übergangsweise die unveränderte Bestands-View;
// der inhaltliche Umbau (Kopfzeile, Chrome-Abbau, d/view/block-Schema)
// folgt mit docs/06.

import { Suspense } from "react";

import { BetreuungsplanView } from "~/components/timetable/betreuungsplan-view";

export default function BetreuungsplanPage() {
  return (
    <Suspense fallback={null}>
      <BetreuungsplanView />
    </Suspense>
  );
}
