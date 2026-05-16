import Link from "next/link";
import { RolloverReviewQueue } from "~/components/enrollment/rollover-review-queue";

interface PageProps {
  readonly params: Promise<{ tenant: string; id: string }>;
}

/**
 * Admin review queue for rolled-over enrollments that couldn't be
 * carried forward automatically (grade above cap, missing grade, etc).
 * Reached from the phases page via the "Prüfliste" link on any phase
 * that has rollover_source_phase_id set.
 */
export default async function RolloverReviewPage({ params }: PageProps) {
  const { id } = await params;
  return (
    <div className="mx-auto max-w-5xl space-y-6 p-6">
      <nav>
        <Link
          href="/enrollment-phases"
          className="text-sm font-medium text-[#5080D8] hover:underline"
        >
          ← Zurück zu den Phasen
        </Link>
      </nav>
      <RolloverReviewQueue phaseID={id} />
    </div>
  );
}
