"use client";

// Eigenständiger Dienstplan-Bereich (Planung-Redesign, docs/planung-
// redesign/docs/03). Rendert übergangsweise die unveränderte Bestands-View;
// der inhaltliche Umbau (ResourceGrid, d/view-Schema) folgt mit docs/05.

import { Suspense } from "react";

import { DienstplanView } from "~/components/staff/dienstplan-view";

export default function DienstplanPage() {
  return (
    <Suspense fallback={null}>
      <DienstplanView />
    </Suspense>
  );
}
