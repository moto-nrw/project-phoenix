import { ChildrenOverview } from "~/components/parent/children-overview";

/**
 * Parent dashboard. Lists every child linked to the calling parent's
 * account across every school they have access to. Data source:
 * GET /api/parent/me/children → backend cross-tenant aggregation
 * (PR 9 commit 5).
 */
export default function ParentDashboardPage() {
  return <ChildrenOverview />;
}
