"use client";

// Startseite des Schul-Portals ("moto schule", #2207): die Klassenansicht.
// Gleiche geteilte Komponente wie der (übergangsweise) Tenant-Mount, aber
// mit den school-Session-Fetchern.

import { ClassDayView } from "~/components/class-day/class-day-view";
import {
  fetchClassDaySchool,
  fetchMyClassesSchool,
} from "~/lib/school-class-day-api";

export default function SchoolHomePage() {
  return (
    <ClassDayView
      fetchMyClasses={fetchMyClassesSchool}
      fetchClassDay={fetchClassDaySchool}
    />
  );
}
