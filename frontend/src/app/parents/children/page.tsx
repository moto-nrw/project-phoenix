import { Suspense } from "react";
import { ChildPage } from "~/components/parent/child/child-page";

/**
 * Der Kinderbereich. Bei einem Kind zeigt er direkt dieses Kind, bei mehreren
 * einen Umschalter (Entscheidung E9). Deshalb gibt es hier keine Liste.
 */
export default function ParentsChildrenPage() {
  return (
    <Suspense fallback={null}>
      <ChildPage />
    </Suspense>
  );
}
