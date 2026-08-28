"use client";

import { ToggleLeft } from "lucide-react";
import { useTranslations } from "next-intl";
import { ErrorPage, ErrorPageIconVisual } from "~/components/error-page";
import { Button } from "~/components/ui/button";
import { useTenantRouter } from "~/lib/tenant-router";

/**
 * Ganzseitiger Hinweis, wenn eine Seite zu einer Funktion gehört, die an
 * dieser Schule ausgeschaltet ist (Feature-Guards). Ersetzt das frühere
 * `notFound()`, das fälschlich „Schule nicht gefunden" anzeigte (#2624).
 * Texte kommen aus dem Nachrichtenkatalog (Staff-Portal: immer Deutsch).
 */
export function FeatureDisabledPage() {
  const router = useTenantRouter();
  const t = useTranslations("featureDisabledPage");

  return (
    <ErrorPage
      visual={<ErrorPageIconVisual icon={ToggleLeft} />}
      title={t("title")}
      description={t("description")}
      actions={
        <Button
          type="button"
          variant="primary"
          size="md"
          onClick={() => router.push("/dashboard")}
        >
          {t("home")}
        </Button>
      }
    />
  );
}
