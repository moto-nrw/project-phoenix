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
import {
  addHomeworkSchool,
  changeHomeworkSchool,
  deleteHomeworkSchool,
  fetchClassDayClassesSchool,
  fetchClassDaySchool,
  handOverClassSchool,
  recordMorningAttendanceSchool,
  withdrawClassHandoverSchool,
} from "~/lib/school-class-day-api";

// Die Schreibwege der Hausaufgaben (#4796), einmal für alle Renders.
const homeworkApi = {
  add: addHomeworkSchool,
  change: changeHomeworkSchool,
  remove: deleteHomeworkSchool,
};

function ClassFromAddress() {
  const schoolClass = classDayClassParam(
    useSearchParams().get(CLASS_DAY_CLASS_PARAM),
  );
  return (
    <ClassDayClass
      schoolClass={schoolClass}
      fetchClassDay={fetchClassDaySchool}
      fetchClasses={fetchClassDayClassesSchool}
      recordMorningAttendance={recordMorningAttendanceSchool}
      homework={homeworkApi}
      handOverClass={handOverClassSchool}
      withdrawClassHandover={withdrawClassHandoverSchool}
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
