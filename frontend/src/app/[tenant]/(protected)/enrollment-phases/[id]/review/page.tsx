import Link from "next/link";
import { RolloverReviewQueue } from "~/components/enrollment/rollover-review-queue";
import { MobileBackButton } from "~/components/ui/mobile-back-button";
import { PageHeaderWithSearch } from "~/components/ui/page-header";

interface PageProps {
  readonly params: Promise<{ tenant: string; id: string }>;
}

/**
 * Admin review queue for rolled-over enrollments that could not be
 * carried forward automatically (grade above cap, missing grade, etc).
 */
export default async function RolloverReviewPage({ params }: PageProps) {
  const { id } = await params;
  return (
    <div className="-mt-1.5 w-full">
      <MobileBackButton />
      <PageHeaderWithSearch
        title="Prüfliste"
        actionButton={
          <Link
            href="/enrollment-phases"
            className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            Zurück zu den Anmeldephasen
          </Link>
        }
      />
      <nav className="sr-only">
        <Link
          href="/enrollment-phases"
          className="text-sm font-medium text-[#5080D8] hover:underline"
        >
          Zurück zu den Anmeldephasen
        </Link>
      </nav>
      <RolloverReviewQueue phaseID={id} />
    </div>
  );
}
