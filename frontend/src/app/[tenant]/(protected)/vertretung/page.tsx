"use client";

// Eigenständiger Vertretungs-Bereich (Planung-Redesign, docs/planung-
// redesign/docs/03). Rendert übergangsweise die unveränderte Bestands-View;
// der Zweiteiler-Umbau (d/block/verlauf-Schema) folgt mit docs/07.

import { Suspense } from "react";

import { VertretungsplanView } from "~/components/timetable/vertretungsplan-view";

export default function VertretungPage() {
  return (
    <Suspense fallback={null}>
      <VertretungsplanView />
    </Suspense>
  );
}
