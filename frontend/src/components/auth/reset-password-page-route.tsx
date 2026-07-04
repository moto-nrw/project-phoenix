"use client";

import { Suspense } from "react";
import { ResetPasswordPageContent } from "~/components/auth/reset-password-page-content";
import { Loading } from "~/components/ui/loading";

export function ResetPasswordPageRoute() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <ResetPasswordPageContent />
    </Suspense>
  );
}
