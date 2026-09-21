"use client";

import { useEffect, useState } from "react";
import { signIn } from "next-auth/react";
import { CaretDownIcon } from "@phosphor-icons/react";
import { ButtonLink } from "~/components/ui/button";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { StatusBadge } from "~/components/ui/status-badge";
import { useToast } from "~/contexts/ToastContext";
import { registerDemoVisit, trackDemoEvent } from "~/lib/analytics";
import {
  DEMO_ROLES,
  DEMO_START_URL,
  type DemoRole,
  type DemoVisit,
  demoRoleLabel,
  isDemoBuild,
  readDemoVisit,
  saveDemoVisit,
  switchDemoRole,
} from "~/lib/demo-access";
import { createLogger } from "~/lib/logger";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { useTenantSafe } from "~/lib/tenant-context";

const logger = createLogger({ component: "DemoBanner" });

const SWITCH_FAILED =
  "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.";
const LINK_EXPIRED =
  "Ihr Demo-Link gilt nicht mehr. Auf unserer Website bekommen Sie einen neuen.";

/**
 * Schmaler Streifen über jeder Seite der öffentlichen Demo (#3467): links
 * „Demo" und der Schulname, dann das Menü der Demo-Rolle, rechts als einziger
 * Knopf „Kostenlos starten". Es gibt ihn nur im Demo-Build.
 *
 * Wie der Streifen der Mitarbeiter-Vorschau liegt er fest oben (h-12) und
 * die AppShell rückt um dieselbe Höhe nach unten. So kollidiert er auf dem
 * Handy nicht mit der unteren Leiste.
 */
export function DemoBanner() {
  if (!isDemoBuild()) return null;
  return <ActiveDemoBanner />;
}

// Before a visit is known (no storage, a direct visit) the banner shows
// "Alle Funktionen", which is what the standing demo school signs in as.
const UNKNOWN_VISIT: DemoVisit = { accessId: "", role: "all" };

function ActiveDemoBanner() {
  const schoolName = useTenantSafe()?.tenant?.name;
  const startPath = useTenantAwarePath()("/");
  const toast = useToast();
  const [visit, setVisit] = useState<DemoVisit>(
    () => readDemoVisit() ?? UNKNOWN_VISIT,
  );
  const [switching, setSwitching] = useState(false);

  // An entry or a switch reloads the page; it reports itself here, once.
  useEffect(() => {
    const { pending, ...current } = visit;
    registerDemoVisit(current);
    if (pending) {
      trackDemoEvent(pending, current);
      saveDemoVisit(current);
      setVisit(current);
    }
  }, [visit]);

  const chooseRole = async (role: DemoRole) => {
    if (role === visit.role || switching) return;
    setSwitching(true);
    try {
      const session = await switchDemoRole(role);
      if (!session) {
        toast.error(LINK_EXPIRED);
        return;
      }
      const result = await signIn("credentials", {
        redirect: false,
        internalRefresh: true,
        token: session.access_token,
        refreshToken: session.refresh_token,
      });
      if (result?.error) throw new Error(result.error);
      saveDemoVisit({ ...session.visit, pending: "demo_role_switched" });
      // Volle Neuladung: Seitenleiste und Startseite folgen den Rechten der
      // neuen Sitzung.
      globalThis.location.assign(startPath);
    } catch (error) {
      logger.error("demo_role_switch_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      toast.error(SWITCH_FAILED);
    } finally {
      setSwitching(false);
    }
  };

  const roleLabel = demoRoleLabel(visit.role);

  return (
    <div
      role="region"
      aria-label="Demo"
      className="fixed inset-x-0 top-0 z-50 flex h-12 items-center gap-2 border-b border-gray-200 bg-white px-4 shadow-sm"
    >
      <StatusBadge tone="orange" label="Demo" compact />
      {schoolName ? (
        <span className="hidden min-w-0 truncate text-sm font-medium text-gray-900 sm:inline">
          {schoolName}
        </span>
      ) : null}
      <OverflowMenu
        ariaLabel={`Rolle wechseln, jetzt ${roleLabel}`}
        triggerContent={
          <span className="inline-flex h-8 items-center gap-1 rounded-md px-2 text-sm font-medium text-gray-900 hover:bg-gray-100">
            {roleLabel}
            <CaretDownIcon aria-hidden="true" className="size-4" />
          </span>
        }
        items={[
          { kind: "header", label: "Demo ansehen als" },
          ...DEMO_ROLES.map((entry) => ({
            kind: "radio" as const,
            label: entry.label,
            checked: entry.role === visit.role,
            onClick: () => void chooseRole(entry.role),
          })),
        ]}
      />
      <ButtonLink
        href={DEMO_START_URL}
        target="_blank"
        rel="noopener noreferrer"
        variant="success"
        size="compact"
        className="ml-auto shrink-0 text-sm"
        onClick={() => trackDemoEvent("demo_start_clicked", visit)}
      >
        Kostenlos starten
      </ButtonLink>
    </div>
  );
}
