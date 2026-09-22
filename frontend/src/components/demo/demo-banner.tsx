"use client";

import { useEffect, useState } from "react";
import {
  ArrowCounterClockwiseIcon,
  CaretDownIcon,
} from "@phosphor-icons/react";
import { ButtonLink } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { StatusBadge } from "~/components/ui/status-badge";
import { useToast } from "~/contexts/ToastContext";
import { registerDemoVisit, trackDemoEvent } from "~/lib/analytics";
import {
  DEMO_RESTART_CONFIRM,
  DEMO_RESTART_LABEL,
  DEMO_RESTART_RUNNING,
  DEMO_RESTART_TEXT,
  DEMO_RESTART_TITLE,
  DEMO_ROLES,
  DEMO_START_URL,
  type DemoRole,
  type DemoVisit,
  demoHandoffPath,
  demoRoleLabel,
  isDemoBuild,
  isParentDemoRole,
  readDemoVisit,
  restartDemo,
  saveDemoVisit,
  startDemoSession,
  switchDemoRole,
} from "~/lib/demo-access";
import { createLogger } from "~/lib/logger";
import { useShellAuthSafe } from "~/lib/shell-auth-context";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { useTenantSafe } from "~/lib/tenant-context";

const logger = createLogger({ component: "DemoBanner" });

const SWITCH_FAILED =
  "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.";
const OPEN_MAILED_LINK =
  "Bitte öffnen Sie die Demo noch einmal über den Link aus Ihrer E-Mail.";

/**
 * Schmaler Streifen über jeder Seite der öffentlichen Demo (#3467): links
 * „Demo" und der Schulname, dann das Menü der Demo-Rolle, rechts als einziger
 * Knopf „Kostenlos starten". Es gibt ihn nur im Demo-Build, in der OGS-App
 * und in der Eltern-App (#3468). Eine Rolle der jeweils anderen App öffnet
 * diese auf ihrem eigenen Host.
 *
 * Wie der Streifen der Mitarbeiter-Vorschau liegt er fest oben (h-12) und
 * die AppShell rückt um dieselbe Höhe nach unten. So kollidiert er auf dem
 * Handy nicht mit der unteren Leiste.
 */
export function DemoBanner() {
  if (!isDemoBuild()) return null;
  return <ActiveDemoBanner />;
}

/**
 * Whether the shell shows the demo banner: in the demo build, in the OGS app
 * and the parents app (the operator portal shares the OGS shell), and not
 * during a staff preview, whose own strip takes the place.
 */
export function isDemoBannerShown(
  shellAuth: { mode: string; isPreview?: boolean } | null | undefined,
): boolean {
  return (
    isDemoBuild() &&
    (shellAuth?.mode === "teacher" || shellAuth?.mode === "parent") &&
    shellAuth.isPreview !== true
  );
}

export function useDemoBannerShown(): boolean {
  return isDemoBannerShown(useShellAuthSafe());
}

const CHOOSE_ROLE = "Rolle wählen";

function ActiveDemoBanner() {
  const tenantName = useTenantSafe()?.tenant?.name;
  const inParentsApp = useShellAuthSafe()?.mode === "parent";
  const startPath = useTenantAwarePath()("/");
  const toast = useToast();
  // Undefined until mounted: the server knows no stored visit, so reading it
  // during the first render would not match the server's markup. Null without
  // a stored visit (no storage, a direct visit): the banner then claims no
  // role and reports no identity it does not know.
  const [visit, setVisit] = useState<DemoVisit | null | undefined>();
  const [switching, setSwitching] = useState(false);
  // „Demo neu anfangen" (#3470) asks first; restarting keeps the question
  // open until the waiting room takes over.
  const [restartAsked, setRestartAsked] = useState(false);
  const [restarting, setRestarting] = useState(false);

  useEffect(() => {
    setVisit(readDemoVisit());
  }, []);

  // An entry or a switch reloads the page; it reports itself here, once.
  useEffect(() => {
    if (!visit) return;
    const { pending, ...current } = visit;
    registerDemoVisit(current);
    if (pending) {
      trackDemoEvent(pending, current);
      saveDemoVisit(current);
      setVisit(current);
    }
  }, [visit]);

  // The parents app knows no school; the entry noted its name (#3468).
  const schoolName = tenantName ?? visit?.schoolName;

  const chooseRole = async (role: DemoRole) => {
    if (role === visit?.role || switching) return;
    setSwitching(true);
    if (isParentDemoRole(role) !== inParentsApp) {
      // The other app redeems the same link on its own host; its entry
      // page reports the switch.
      globalThis.location.assign(demoHandoffPath(role));
      return;
    }
    try {
      const session = await switchDemoRole(role);
      if (!session) {
        toast.error(OPEN_MAILED_LINK);
        return;
      }
      if (!(await startDemoSession(session, "demo_role_switched"))) {
        throw new Error("demo sign-in failed");
      }
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

  // The restart leads through the waiting room, where the setup screen of
  // the first entry shows again; the new school's entry reports it.
  const restart = async () => {
    if (restarting) return;
    setRestarting(true);
    try {
      const entryUrl = await restartDemo(visit?.role);
      if (!entryUrl) {
        setRestartAsked(false);
        toast.error(OPEN_MAILED_LINK);
        return;
      }
      globalThis.location.assign(entryUrl);
    } catch (error) {
      logger.error("demo_restart_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      setRestartAsked(false);
      toast.error(SWITCH_FAILED);
    } finally {
      setRestarting(false);
    }
  };

  const roleLabel = visit ? demoRoleLabel(visit.role) : CHOOSE_ROLE;

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
      {visit === undefined ? null : visit?.fixedRole ? (
        // The standing demo school is shared; its role cannot change.
        <span className="px-2 text-sm font-medium text-gray-900">
          {roleLabel}
        </span>
      ) : (
        <OverflowMenu
          ariaLabel={visit ? `Rolle wechseln, jetzt ${roleLabel}` : roleLabel}
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
              checked: entry.role === visit?.role,
              onClick: () => void chooseRole(entry.role),
            })),
            { kind: "separator" as const },
            {
              label: DEMO_RESTART_LABEL,
              icon: <ArrowCounterClockwiseIcon aria-hidden="true" />,
              onClick: () => setRestartAsked(true),
            },
          ]}
        />
      )}
      <ConfirmationModal
        isOpen={restartAsked}
        onClose={() => {
          if (!restarting) setRestartAsked(false);
        }}
        onConfirm={() => void restart()}
        title={DEMO_RESTART_TITLE}
        confirmText={DEMO_RESTART_CONFIRM}
        confirmVariant="warning"
        isConfirmLoading={restarting}
        isDismissDisabled={restarting}
        loadingText={DEMO_RESTART_RUNNING}
      >
        <p className="text-sm text-gray-700">{DEMO_RESTART_TEXT}</p>
      </ConfirmationModal>
      <ButtonLink
        href={DEMO_START_URL}
        target="_blank"
        rel="noopener noreferrer"
        variant="success"
        size="compact"
        className="ml-auto shrink-0 text-sm"
        onClick={() => trackDemoEvent("demo_start_clicked", visit ?? null)}
      >
        Kostenlos starten
      </ButtonLink>
    </div>
  );
}
