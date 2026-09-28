"use client";

import { ChartLine } from "lucide-react";
import { SectionCard } from "~/components/ui/section-card";
import { useTenant } from "~/lib/tenant-context";

/**
 * Tells staff that their school gave the Analyse-Freigabe (#3603): while it is
 * on, the OGS portal records masked sessions under a pseudonymous ID. Shown on
 * the profile page for as long as the Freigabe lasts; read-only, because the
 * moto team switches it with the school's written consent.
 */
export function AnalyseFreigabeNotice() {
  const { tenant } = useTenant();
  if (tenant?.analyticsFreigabe !== true) return null;

  return (
    <SectionCard
      icon={ChartLine}
      headingLevel={3}
      title="Nutzungsanalyse"
      description="Ihre Schule hat zugestimmt: moto zeichnet auf, wie die App hier benutzt wird. Texte, Eingaben und Bilder sind dabei unkenntlich. So wird moto besser. Niemand wird damit bewertet. Fragen dazu beantwortet Ihre Schulleitung."
    />
  );
}
