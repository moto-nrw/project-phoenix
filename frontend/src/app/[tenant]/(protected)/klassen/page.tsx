"use client";

// Lehrkraft-Klassenansicht im Tenant-Portal (#1772). Seit #2207 ist die
// Ansicht selbst eine geteilte Komponente (components/class-day) — dieser
// Mount bleibt als Übergangsbrücke bestehen, bis der Cutover (PR 3) den
// Lehrkraft-Zugang vollständig ins Schul-Portal verlegt.

import { ClassDayView } from "~/components/class-day/class-day-view";
import { fetchClassDay, fetchMyClasses } from "~/lib/class-day-api";

export default function KlassenPage() {
  return (
    <ClassDayView
      fetchMyClasses={fetchMyClasses}
      fetchClassDay={fetchClassDay}
    />
  );
}
