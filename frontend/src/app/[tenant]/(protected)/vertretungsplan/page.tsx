"use client";

// /vertretungsplan ist in /planung aufgegangen (#1886). Deep-Links und
// Bookmarks bleiben gültig: alle Params (week, instance, history, …) werden
// durchgereicht.

import { Suspense } from "react";

import { PlanungRedirect } from "~/components/timetable/planung-redirect";

export default function VertretungsplanRedirectPage() {
  return (
    <Suspense fallback={null}>
      <PlanungRedirect tab="vertretung" />
    </Suspense>
  );
}
