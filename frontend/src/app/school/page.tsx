"use client";

// Startseite des Schul-Portals ("moto schule", #2207): die Tagesübersicht
// der Klassenansicht. Die Kinderlisten liegen unter /school/klasse/[klasse]
// (#2294), eine Klasse pro Seite.

import { Suspense } from "react";
import { ClassDayOverview } from "~/components/class-day/class-day-overview";
import { Skeleton } from "~/components/ui/skeleton";
import {
  fetchClassDaySchool,
  fetchMyClassesSchool,
} from "~/lib/school-class-day-api";

export default function SchoolHomePage() {
  return (
    // useSearchParams (der angezeigte Tag steht in der Adresse) braucht eine
    // Suspense-Grenze.
    <Suspense fallback={<Skeleton className="h-64 w-full" />}>
      <ClassDayOverview
        fetchMyClasses={fetchMyClassesSchool}
        fetchClassDay={fetchClassDaySchool}
      />
    </Suspense>
  );
}
