"use client";

// Eigenständiger Vertretungs-Bereich (Planung-Redesign, docs/planung-
// redesign/docs/07). Der heute-zentrierte Zweiteiler mit dem
// d/block/verlauf-URL-Schema.

import { Suspense } from "react";

import { VertretungView } from "~/components/timetable/vertretung-view";

export default function VertretungPage() {
  return (
    <Suspense fallback={null}>
      <VertretungView />
    </Suspense>
  );
}
