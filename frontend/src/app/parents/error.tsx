"use client";

import * as Sentry from "@sentry/nextjs";
import { useTranslations } from "next-intl";
import { useEffect } from "react";
import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";
import { ButtonLink } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { parentPath } from "~/lib/parent-url";

/**
 * A page of the parents portal crashed while rendering (#2518). Without this
 * boundary the German-only global error page took over, which also sent
 * families to a "Support" they do not have. Here the crash sentence of the
 * error catalog speaks the parent's language, offers Wiederholen and a way
 * back to the start page; the parent shell stays in place.
 */
export default function ParentError({
  error,
  retry,
}: {
  readonly error: Error & { digest?: string };
  readonly retry: () => void;
}) {
  const t = useTranslations("parentCrash");
  const catalog = useTranslations("errorCatalog.actions");

  useEffect(() => {
    Sentry.captureException(error);
  }, [error]);

  const message = catalog("crash", { object: t("object") });

  return (
    <ParentPage>
      <ParentPageHeader title={t("title")} />
      <LoadErrorAlert
        error={{
          attempt: 1,
          message: message.charAt(0).toUpperCase() + message.slice(1),
          retry: { label: catalog("retry"), onClick: retry },
        }}
      />
      <ButtonLink href={parentPath("/parents")} variant="outline" size="md">
        {t("home")}
      </ButtonLink>
    </ParentPage>
  );
}
