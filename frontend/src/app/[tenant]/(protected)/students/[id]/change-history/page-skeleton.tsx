import { SkeletonRegion, TableSkeleton } from "~/components/ui/page-skeletons";

export function ChangeHistorySkeleton() {
  return (
    <SkeletonRegion label="Änderungsverlauf wird geladen">
      <TableSkeleton rows={8} columns={5} />
    </SkeletonRegion>
  );
}
