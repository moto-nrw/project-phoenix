"use client";

import { Suspense } from "react";
import { ResetPasswordPageContent } from "~/components/auth/reset-password-page-content";
import { AuthShellSkeleton } from "~/components/auth/auth-shell";
import { confirmTraegerPasswordReset } from "~/lib/auth-api";
import { traegerPath } from "~/lib/traeger-url";

export default function TraegerResetPasswordPage() {
  const loginPath = traegerPath("/traeger/login");

  return (
    <Suspense fallback={<AuthShellSkeleton />}>
      <ResetPasswordPageContent
        confirmReset={confirmTraegerPasswordReset}
        successRedirectPath={loginPath}
        backHref={loginPath}
        backLabel="Zurück zur Anmeldung"
      />
    </Suspense>
  );
}
