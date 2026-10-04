"use client";

// Eigener Dienstplan der Mitarbeitenden (#3821), lesend. Unterseite der
// Zeiterfassung, weil Schichten und Soll dort schon zu Hause sind.

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
