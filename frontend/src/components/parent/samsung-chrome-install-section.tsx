"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { ParentSection } from "~/components/parent/shell/parent-section";
import { ButtonLink } from "~/components/ui/button";
import { isStandaloneApp } from "~/lib/push-api";
import {
  createChromeIntentUrl,
  isSamsungInternet,
} from "~/lib/pwa-install-prompt";

export function SamsungChromeInstallSection() {
  const t = useTranslations("parentSettings");
  const [pageUrl, setPageUrl] = useState<URL | null>(null);

  useEffect(() => {
    if (isSamsungInternet(window.navigator) && !isStandaloneApp()) {
      setPageUrl(new URL(window.location.href));
    }
  }, []);

  if (!pageUrl) return null;

  return (
    <ParentSection
      title={t("appInstallTitle")}
      description={t("appInstallDescription")}
      concept="devices"
    >
      <ol
        className="list-decimal space-y-2 ps-5 text-sm leading-6 text-gray-700"
        aria-label={t("appInstallStepsLabel")}
      >
        <li>{t("appInstallStep1")}</li>
        <li>{t("appInstallStep2")}</li>
        <li>{t("appInstallStep3")}</li>
      </ol>
      <ButtonLink href={createChromeIntentUrl(pageUrl)} size="touch">
        {t("openInChrome")}
      </ButtonLink>
      <p className="text-sm leading-6 text-gray-600">
        {t("appInstallFallback", { host: pageUrl.host })}
      </p>
    </ParentSection>
  );
}
