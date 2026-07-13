"use client";

// /planung war die vereinheitlichte Tab-Seite (#1886) und ist jetzt ein
// Redirect: Betreuungsplan, Dienstplan und Vertretung sind eigenständige
// Bereiche mit eigenen Sidebar-Einträgen (Planung-Redesign,
// docs/planung-redesign/docs/03). `?tab=` und die Alt-Params werden in das
// neue Schema übersetzt.

import { Suspense } from "react";

import { PlanungRedirect } from "~/components/timetable/planung-redirect";

export default function PlanungRedirectPage() {
  return (
    <Suspense fallback={null}>
      <PlanungRedirect />
    </Suspense>
  );
}
