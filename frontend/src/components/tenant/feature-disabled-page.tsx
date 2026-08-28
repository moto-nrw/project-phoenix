"use client";

import { ToggleLeft } from "lucide-react";
import { ErrorPage, ErrorPageIconVisual } from "~/components/error-page";
import { Button } from "~/components/ui/button";
import { useTenantRouter } from "~/lib/tenant-router";

/**
 * Ganzseitiger Hinweis, wenn eine Seite zu einer Funktion gehört, die an
 * dieser Schule ausgeschaltet ist (Feature-Guards). Ersetzt das frühere
 * `notFound()`, das fälschlich „Schule nicht gefunden" anzeigte (#2624).
 * Deutsch im Code wie das übrige Staff-Portal: die Guards rendern als
 * Client-Komponenten in der Dashboard-Shell, die bewusst nur das
 * Nav-Subset der Messages trägt — useTranslations würde hier zur Laufzeit
 * mit MISSING_MESSAGE scheitern.
 */
export function FeatureDisabledPage() {
  const router = useTenantRouter();

  return (
    <ErrorPage
      visual={<ErrorPageIconVisual icon={ToggleLeft} />}
      title="Diese Funktion ist ausgeschaltet"
      description="Ihre Schule nutzt diese Funktion zurzeit nicht. Bei Fragen wenden Sie sich an Ihre Schulleitung."
      actions={
        <Button
          type="button"
          variant="primary"
          size="md"
          onClick={() => router.push("/dashboard")}
        >
          Zur Startseite
        </Button>
      }
    />
  );
}
