"use client";

import { use } from "react";
import { OgsConversation } from "~/components/parent/ogs-conversation";

export default function ParentChildConversationPage({
  params,
}: {
  readonly params: Promise<{ studentId: string }>;
}) {
  const { studentId } = use(params);
  // Reached only by multi-child parents picking a child from the list, so show
  // the back link and the child (to disambiguate which conversation this is).
  return <OgsConversation studentId={studentId} showBack showChild />;
}
