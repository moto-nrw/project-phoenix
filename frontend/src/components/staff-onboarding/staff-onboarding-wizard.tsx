"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import {
  SetupBeacon,
  SetupChecklist,
  TopicList,
} from "~/components/school-setup/school-setup-checklist";
import { useSetupTour } from "~/components/school-setup/use-setup-tour";
import { CoachMark } from "~/components/ui/coach-mark";
import { ConfirmationModal } from "~/components/ui/modal";
import { hasPermission } from "~/lib/auth-utils";
import {
  buildHelpGroupHref,
  buildHelpHref,
  type HelpTopicId,
} from "~/lib/help-topics";
import { createLogger } from "~/lib/logger";
import { useShellAuthSafe } from "~/lib/shell-auth-context";
import {
  setStaffOnboardingDismissed,
  setStaffOnboardingStepState,
  type StaffOnboardingState,
  type StaffOnboardingStepKey,
} from "~/lib/staff-onboarding-api";
import { useOptionalSupervision } from "~/lib/supervision-context";
import {
  useAttendanceWebEnabled,
  useNFCEnabled,
  useOpenCareGroupMode,
  usePresenceMode,
  useTenantSlugSafe,
} from "~/lib/tenant-context";
import {
  firstOpenStaffStep,
  STAFF_NEXT_TOPICS,
  STAFF_ONBOARDING_STEP_CONTENT,
  STAFF_ONBOARDING_TOURS,
  staffOnboardingSteps,
} from "./staff-onboarding-steps";
import { useStaffOnboarding } from "./use-staff-onboarding";

const logger = createLogger({ component: "StaffOnboardingWizard" });

const SAVE_FAILED =
  "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.";
const TOUR_LEFT =
  "Die Tour ist beendet, weil Sie eine andere Seite geöffnet haben. Mit „Zeig es mir“ starten Sie sie neu.";
const CLICK_ACTION = "Klicken Sie auf die grün umrandete Stelle.";
const TARGET_MISSING =
  "Diese Stelle ist gerade nicht zu sehen. Vielleicht fehlt Ihnen ein Recht, oder die Seite lädt noch.";

/** Knopf unten rechts oder offene Checkliste. */
type View = "beacon" | "checklist";

/** Einmal pro Anmeldung öffnet sich die Checkliste von selbst. */
function claimFirstOpen(tenantSlug: string, accountID: string): boolean {
  const key = `staff-onboarding-opened:${tenantSlug}:${accountID}`;
  try {
    if (sessionStorage.getItem(key)) return false;
    sessionStorage.setItem(key, "1");
    return true;
  } catch {
    return false;
  }
}

/**
 * Die ersten Schritte für Betreuungskräfte (#3748, ADR 0043 Nachtrag).
 *
 * Dieselbe Checkliste unten rechts wie bei der Einrichtung der Schule, aber
 * für den Arbeitsalltag: Jeder Schritt zeigt per Tour, wo etwas in moto
 * steht. Ein Schritt ist erledigt, wenn die Person seine Tour bis zum Ende
 * gesehen hat. Die Checkliste erscheint erst, wenn die Schule eine Gruppe
 * oder ein Kind hat; vorher zeigten die Touren leere Seiten.
 */
export function StaffOnboardingWizard() {
  const shell = useShellAuthSafe();
  // Ohne Konto oder in der Vorschau einer Mitarbeiter-Ansicht: keine Liste.
  if (!shell?.user || shell.isPreview) return null;
  return <StaffOnboardingForPerson />;
}

