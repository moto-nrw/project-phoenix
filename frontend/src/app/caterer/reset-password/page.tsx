"use client";

import { Suspense } from "react";
import { ResetPasswordPageContent } from "~/components/auth/reset-password-page-content";
import { AuthShellSkeleton } from "~/components/auth/auth-shell";
import { confirmCatererPasswordReset } from "~/lib/auth-api";
import { catererPath } from "~/lib/caterer-url";

export default function CatererResetPasswordPage() {
  const loginPath = catererPath("/caterer/login");

  return (
    <Suspense fallback={<AuthShellSkeleton />}>
      <ResetPasswordPageContent
        confirmReset={confirmCatererPasswordReset}
        successRedirectPath={loginPath}
        backHref={loginPath}
        backLabel="Zurück zur Anmeldung"
      />
    </Suspense>
  );
}
