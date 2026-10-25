"use client";

// Eigener Dienstplan der Mitarbeitenden (#3821), lesend. Eigener Eintrag in
// der Seitenleiste (Team); die Zeiterfassung verlinkt zusätzlich hierher.

import { Suspense } from "react";

import { DienstplanPageSkeleton } from "~/components/staff/dienstplan-skeleton";
import { ModuleRouteGuard } from "~/components/tenant/module-route-guard";
import { OwnDienstplanView } from "~/components/time-tracking/own-dienstplan-view";

export default function OwnDienstplanPage() {
  return (
    <ModuleRouteGuard module="dienstplan" title="Mein Dienstplan">
      <Suspense fallback={<DienstplanPageSkeleton />}>
        <OwnDienstplanView />
      </Suspense>
    </ModuleRouteGuard>
  );
}
