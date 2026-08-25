"use client";

import { Suspense } from "react";
import { ResetPasswordPageContent } from "~/components/auth/reset-password-page-content";
import { AuthShellSkeleton } from "~/components/auth/auth-shell";
import { confirmSchoolPasswordReset } from "~/lib/auth-api";
import { schoolPath } from "~/lib/school-url";

export default function SchoolResetPasswordPage() {
  const loginPath = schoolPath("/school/login");

  return (
    <Suspense fallback={<AuthShellSkeleton />}>
      <ResetPasswordPageContent
        confirmReset={confirmSchoolPasswordReset}
        successRedirectPath={loginPath}
        backHref={loginPath}
        backLabel="Zurück zur Anmeldung"
      />
    </Suspense>
  );
}
