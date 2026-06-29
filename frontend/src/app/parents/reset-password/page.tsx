"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { ResetPasswordPageContent } from "~/components/auth/reset-password-page-content";
import { buildParentAuthShellCopy } from "~/components/auth/parent-auth-shell-copy";
import { Loading } from "~/components/ui/loading";
import { confirmParentPasswordReset } from "~/lib/auth-api";
import { parentPath } from "~/lib/parent-url";

const parentLoginPath = "/parents/login";

export default function ParentResetPasswordPage() {
  const t = useTranslations("parentPasswordResetPage");
  const tAuthShell = useTranslations("parentAuthShell");
  const [loginPath, setLoginPath] = useState(parentLoginPath);
  const testimonialPanelCopy = useMemo(
    () => buildParentAuthShellCopy(tAuthShell),
    [tAuthShell],
  );

  useEffect(() => {
    setLoginPath(parentPath(parentLoginPath));
  }, []);

  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <ResetPasswordPageContent
        confirmReset={confirmParentPasswordReset}
        successRedirectPath={loginPath}
        backHref={loginPath}
        backLabel={t("backToLogin")}
        testimonialPanelCopy={testimonialPanelCopy}
        copy={{
          missingToken: t("errors.missingToken"),
          invalidToken: t("errors.invalidToken"),
          passwordTooShort: t("validation.tooShort"),
          passwordMissingUppercase: t("validation.missingUppercase"),
          passwordMissingLowercase: t("validation.missingLowercase"),
          passwordMissingNumber: t("validation.missingNumber"),
          passwordMissingSpecial: t("validation.missingSpecial"),
          passwordMismatch: t("validation.passwordMismatch"),
          genericError: t("errors.generic"),
          invalidRequest: t("errors.invalidRequest"),
          expiredLink: t("errors.expired"),
          notFoundLink: t("errors.notFound"),
          successEyebrow: t("success.eyebrow"),
          successTitle: t("success.title"),
          successSubtitle: t("success.subtitle"),
          successBody: t("success.body"),
          formEyebrow: t("form.eyebrow"),
          formTitle: t("form.title"),
          formSubtitle: t("form.subtitle"),
          passwordLabel: t("form.passwordLabel"),
          confirmPasswordLabel: t("form.confirmPasswordLabel"),
          showPassword: t("form.showPassword"),
          hidePassword: t("form.hidePassword"),
          requirementsTitle: t("requirements.title"),
          requirements: [
            t("requirements.minLength"),
            t("requirements.mixedCase"),
            t("requirements.number"),
            t("requirements.special"),
          ],
          submitting: t("form.submitting"),
          submit: t("form.submit"),
        }}
      />
    </Suspense>
  );
}
