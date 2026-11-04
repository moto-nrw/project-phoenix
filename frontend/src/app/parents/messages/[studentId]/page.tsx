"use client";

import { Suspense, use } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  type MessageAudience,
  childConversationPath,
} from "~/lib/messaging-status";
import { parentPath } from "~/lib/parent-url";
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
  // `?audience=leadership` shows the conversation with the leadership only
  // (#5200); the switch writes it back, so reload and back keep the choice.
  const router = useRouter();
  const audience: MessageAudience =
    useSearchParams().get("audience") === "leadership" ? "leadership" : "team";
  // Reached only by multi-child parents picking a child from the list, so show
  // the back link and the child (to disambiguate which conversation this is).
  return (
    <OgsConversation
      studentId={studentId}
      showBack
      showChild
      audience={audience}
      onAudienceChange={(next) =>
        router.replace(parentPath(childConversationPath(studentId, next)))
      }
    />
  );
}
