import { ChildrenOverview } from "~/components/parent/children-overview";
import { EnrollmentsOverview } from "~/components/parent/enrollments-overview";

/**
 * Parent dashboard. Stacks two sections:
 *   1. EnrollmentsOverview — every enrollment.requests row owned by
 *      the parent (in-progress, decided, or withdrawn). Hidden when
 *      empty so accounts without submissions don't see a stub.
 *   2. ChildrenOverview — every active/pending student linked to the
 *      parent across schools (the post-approval lifecycle view).
 */
export default function ParentDashboardPage() {
  return (
    <div className="space-y-8">
      <EnrollmentsOverview />
      <ChildrenOverview />
    </div>
  );
}
