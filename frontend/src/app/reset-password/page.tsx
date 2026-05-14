"use client";

import { Suspense } from "react";
import { ResetPasswordPageContent } from "~/components/auth/reset-password-page-content";
import { Loading } from "~/components/ui/loading";

export default function ResetPasswordPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <ResetPasswordPageContent />
    </Suspense>
  );
}
