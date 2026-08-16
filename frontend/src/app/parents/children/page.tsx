import { Suspense } from "react";
import { ChildPage } from "~/components/parent/child/child-page";
import { ParentPageSkeleton } from "~/components/parent/parent-page";

/**
 * Bei einem Kind beginnt die Seite direkt im Profil. Bei mehreren Kindern
 * beginnt sie mit einer Auswahl und oeffnet danach das einzelne Kinderprofil.
 */
export default function ParentsChildrenPage() {
  return (
    <Suspense fallback={<ParentPageSkeleton rows={2} />}>
      <ChildPage />
    </Suspense>
  );
}