function StaffOnboardingForPerson() {
  const [view, setView] = useState<View>("beacon");
  const { state, accountID, replace } = useStaffOnboarding();
  const [expanded, setExpanded] = useState<StaffOnboardingStepKey | null>(null);
  const [expandedByPerson, setExpandedByPerson] = useState(false);
  const [busy, setBusy] = useState(false);
  const [confirmDismiss, setConfirmDismiss] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const pathname = usePathname();
  const { data: session } = useSession();
  const nfcEnabled = useNFCEnabled();
  const presenceMode = usePresenceMode();
  const openCareGroupMode = useOpenCareGroupMode();
  const webAttendance = useAttendanceWebEnabled();
  const tenantSlug = useTenantSlugSafe();
  const { groups } = useOptionalSupervision();

  const hasOwnGroup =
    !openCareGroupMode && groups.some((group) => group.is_personal !== false);
  const canSeeCalendar = hasPermission(session, "calendar:own");
  const steps = useMemo(
    () =>
      state
        ? staffOnboardingSteps(state, {
            hasOwnGroup,
            webAttendance,
            canSeeCalendar,
            roomsTracked: presenceMode !== "binary",
          })
        : [],
    [state, hasOwnGroup, webAttendance, canSeeCalendar, presenceMode],
  );

  const run = useCallback(
    async (action: () => Promise<StaffOnboardingState>, after?: () => void) => {
      setBusy(true);
      setError(null);
      try {
        await replace(await action());
        after?.();
      } catch (err) {
        logger.warn("staff_onboarding_action_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError(SAVE_FAILED);
      } finally {
        setBusy(false);
      }
    },
    [replace],
  );

  // Wer die letzte Station erreicht, hat den Schritt erledigt.
  const onTourFinished = useCallback(
    (step: StaffOnboardingStepKey) => {
      setView("checklist");
      void run(() => setStaffOnboardingStepState(step, "done"));
    },
    [run],
  );
  const onTourLeft = useCallback((step: StaffOnboardingStepKey) => {
    setView("checklist");
    setExpanded(step);
    setExpandedByPerson(true);
    setNotice(TOUR_LEFT);
  }, []);
  const tour = useSetupTour(STAFF_ONBOARDING_TOURS, onTourFinished, onTourLeft);

  const visible =
    state !== null && !state.dismissed && state.schoolReady && steps.length > 0;
  const nextOpen = firstOpenStaffStep(steps);

  // Beim ersten Aufruf der Sitzung geht die Checkliste von selbst auf.
  useEffect(() => {
    if (
      visible &&
      tenantSlug &&
      accountID &&
      claimFirstOpen(tenantSlug, accountID)
    ) {
      setView("checklist");
    }
  }, [visible, tenantSlug, accountID]);

  // Aufgeklappt ist der nächste offene Schritt, solange die Person nicht
  // selbst einen anderen gewählt hat.
  const previousNext = useRef(nextOpen);
  useEffect(() => {
    if (!expandedByPerson || previousNext.current !== nextOpen) {
      setExpanded(nextOpen);
      setExpandedByPerson(false);
    }
    previousNext.current = nextOpen;
  }, [nextOpen, expandedByPerson]);

  if (!visible) return null;

  const helpContext = {
    role: "caregiver",
    nfcEnabled,
    presenceMode: presenceMode === "binary" ? "binary" : "detailed",
    groupMode: openCareGroupMode ? "open_care" : "fixed_groups",
    returnTo: pathname,
  } as const;
  const helpHref = (topic: HelpTopicId) => buildHelpHref(helpContext, topic);
  const helpGroupHref = (group: string) =>
    buildHelpGroupHref(helpContext, group);

  if (tour.active) {
    const {
      stop,
      index: stopIndex,
      total: stopTotal,
      target,
      missing,
    } = tour.active;
    return (
      <CoachMark
        searching={!target && !missing}
        target={target}
        endBefore={
          stop.endBefore && target ? target.querySelector(stop.endBefore) : null
        }
        beside={stop.nav === true}
        title={stop.title}
        text={
          missing && !target
            ? (stop.missingText ?? TARGET_MISSING)
            : tour.active.text
        }
        progress={`Station ${stopIndex + 1} von ${stopTotal}`}
        action={stop.advance === "click" && target ? CLICK_ACTION : undefined}
        onNext={stop.advance === "next" || missing ? tour.next : undefined}
        nextLabel={stopIndex + 1 === stopTotal ? "Fertig" : "Weiter"}
        onBack={tour.active.canGoBack ? tour.back : undefined}
        onClose={() => {
          tour.stop();
          setView("checklist");
        }}
      />
    );
  }

  const open = steps.filter((step) => !step.done && !step.skipped).length;

  if (view === "beacon") {
    return <SetupBeacon open={open} onOpen={() => setView("checklist")} />;
  }

  const checklistSteps = steps.map((step) => ({
    ...step,
    ...STAFF_ONBOARDING_STEP_CONTENT[step.key],
    hasTour: true,
  }));

  return (
    <>
      <SetupChecklist
        steps={checklistSteps}
        expanded={expanded}
        busy={busy}
        error={error}
        notice={notice}
        helpHref={helpHref}
        replayDone
        onExpand={(step) => {
          setError(null);
          setNotice(null);
          setExpanded(step);
          setExpandedByPerson(true);
        }}
        onStartTour={(step) => {
          setNotice(null);
          tour.start(step);
        }}
        onSkip={(step, skipped) =>
          void run(() =>
            setStaffOnboardingStepState(step, skipped ? "skipped" : "open"),
          )
        }
        onCollapse={() => setView("beacon")}
        onDismiss={() => setConfirmDismiss(true)}
        finishLabel="Abschließen"
        onFinish={() => void run(() => setStaffOnboardingDismissed(true))}
        finished={
          <>
            <div className="flex flex-col gap-1">
              <h3 className="text-base font-semibold text-gray-900">
                Geschafft!
              </h3>
              <p className="text-sm text-gray-700">
                Sie kennen jetzt die wichtigsten Handgriffe in moto.
              </p>
            </div>
            <TopicList
              title="Auch nützlich"
              topics={STAFF_NEXT_TOPICS}
              helpHref={helpHref}
              helpGroupHref={helpGroupHref}
            />
            <p className="text-sm text-gray-600">
              Mit „Abschließen“ verschwindet die Checkliste. Die Anleitungen
              bleiben unter „Hilfe“.
            </p>
          </>
        }
      />
      {/* Ausblenden ist endgültig: kein Weg zurück, die Anleitungen bleiben
          in der Hilfe. Deshalb eigens bestätigen. */}
      <ConfirmationModal
        isOpen={confirmDismiss}
        onClose={() => setConfirmDismiss(false)}
        onConfirm={() =>
          void run(
            () => setStaffOnboardingDismissed(true),
            () => setConfirmDismiss(false),
          )
        }
        title="Erste Schritte ausblenden?"
        confirmText="Ausblenden"
        isConfirmLoading={busy}
        loadingText="Wird ausgeblendet…"
      >
        <p className="text-sm text-gray-700">
          Die Checkliste verschwindet für Sie. Sie können sie nicht wieder
          einblenden.
        </p>
        <p className="mt-2 text-sm text-gray-700">
          Die Anleitungen zu allen Schritten finden Sie weiter unter „Hilfe“.
        </p>
      </ConfirmationModal>
    </>
  );
}
