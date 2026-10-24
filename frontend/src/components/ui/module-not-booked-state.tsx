"use client";

import { LockSimpleIcon } from "@phosphor-icons/react";
import { useSession } from "next-auth/react";

import { ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { seesLockedModules } from "~/lib/module-register";
import { MODULE_LABELS, type ModuleKey } from "~/lib/modules";
import { useTenantAwarePath } from "~/lib/tenant-path";

/** Was der Zustand sagt, je nachdem, wer ihn liest. */
export function moduleNotBookedText(
  module: ModuleKey,
  leadsSchool: boolean,
): { readonly title: string; readonly description: string } {
  const label = MODULE_LABELS[module];
  return {
    title: `${label} ist nicht gebucht`,
    // Wer die Schule leitet, bucht über das moto-Team. Alle anderen fragen
    // die Leitung: sie selbst können beim moto-Team nichts buchen.
    description: leadsSchool
      ? `Diese Seite gehört zum Modul ${label}. Ihre Schule hat es nicht gebucht. Für eine Buchung wenden Sie sich an das moto-Team.`
      : `Diese Seite gehört zum Modul ${label}. Ihre Schule nutzt es nicht. Wenn Sie es brauchen, sprechen Sie mit Ihrer Leitung.`,
  };
}

/**
 * Der erklärende Zustand für ein nicht gebuchtes Modul (#4894). Er steht
 * dort, wo sonst die Seite oder ihr Inhalt stünde: beim direkten Aufruf
 * einer Modulseite und wenn eine Anfrage mit `module.not_booked` abgelehnt
 * wird. Kein Fehler, keine 404: die Schule hat das Modul nicht im Vertrag,
 * und der Text sagt, wer als Nächstes handelt.
 */
export function ModuleNotBookedState({
  module,
}: {
  readonly module: ModuleKey;
}) {
  const { data: session } = useSession();
  const tenantPath = useTenantAwarePath();
  const leads = seesLockedModules(session);
  const { title, description } = moduleNotBookedText(module, leads);
  return (
    <EmptyState
      icon={<LockSimpleIcon className="h-12 w-12" aria-hidden="true" />}
      title={title}
      description={description}
      action={
        leads ? (
          <ButtonLink
            href={tenantPath("/settings?tab=modules")}
            variant="outline"
            size="md"
          >
            Gebuchte Module ansehen
          </ButtonLink>
        ) : undefined
      }
    />
  );
}
