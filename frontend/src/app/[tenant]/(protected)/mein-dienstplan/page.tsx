"use client";

// Eigener Dienstplan der Mitarbeitenden (#3821), lesend. Eigener Eintrag in
// der Seitenleiste (Team); die Zeiterfassung verlinkt zusätzlich hierher.

import { Suspense } from "react";

import { DienstplanPageSkeleton } from "~/components/staff/dienstplan-skeleton";
import { OwnDienstplanView } from "~/components/time-tracking/own-dienstplan-view";

export default function OwnDienstplanPage() {
  return (
    <Suspense fallback={<DienstplanPageSkeleton />}>
      <OwnDienstplanView />
    </Suspense>
  );
}
