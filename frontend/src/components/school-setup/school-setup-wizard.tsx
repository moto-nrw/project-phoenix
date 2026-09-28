"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { CoachMark } from "~/components/ui/coach-mark";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  buildHelpGroupHref,
  buildHelpHref,
  type HelpTopicId,
} from "~/lib/help-topics";
import { createLogger } from "~/lib/logger";
import {
  completeSchoolSetup,
  setSchoolSetupDismissed,
  setSchoolSetupStepSkipped,
  type SchoolSetupState,
  type SchoolSetupStepKey,
} from "~/lib/school-setup-api";
import { useShellAuthSafe } from "~/lib/shell-auth-context";
import { useNFCEnabled } from "~/lib/tenant-context";
import {
  SchoolSetupBeacon,
  SchoolSetupChecklist,
} from "./school-setup-checklist";
import {
  applicableSteps,
  firstOpenStep,
  setupProgress,
} from "./school-setup-steps";
import { useSchoolSetup } from "./use-school-setup";
import { useSetupTour } from "./use-setup-tour";

const logger = createLogger({ component: "SchoolSetupWizard" });

const SAVE_FAILED =
  "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.";
/** Nachladen nach dem Tourende, bis laufende Anfragen durch sind. */
const TOUR_FOLLOW_UP_MS = [500, 2000, 5000];
const TOUR_LEFT =
  "Die Tour ist beendet, weil Sie eine andere Seite geöffnet haben. Mit „Zeig es mir“ starten Sie sie neu.";
const CLICK_ACTION = "Klicken Sie auf die grün umrandete Stelle.";
const TARGET_MISSING =
  "Diese Stelle ist gerade nicht zu sehen. Vielleicht fehlt Ihnen ein Recht, oder die Seite lädt noch.";

/** Knopf unten rechts oder offene Checkliste. */
type View = "beacon" | "checklist";

/** Einmal pro Anmeldung öffnet sich der Assistent von selbst. */
function claimFirstOpen(accountID: string): boolean {
  const key = `school-setup-opened:${accountID}`;
  try {
    if (sessionStorage.getItem(key)) return false;
    sessionStorage.setItem(key, "1");
    return true;
  } catch {
    return false;
  }
}

/**
 * Der Einrichtungs-Assistent für neue Schulen (#2832, ADR 0040).
 *
 * Eine Checkliste unten rechts, über allen Seiten. Der nächste offene
 * Schritt ist aufgeklappt; „Zeig es mir“ führt per Tour durch die
 * Seitenleiste und die Seite. Er
 * zeigt sich nur Personen, die die Einstellungen der Schule ändern dürfen,
 * bis die Schule fertig ist oder die Person ihn ausblendet.
 */
export function SchoolSetupWizard() {
  const shell = useShellAuthSafe();
  // Ohne Konto oder in der Vorschau einer Mitarbeiter-Ansicht: kein Assistent.
  if (!shell?.user || shell.isPreview) return null;
  return <SchoolSetupWizardForAdmin />;
}

