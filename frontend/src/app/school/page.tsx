"use client";

// Startseite des Schul-Portals ("moto schule", #2207): die Tagesübersicht
// der Klassenansicht. Die Kinderlisten liegen unter /school/klasse?klasse=…
// (#2294), eine Klasse pro Seite.

import { Suspense } from "react";
import { ClassDayOverview } from "~/components/class-day/class-day-overview";
import { TodayNoticesCard } from "~/components/school/today-notices-card";
import { Skeleton } from "~/components/ui/skeleton";
import {
  fetchClassDaySchool,
  fetchMyClassesSchool,
} from "~/lib/school-class-day-api";

export default function SchoolHomePage() {
  return (
    <div className="space-y-6">
      {/* Hinweise der OGS-Leitung für heute (#2208): oben, damit sie einem
          begegnen; ohne Hinweis rendert die Karte nichts. */}
      <TodayNoticesCard />
      {/* useSearchParams (der angezeigte Tag steht in der Adresse) braucht
          eine Suspense-Grenze. */}
      <Suspense fallback={<Skeleton className="h-64 w-full" />}>
        <ClassDayOverview
          fetchMyClasses={fetchMyClassesSchool}
          fetchClassDay={fetchClassDaySchool}
        />
      </Suspense>
    </div>
  );
}
