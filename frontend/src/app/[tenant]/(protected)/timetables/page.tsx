"use client";

// /timetables ist im Betreuungsplan aufgegangen. Deep-Links und Bookmarks
// bleiben gültig: die Alt-Params (week, view, instance, …) werden in das
// neue Schema (d, view, block) übersetzt.

import { Suspense } from "react";

import { PlanungRedirect } from "~/components/timetable/planung-redirect";

export default function TimetablesRedirectPage() {
  return (
    <Suspense fallback={null}>
      <PlanungRedirect target="betreuungsplan" />
    </Suspense>
  );
}
