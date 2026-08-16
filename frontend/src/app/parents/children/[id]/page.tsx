import { Suspense, use } from "react";
import { ChildPage } from "~/components/parent/child/child-page";
import { ParentPageSkeleton } from "~/components/parent/parent-page";

/** Derselbe Kinderbereich, mit dem Kind aus der Adresse vorausgewaehlt. */
export default function ParentChildPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return (
    <Suspense fallback={<ParentPageSkeleton rows={2} />}>
      <ChildPage studentId={id} />
    </Suspense>
  );
}
