"use client";

// /staff/dienstplan ist in /planung aufgegangen (#1886). Deep-Links und
// Bookmarks bleiben gültig: alle Params werden durchgereicht.

import { Suspense } from "react";

import { PlanungRedirect } from "~/components/timetable/planung-redirect";

export default function DienstplanRedirectPage() {
  return (
    <Suspense fallback={null}>
      <PlanungRedirect tab="dienstplan" />
    </Suspense>
  );
}
