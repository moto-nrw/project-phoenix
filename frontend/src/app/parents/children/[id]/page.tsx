import { use } from "react";
import { ChildDetail } from "~/components/parent/child-detail";

/**
 * Per-child detail page. studentId comes from the URL segment.
 * Read-only stub — see ChildDetail for the rendered fields.
 */
export default function ParentChildPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return <ChildDetail studentId={id} />;
}
