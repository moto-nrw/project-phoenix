"use client";

// /staff/dienstplan ist im Dienstplan-Bereich aufgegangen. Der Dienstplan
// hatte nie URL-Zustand, es gibt nichts zu übersetzen.

import { Suspense } from "react";

import { PlanungRedirect } from "~/components/timetable/planung-redirect";

export default function DienstplanRedirectPage() {
  return (
    <Suspense fallback={null}>
      <PlanungRedirect target="dienstplan" />
    </Suspense>
  );
}
