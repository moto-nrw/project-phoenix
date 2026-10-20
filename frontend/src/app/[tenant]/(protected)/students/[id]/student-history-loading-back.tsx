"use client";

import { useParams } from "next/navigation";
import { TenantBackLink } from "~/components/ui/tenant-page";
import { resolveStudentReferrer, studentHistoryBack } from "./student-referrer";

/**
 * Rückweg einer Verlaufsseite, solange sie lädt. Das Skelett steht vor der
 * Suspense-Grenze um `useSearchParams`, kennt also nur die Kind-ID: es führt
 * in die Kindakte mit dem Standard-Rückweg. Beschriftung und Ziel entsprechen
 * dem geladenen Zustand ohne Fehler.
 */
export function StudentHistoryLoadingBack() {
  const studentId = useParams<{ id?: string }>()?.id;
  // Ohne ID (außerhalb der Route) bleibt nur die Liste als Ziel.
  const back = studentHistoryBack(
    studentId ?? "",
    resolveStudentReferrer(null),
    !studentId,
  );
  return <TenantBackLink href={back.backHref} label={back.backLabel} />;
}
