"use client";

import { LockSimpleIcon } from "@phosphor-icons/react";
import { useSession } from "next-auth/react";
import { usePathname } from "next/navigation";
import { useMemo, type ReactNode } from "react";

import { ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { Alert } from "~/components/ui/alert";
import {
  NotBookedSlotProvider,
  type NotBookedSlots,
} from "~/components/ui/not-booked-slot";
import { moduleOfPath, seesLockedModules } from "~/lib/module-register";
import { normalizeLocale } from "~/i18n/locales";
import { moduleNotBookedExplanation } from "~/lib/module-not-booked";
import { useModulePortal } from "~/lib/module-portal-context";
import { portalNotBookedExplanation } from "~/lib/portal-not-booked";
import { MODULE_LABELS, type ModuleKey } from "~/lib/modules";
import { normalizeTenantPathname, useTenantAwarePath } from "~/lib/tenant-path";
import {
  useBookedModules,
  useTenantRoutingModeSafe,
  useTenantSafe,
  useTenantSlugSafe,
} from "~/lib/tenant-context";

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

const NOT_BOOKED_SLOTS: NotBookedSlots = {
  Alert: ModuleNotBookedAlert,
  State: ModuleNotBookedState,
};

/** Gives the kit's error alerts and page scaffold the explanation (#5586). */
export function ModuleNotBookedSlots({
  children,
}: {
  readonly children: ReactNode;
}) {
  return (
    <NotBookedSlotProvider slots={NOT_BOOKED_SLOTS}>
      {children}
    </NotBookedSlotProvider>
  );
}

/**
 * Marks the page as one of a module the school did not book: `TenantPage`
 * keeps its head and `PageBody` shows the explanation instead of the
 * content (#4894, #6577). `null` leaves the page open.
 */
export function NotBookedPageSlot({
  module,
  children,
}: {
  readonly module: ModuleKey | null;
  readonly children: ReactNode;
}) {
  const slots = useMemo(
    () => ({ ...NOT_BOOKED_SLOTS, pageModule: module }),
    [module],
  );
  return (
    <NotBookedSlotProvider slots={slots}>{children}</NotBookedSlotProvider>
  );
}

/**
 * Ermittelt für die geschützten Seiten der Mitarbeiter-Oberfläche, ob die
 * aktuelle Seite zu einem Modul gehört, das die Schule nicht gebucht hat
 * (Modul-Register, #4894), und reicht es dem Seitengerüst über den Slot.
 *
 * Der Anbieter sitzt im Layout, die Anzeige im Seitengerüst: so laden reine
 * Weiterleitungsseiten das Gerüst nicht mit, und ein Gerüst ohne Anbieter
 * (Tests, andere Portale) gilt als offen.
 *
 * Solange die Schuldaten noch nicht geladen sind, gilt die Seite als offen:
 * sonst stünde auf jeder Modulseite kurz die Erklärung, und das Backend
 * sperrt ohnehin selbst.
 */
export function NotBookedPageProvider({
  children,
}: {
  readonly children: ReactNode;
}) {
  const tenant = useTenantSafe()?.tenant;
  const booked = useBookedModules();
  const pathname = normalizeTenantPathname(
    usePathname() ?? "/",
    useTenantSlugSafe(),
    useTenantRoutingModeSafe(),
  );
  const pageModule = tenant ? moduleOfPath(pathname) : null;
  const notBooked =
    pageModule !== null && !booked.includes(pageModule) ? pageModule : null;
  return <NotBookedPageSlot module={notBooked}>{children}</NotBookedPageSlot>;
}
