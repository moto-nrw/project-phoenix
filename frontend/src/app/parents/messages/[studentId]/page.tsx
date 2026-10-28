"use client";

import { Suspense, use } from "react";
import { useSearchParams } from "next/navigation";
import { OgsConversation } from "~/components/parent/ogs-conversation";
import { ParentPageSkeleton } from "~/components/parent/parent-page";

export default function ParentChildConversationPage({
  params,
}: {
  readonly params: Promise<{ studentId: string }>;
}) {
  return (
    <Suspense fallback={<ParentPageSkeleton rows={1} />}>
      <ParentChildConversationContent params={params} />
    </Suspense>
  );
}

function ParentChildConversationContent({
  params,
}: {
  readonly params: Promise<{ studentId: string }>;
}) {
  const { studentId } = use(params);
  // `?audience=leadership` opens the conversation with the leadership only
  // (#5200); every other link opens the one with the whole team.
  const leadership = useSearchParams().get("audience") === "leadership";
  // Reached only by multi-child parents picking a child from the list, so show
  // the back link and the child (to disambiguate which conversation this is).
  return (
    <OgsConversation
      studentId={studentId}
      showBack
      showChild
      initialAudience={leadership ? "leadership" : "team"}
    />
  );
}