function SchoolSetupWizardForAdmin() {
  const [view, setView] = useState<View>("beacon");
  const { state, accountID, refresh, replace } = useSchoolSetup(
    view === "checklist",
  );
  const [expanded, setExpanded] = useState<SchoolSetupStepKey | null>(null);
  const [expandedByPerson, setExpandedByPerson] = useState(false);
  const [busy, setBusy] = useState(false);
  const [confirmDismiss, setConfirmDismiss] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const pathname = usePathname();
  const nfcEnabled = useNFCEnabled();
  // Die letzte Station löst oft eine Anfrage aus (Einladung senden, Kind
  // anlegen), die beim Tourende noch läuft. Mehrmals nachladen, damit der
  // Haken ohne Umweg erscheint.
  const followUps = useRef<number[]>([]);
  useEffect(
    () => () => followUps.current.forEach((id) => window.clearTimeout(id)),
    [],
  );
  const onTourFinished = useCallback(() => {
    setView("checklist");
    void refresh();
    followUps.current = TOUR_FOLLOW_UP_MS.map((delay) =>
      window.setTimeout(() => void refresh(), delay),
    );
  }, [refresh]);
  // Hat die Person die Seite der Tour verlassen, endet die Tour. Die
  // Checkliste geht beim Schritt auf und sagt, warum, statt dass nur die
  // Abdunklung stehen bleibt.
  const onTourLeft = useCallback((step: SchoolSetupStepKey) => {
    setView("checklist");
    setExpanded(step);
    setExpandedByPerson(true);
    setNotice(TOUR_LEFT);
  }, []);
  const tour = useSetupTour(onTourFinished, onTourLeft);

  const visible = state !== null && !state.completed && !state.dismissed;
  const nextOpen = state ? firstOpenStep(state) : null;

  // Beim ersten Aufruf der Sitzung geht die Checkliste von selbst auf.
  useEffect(() => {
    if (visible && accountID && claimFirstOpen(accountID)) {
      setView("checklist");
    }
  }, [visible, accountID]);

  // Aufgeklappt ist der nächste offene Schritt, solange die Person nicht
  // selbst einen anderen gewählt hat. Wird er erledigt, rückt der nächste nach.
  const previousNext = useRef(nextOpen);
  useEffect(() => {
    if (!expandedByPerson || previousNext.current !== nextOpen) {
      setExpanded(nextOpen);
      setExpandedByPerson(false);
    }
    previousNext.current = nextOpen;
  }, [nextOpen, expandedByPerson]);

  if (!visible) return null;

  const steps = applicableSteps(state);
  const { finished, total } = setupProgress(state);

  // Rolle und Einstellungen der Schule reisen mit, damit die Hilfe die
  // passenden Abläufe zeigt.
  const helpContext = {
    role: "lead",
    nfcEnabled,
    presenceMode: state.basics.presenceMode,
    groupMode: state.basics.groupMode,
    returnTo: pathname,
  } as const;
  const helpHref = (topic: HelpTopicId) => buildHelpHref(helpContext, topic);
  const helpGroupHref = (group: string) =>
    buildHelpGroupHref(helpContext, group);

  const run = async (
    action: () => Promise<SchoolSetupState>,
    after?: (next: SchoolSetupState) => void,
  ) => {
    setBusy(true);
    setError(null);
    try {
      const next = await action();
      await replace(next);
      after?.(next);
    } catch (err) {
      logger.warn("school_setup_action_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(SAVE_FAILED);
    } finally {
      setBusy(false);
    }
  };

  // Während der Tour tritt die Checkliste zurück.
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
        // Dasselbe Element über alle Stationen: Die Abdunklung bleibt beim
        // Wechsel stehen, der Bildschirm blitzt nicht auf. Solange die Stelle
        // gesucht wird, wartet nur die Sprechblase; in der Mitte steht sie
        // nur, wenn die Stelle fehlt.
        searching={!target && !missing}
        target={target}
        endBefore={
          stop.endBefore && target ? target.querySelector(stop.endBefore) : null
        }
        // Stationen in der Seitenleiste: neben die Leiste, nicht darüber.
        // Sonst verdeckt die Sprechblase die Einträge darunter -- also
        // genau den Eintrag, den die nächste Station meint.
        beside={stop.nav === true}
        title={stop.title}
        text={
          missing && !target ? (stop.missingText ?? TARGET_MISSING) : stop.text
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

  if (view === "beacon") {
    return (
      <SchoolSetupBeacon
        open={total - finished}
        onOpen={() => {
          setView("checklist");
          void refresh();
        }}
      />
    );
  }

  return (
    <>
      <SchoolSetupChecklist
        steps={steps}
        expanded={expanded}
        busy={busy}
        error={error}
        notice={notice}
        helpHref={helpHref}
        helpGroupHref={helpGroupHref}
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
          void run(() => setSchoolSetupStepSkipped(step, skipped))
        }
        onComplete={() => void run(completeSchoolSetup)}
        onCollapse={() => setView("beacon")}
        onDismiss={() => setConfirmDismiss(true)}
      />
      {/* Ausblenden ist endgültig: kein Weg zurück in den Einstellungen,
        die Anleitungen bleiben in der Hilfe. Deshalb eigens bestätigen. */}
      <ConfirmationModal
        isOpen={confirmDismiss}
        onClose={() => setConfirmDismiss(false)}
        onConfirm={() =>
          void run(
            () => setSchoolSetupDismissed(true),
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
