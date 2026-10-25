"use client";

import { LockSimpleIcon } from "@phosphor-icons/react";
import { useSession } from "next-auth/react";

import { ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { Alert } from "~/components/ui/alert";
import { seesLockedModules } from "~/lib/module-register";
import { normalizeLocale } from "~/i18n/locales";
import { moduleNotBookedExplanation } from "~/lib/module-not-booked";
import { useModulePortal } from "~/lib/module-portal-context";
import { portalNotBookedExplanation } from "~/lib/portal-not-booked";
import { MODULE_LABELS, type ModuleKey } from "~/lib/modules";
import { useTenantAwarePath } from "~/lib/tenant-path";

/** Derselbe Hinweis für einen Fehlerkasten innerhalb einer bestehenden Seite. */
export function ModuleNotBookedAlert({
  module,
  className,
}: {
  readonly module: ModuleKey;
  readonly className?: string;
}) {
  const { data: session } = useSession();
  const portal = useModulePortal();
  // The parents and the school portal follow a module of their own (#4895).
  const description =
    portal === "staff"
      ? moduleNotBookedExplanation(module, seesLockedModules(session))
      : portalNotBookedExplanation(
          portal,
          normalizeLocale(
            typeof document === "undefined"
              ? undefined
              : document.documentElement.lang,
          ),
        );
  return <Alert type="info" message={description} className={className} />;
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
  const title = `${MODULE_LABELS[module]} ist nicht gebucht`;
  const description = moduleNotBookedExplanation(module, leads);
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
