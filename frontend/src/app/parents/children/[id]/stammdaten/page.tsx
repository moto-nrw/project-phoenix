import { use } from "react";
import { ChildMasterDataView } from "~/components/parent/child-master-data";

/**
 * Per-child Stammdaten page. Shows the structured master-data view with
 * Track A direct edits and Track B change requests. studentId from the URL.
 */
export default function ParentChildMasterDataPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return <ChildMasterDataView studentId={id} />;
}
