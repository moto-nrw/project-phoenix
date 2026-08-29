"use client";

// Eine Klasse an einem Tag ("moto schule", #2294). Klasse und Tag stehen als
// Query-Parameter in der Adresse (`?klasse=…&tag=…`) — begründet an
// `CLASS_DAY_CLASS_PARAM`. Ob die Klasse der Lehrkraft zugewiesen ist,
// entscheidet weiterhin das Backend (403), nicht diese Seite.

import { useSearchParams } from "next/navigation";
import { Suspense } from "react";
import { ClassDayClass } from "~/components/class-day/class-day-class";
import {
  CLASS_DAY_CLASS_PARAM,
  classDayClassParam,
} from "~/components/class-day/routes";
import { Skeleton } from "~/components/ui/skeleton";
import { fetchClassDaySchool } from "~/lib/school-class-day-api";

function ClassFromAddress() {
  const schoolClass = classDayClassParam(
    useSearchParams().get(CLASS_DAY_CLASS_PARAM),
  );
  return (
    <ClassDayClass
      schoolClass={schoolClass}
      fetchClassDay={fetchClassDaySchool}
    />
  );
}

export default function SchoolClassDayPage() {
  return (
    // useSearchParams (Klasse und Tag stehen in der Adresse) braucht eine
    // Suspense-Grenze.
    <Suspense fallback={<Skeleton className="h-64 w-full" />}>
      <ClassFromAddress />
    </Suspense>
  );
}
