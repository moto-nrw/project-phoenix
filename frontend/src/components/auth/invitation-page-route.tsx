"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { InvitationPageContent } from "~/components/auth/invitation-page-content";
import { Loading } from "~/components/ui/loading";

function InviteWithToken() {
  const searchParams = useSearchParams();
  return <InvitationPageContent token={searchParams.get("token")} />;
}

export function InvitationPageRoute() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <InviteWithToken />
    </Suspense>
  );
}
