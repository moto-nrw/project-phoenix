"use client";

// Startseite des Schul-Portals ("moto schule", #2207): die Tagesübersicht
// der Klassenansicht. Die Kinderlisten liegen unter /school/klasse?klasse=…
// (#2294), eine Klasse pro Seite.

import { ClassDayOverview } from "~/components/class-day/class-day-overview";
import { TodayNoticesCard } from "~/components/school/today-notices-card";
import {
  fetchClassDaySchool,
  fetchClassDayClassesSchool,
  fetchSchoolDaySchool,
  reportAbsenceSchool,
  withdrawAbsenceReportSchool,
} from "~/lib/school-class-day-api";

export default function SchoolHomePage() {
  return (
    <ClassDayOverview
      fetchClasses={fetchClassDayClassesSchool}
      fetchClassDay={fetchClassDaySchool}
      fetchSchoolDay={fetchSchoolDaySchool}
      reportAbsence={reportAbsenceSchool}
      withdrawAbsenceReport={withdrawAbsenceReportSchool}
      // Hinweise der OGS-Leitung für heute (#2208): direkt unter dem Kopf;
      // ohne Hinweis rendert die Karte nichts.
      notices={<TodayNoticesCard />}
    />
  );
}
