import { Suspense } from "react";
import { EnrollmentEditPage } from "~/components/enrollment/enrollment-edit-page";
import { PublicEnrollmentContentSkeleton } from "~/components/enrollment/public-enrollment-shell";

interface PageProps {
  readonly params: Promise<{ token: string }>;
}

// Reduced Halbjahreswechsel flow (#2251): offerings/weekdays only.
export default function ParentEnrollmentAdjustRoute({ params }: PageProps) {
  return (
    <Suspense
      fallback={
        <main className="moto-dotted-background moto-dotted-background--fullscreen min-h-screen px-4 py-5 sm:px-6 sm:py-6">
          <PublicEnrollmentContentSkeleton sections={3} />
        </main>
      }
    >
      <EnrollmentEditPage params={params} adjustOnly />
    </Suspense>
  );
}
