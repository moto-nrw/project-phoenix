"use client";

// /timetables ist in /planung aufgegangen (#1886). Deep-Links und Bookmarks
// bleiben gültig: alle Params (week, view, instance, …) werden durchgereicht.

import { Suspense } from "react";

import { PlanungRedirect } from "~/components/timetable/planung-redirect";

export default function TimetablesRedirectPage() {
  return (
    <Suspense fallback={null}>
      <PlanungRedirect tab="betreuung" />
    </Suspense>
  );
}
