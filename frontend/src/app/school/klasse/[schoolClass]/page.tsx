"use client";

// Eine Klasse an einem Tag ("moto schule", #2294). Der Klassenname steht als
// kodiertes Adress-Segment darin — Klassennamen sind Freitext und enthalten
// Leerzeichen ("Klasse 2a"). Ob die Klasse der Lehrkraft zugewiesen ist,
// entscheidet weiterhin das Backend (403), nicht diese Seite.

import { use, Suspense } from "react";
import { ClassDayClass } from "~/components/class-day/class-day-class";
import { classDayNameFromParam } from "~/components/class-day/routes";
import { Skeleton } from "~/components/ui/skeleton";
import { fetchClassDaySchool } from "~/lib/school-class-day-api";

export default function SchoolClassDayPage({
  params,
}: {
  readonly params: Promise<{ schoolClass: string }>;
}) {
  // Dekodiert, nicht roh: `params` liefert das Adress-Segment hier kodiert.
  // Begründung und Messung stehen an `classDayNameFromParam`, dem Gegenstück
  // zur Kodierung in `classDayPath`.
  const { schoolClass } = use(params);
  return (
    <Suspense fallback={<Skeleton className="h-64 w-full" />}>
      <ClassDayClass
        schoolClass={classDayNameFromParam(schoolClass)}
        fetchClassDay={fetchClassDaySchool}
      />
    </Suspense>
  );
}
