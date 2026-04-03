"use client";

import { Suspense } from "react";
import { Loading } from "~/components/ui/loading";
import { OperatorInvitePageContent } from "~/components/operator/operator-invite-page-content";

export default function OperatorInvitePage() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 p-4">
      <Suspense fallback={<Loading />}>
        <OperatorInvitePageContent />
      </Suspense>
    </div>
  );
}
