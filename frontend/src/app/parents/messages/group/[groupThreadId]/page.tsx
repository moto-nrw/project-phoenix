"use client";

import { Suspense, use } from "react";
import { ParentGroupThreadView } from "~/components/parent/messages/group-thread-view";
import { ParentPageSkeleton } from "~/components/parent/parent-page";

export default function ParentGroupThreadPage({
  params,
}: {
  readonly params: Promise<{ groupThreadId: string }>;
}) {
  return (
    <Suspense fallback={<ParentPageSkeleton rows={2} />}>
      <ParentGroupThreadContent params={params} />
    </Suspense>
  );
}

function ParentGroupThreadContent({
  params,
}: {
  readonly params: Promise<{ groupThreadId: string }>;
}) {
  const { groupThreadId } = use(params);
  return <ParentGroupThreadView groupThreadId={groupThreadId} />;
}
