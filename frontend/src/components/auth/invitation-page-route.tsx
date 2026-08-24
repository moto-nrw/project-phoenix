"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { InvitationPageContent } from "~/components/auth/invitation-page-content";
import { Loading } from "~/components/ui/loading";

function InviteWithToken({ redirectToPath }: { redirectToPath?: string }) {
  const searchParams = useSearchParams();
  return (
    <InvitationPageContent
      token={searchParams.get("token")}
      redirectToPath={redirectToPath}
    />
  );
}

export function InvitationPageRoute({
  redirectToPath,
}: {
  /** Post-accept redirect override — see InvitationAcceptForm (#2207). */
  redirectToPath?: string;
} = {}) {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <InviteWithToken redirectToPath={redirectToPath} />
    </Suspense>
  );
}
