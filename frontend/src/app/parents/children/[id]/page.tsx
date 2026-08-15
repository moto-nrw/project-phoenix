import { Suspense, use } from "react";
import { ChildPage } from "~/components/parent/child/child-page";

/** Derselbe Kinderbereich, mit dem Kind aus der Adresse vorausgewaehlt. */
export default function ParentChildPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return (
    <Suspense fallback={null}>
      <ChildPage studentId={id} />
    </Suspense>
  );
}
