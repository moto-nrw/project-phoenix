"use client";

// /vertretungsplan ist im Vertretungs-Bereich aufgegangen. Deep-Links und
// Bookmarks bleiben gültig: die Alt-Params (week, instance, history, …)
// werden in das neue Schema (d, block, verlauf) übersetzt.

import { Suspense } from "react";

import { PlanungRedirect } from "~/components/timetable/planung-redirect";

export default function VertretungsplanRedirectPage() {
  return (
    <Suspense fallback={null}>
      <PlanungRedirect target="vertretung" />
    </Suspense>
  );
}
