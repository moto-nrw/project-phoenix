"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useSession } from "next-auth/react";
import { Button, ButtonLink } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { createLogger } from "~/lib/logger";
import { resetStudentPickupToOffering } from "~/lib/pickup-schedule-api";
import {
  describeAdjustment,
  formatRemovalSpan,
  formatWeekdayAdverbs,
  lastGapDay,
  type ManualRosterTemplate,
  type PickupOverride,
  removeStudentTemplateDays,
  type RosterImpact,
} from "~/lib/roster-impact";
import { useTenantAwarePath } from "~/lib/tenant-path";

const logger = createLogger({ component: "CarePlanImpact" });

/**
 * Was die OGS in der Karte schon erledigt hat. Zeigt eine Liste die Karte
 * nach einem Wechsel erneut an, stellt sie damit den Stand wieder her.
 */
export interface CarePlanImpactProgress {
  /** Schlüssel der Regeltermine, aus denen das Kind entfernt ist. */
  readonly removed: readonly string[];
  /** Wochentage, deren gebuchte Abholzeit übernommen ist. */
  readonly resetWeekdays: readonly number[];
}

interface CarePlanImpactProps {
  readonly impact: RosterImpact;
  readonly progress?: CarePlanImpactProgress;
  readonly onProgress?: (progress: CarePlanImpactProgress) => void;
  /** Ohne Kind-ID gibt es nichts zu entfernen; die Liste bleibt sichtbar. */
  readonly studentId?: string;
  /** Vorname, wenn eine Seite mehrere Kinder zeigt. */
  readonly childName?: string;
}

/**
 * „Auswirkung auf den Betreuungsplan“ nach einer Buchungsänderung (#5102):
 * was moto automatisch angepasst hat und welche von Hand gepflegten
 * Regeltermine das Kind an weggefallenen Tagen weiter einplanen. Plant ein
 * Regeltermin über Klasse, Jahrgang oder Gruppe, hält ein Entfernen nicht;
 * dort führt der Hinweis zu „Mit Angebot verbinden“ (#5137). Der Hinweis
 * steht einmal über der Liste, nicht in jeder Zeile.
 */
export function CarePlanImpact({
  impact,
  studentId,
  childName,
  progress,
  onProgress,
}: CarePlanImpactProps) {
  const { data: session } = useSession();
  const canManage = hasPermission(session, "schedules:manage");
  const canRemove = studentId !== undefined && canManage;
  const firstRemovableDay = earliestRemovalDay(
    impact.effectiveFrom,
    useBerlinToday(),
  );
  const [removed, setRemoved] = useState<ReadonlySet<string>>(
    () => new Set(progress?.removed),
  );
  const [resetWeekdays, setResetWeekdays] = useState<ReadonlySet<number>>(
    () => new Set(progress?.resetWeekdays),
  );
  const markReset = useCallback(
    (weekday: number) =>
      setResetWeekdays((current) => new Set(current).add(weekday)),
    [],
  );
  // Den erledigten Stand nach oben melden, damit eine Liste ihn behält.
  // Die Ref folgt dem Callback im Effect, nicht beim Rendern (React darf
  // ein Rendern wiederholen). Der Effekt darunter meldet den Stand danach.
  const onProgressRef = useRef(onProgress);
  useEffect(() => {
    onProgressRef.current = onProgress;
  }, [onProgress]);
  useEffect(() => {
    onProgressRef.current?.({
      removed: [...removed],
      resetWeekdays: [...resetWeekdays],
    });
  }, [removed, resetWeekdays]);
  const [target, setTarget] = useState<ManualRosterTemplate | null>(null);
  const [removing, setRemoving] = useState(false);
  const errors = useApiFormError();
  const toast = useToast();
  const tenantPath = useTenantAwarePath();
  const stichtag = formatDate(impact.effectiveFrom);
  const noRosterChange =
    impact.adjusted.length === 0 && impact.notAdjusted.length === 0;

  if (noRosterChange && impact.pickupOverrides.length === 0) {
    return (
      <p className="text-sm text-gray-600">
        Im Betreuungsplan musste kein Regeltermin angepasst werden.
      </p>
    );
  }

  const planLink = tenantPath(
    `/betreuungsplan?view=serien&d=${encodeURIComponent(impact.effectiveFrom)}`,
  );
  const connectLink = (templateId: string) =>
    `${planLink}&verbinden=${encodeURIComponent(templateId)}`;
  const anyViaTarget = impact.notAdjusted.some(
    (template) => template.viaTarget,
  );
  // Ohne Recht fehlt an einem Eintrag eine Aktion, die andere sehen würden.
  const actionMissing =
    !canManage &&
    impact.notAdjusted.some(
      (template) => template.viaTarget || studentId !== undefined,
    );

  const closeConfirm = () => {
    if (removing) return;
    setTarget(null);
    errors.clear();
  };

  const confirmRemoval = async () => {
    if (!target || studentId === undefined) return;
    setRemoving(true);
    errors.clear();
    try {
      await removeStudentTemplateDays({
        templateId: target.templateId,
        studentId,
        weekdays: target.weekdays,
        effectiveFrom: impact.effectiveFrom,
        effectiveUntil: target.until,
      });
      setRemoved((current) => new Set(current).add(manualTemplateKey(target)));
      toast.success(`Das Kind ist aus „${target.name}“ entfernt.`);
      setTarget(null);
    } catch (err) {
      logger.warn("care_plan_impact_remove_failed", {
        error: err instanceof Error ? err.message : String(err),
        template_id: target.templateId,
      });
      await errors.show(err, { object: "das Entfernen aus dem Regeltermin" });
    } finally {
      setRemoving(false);
    }
  };

  return (
    <SectionCard
      title={`Auswirkung auf den Betreuungsplan${childName ? ` für ${childName}` : ""} ab ${stichtag}`}
      headingLevel={4}
      namedRegion
      titleSize="sm"
      bodyClassName="mt-3 flex flex-col gap-5"
    >
      {noRosterChange ? (
        <p className="text-sm text-gray-600">
          Kein Regeltermin musste angepasst werden.
        </p>
      ) : null}
      {impact.pickupOverrides.length > 0 ? (
        <PickupOverrides
          overrides={impact.pickupOverrides}
          effectiveFrom={impact.effectiveFrom}
          studentId={studentId}
          done={resetWeekdays}
          onReset={markReset}
        />
      ) : null}
      {impact.adjusted.length > 0 ? (
        <SectionCard
          title="Automatisch angepasst"
          headingLevel={5}
          titleSize="sm"
          titleClassName="font-medium text-gray-600"
          bare
          className="space-y-2"
        >
          <ul className="divide-y divide-gray-100 text-sm text-gray-700">
            {impact.adjusted.map((template) => (
              <li
                key={template.templateId}
                className="py-2 first:pt-0 last:pb-0"
              >
                <span className="font-medium text-gray-900">
                  {template.name}
                </span>
                : {describeAdjustment(template)}
              </li>
            ))}
          </ul>
        </SectionCard>
      ) : null}
      {impact.notAdjusted.length > 0 ? (
        <SectionCard
          title="Nicht angepasst, weil von Hand gepflegt"
          description="Hier steht das Kind weiter im Plan. Prüfen Sie, ob das so bleiben soll."
          headingLevel={5}
          titleSize="sm"
          titleClassName="font-medium text-gray-600"
          bare
          className="space-y-3"
        >
          {anyViaTarget ? (
            // Einmal über der Liste, auch bei nur einer solchen Zeile: eine
            // Regel, eine Stelle.
            <p className="mb-3 text-sm text-gray-600">
              Regeltermine für eine Klasse, einen Jahrgang oder eine Gruppe
              planen alle Kinder darin ein. Bei der nächsten Planung steht das
              Kind wieder im Plan.
              {canManage
                ? " Verbinden Sie den Regeltermin mit dem Angebot."
                : null}
            </p>
          ) : null}
          <ResponsiveActionList>
            {impact.notAdjusted.map((template) => (
              <ManualTemplateEntry
                key={manualTemplateKey(template)}
                template={template}
                removed={removed.has(manualTemplateKey(template))}
                canRemove={
                  canRemove && gapEndsAfter(template.until, firstRemovableDay)
                }
                canConnect={canManage}
                onRemove={setTarget}
                planLink={planLink}
                connectLink={connectLink(template.templateId)}
              />
            ))}
          </ResponsiveActionList>
          {actionMissing ? (
            <p className="mt-3 text-xs text-gray-500">
              Ändern kann nur, wer den Betreuungsplan bearbeiten darf.
            </p>
          ) : null}
        </SectionCard>
      ) : null}
      <ConfirmDeleteModal
        isOpen={target !== null}
        title="Aus dem Regeltermin entfernen?"
        description={
          target ? (
            <p>
              Das Kind wird{" "}
              {formatRemovalSpan(impact.effectiveFrom, target.until)}{" "}
              {formatWeekdayAdverbs(target.weekdays)} aus{" "}
              <span className="font-medium text-gray-900">„{target.name}“</span>{" "}
              entfernt. Das gilt auch für schon geplante Termine.
              {target.until
                ? ` Ab ${formatDate(target.until)} passt eine andere Buchung wieder. Dann bleibt das Kind eingeplant.`
                : null}{" "}
              Andere Tage und vergangene Termine bleiben.
            </p>
          ) : null
        }
        gate={{ mode: "twoStep", firstStepLabel: "Entfernen" }}
        confirmLabel="Jetzt entfernen"
        loadingLabel="Wird entfernt …"
        onConfirm={confirmRemoval}
        onClose={closeConfirm}
        loading={removing}
        error={errors.error}
      />
    </SectionCard>
  );
}

/**
 * One Regeltermin and its actions. Every action is the same kit button,
 * none the lesser one; its accessible name starts with the visible label
 * and adds the Regeltermin, so a screen reader's list tells the rows apart.
 */
function ManualTemplateEntry({
  template,
  removed,
  canRemove,
  canConnect,
  onRemove,
  planLink,
  connectLink,
}: Readonly<{
  template: ManualRosterTemplate;
  removed: boolean;
  canRemove: boolean;
  canConnect: boolean;
  onRemove: (template: ManualRosterTemplate) => void;
  planLink: string;
  connectLink: string;
}>) {
  const about = `„${template.name}“`;
  return (
    <li className="flex flex-col gap-2 py-3 text-sm text-gray-700 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center">
      <div className="min-w-0">
        <p>
          <span className="font-medium text-gray-900">{template.name}</span>,{" "}
          {formatWeekdayAdverbs(template.weekdays)}
          {template.until
            ? ` bis ${formatDate(lastGapDay(template.until))}`
            : null}
          : {removed ? "Kind entfernt." : "Kind bleibt eingeplant."}
        </p>
      </div>
      <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row min-[480px]:items-center sm:justify-end">
        {removed ? (
          <span className="flex">
            <StatusBadge tone="green" label="Entfernt" />
          </span>
        ) : template.viaTarget ? (
          canConnect ? (
            <ButtonLink
              href={connectLink}
              variant="surface"
              size="compact"
              aria-label={`Mit Angebot verbinden: ${about}`}
              tooltip="Öffnet den Betreuungsplan, um den Regeltermin mit dem Angebot zu verbinden."
            >
              Mit Angebot verbinden
            </ButtonLink>
          ) : null
        ) : canRemove ? (
          <Button
            type="button"
            variant="surface"
            size="compact"
            aria-label={`Dort auch entfernen: ${about}`}
            tooltip="Nimmt das Kind aus diesem Regeltermin, moto fragt vorher nach."
            onClick={() => onRemove(template)}
          >
            Dort auch entfernen
          </Button>
        ) : null}
        <ButtonLink
          href={planLink}
          variant="surface"
          size="compact"
          aria-label={`Im Betreuungsplan ansehen: ${about}`}
          tooltip="Öffnet die Regeltermine im Betreuungsplan ab dem Stichtag."
        >
          Im Betreuungsplan ansehen
        </ButtonLink>
      </div>
    </li>
  );
}

/**
 * Wochentage, an denen eine von Hand eingetragene Abholzeit die gebuchte
 * überdeckt. Ein Klick übernimmt für alle die gebuchte Zeit; der Server
 * löscht dafür die eingetragene Zeit des Wochentags.
 */
function PickupOverrides({
  overrides,
  effectiveFrom,
  studentId,
  done,
  onReset,
}: Readonly<{
  overrides: readonly PickupOverride[];
  effectiveFrom: string;
  studentId?: string;
  done: ReadonlySet<number>;
  onReset: (weekday: number) => void;
}>) {
  const { data: session } = useSession();
  const canReset =
    studentId !== undefined && hasPermission(session, "users:update");
  const [resetting, setResetting] = useState(false);
  const errors = useApiErrorDisplay();
  const toast = useToast();
  const open = overrides.filter((override) => !done.has(override.weekday));
  // Eine eingetragene Zeit hat kein Datum: weg ist sie ab heute. Gilt sie
  // bis zum Stichtag noch, bleibt sie bis dahin stehen.
  const resettable = open.filter((override) => override.resetNow);
  const later = open.filter((override) => !override.resetNow);
  // Liegt der Stichtag in der Zukunft, gilt die gebuchte Zeit erst ab dann:
  // das Datum steht dabei, vor und nach dem Übernehmen.
  const today = useBerlinToday();
  const startsLater = (override: PickupOverride) => override.date > today;

  const takeBookedTimes = async () => {
    if (studentId === undefined) return;
    setResetting(true);
    try {
      for (const override of resettable) {
        await resetStudentPickupToOffering(
          studentId,
          override.weekday,
          override.date,
        );
        onReset(override.weekday);
      }
      // Die Meldung nennt denselben Stichtag wie die Freigabe darüber.
      toast.success(
        effectiveFrom > today
          ? `Ab ${formatDate(effectiveFrom)} gelten die gebuchten Abholzeiten.`
          : "Jetzt gelten die gebuchten Abholzeiten.",
      );
    } catch (err) {
      logger.warn("care_plan_impact_pickup_reset_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await errors.show(err, { object: "die Abholzeit" });
    } finally {
      setResetting(false);
    }
  };

  return (
    <SectionCard
      title="Abholzeit weicht von der Buchung ab"
      description={
        open.length > 0
          ? "An diesen Tagen gilt noch die eingetragene Zeit."
          : undefined
      }
      headingLevel={5}
      titleSize="sm"
      titleClassName="font-medium text-gray-600"
      bare
      className="space-y-3"
      action={
        canReset && resettable.length > 0 ? (
          <Button
            type="button"
            variant="surface"
            size="compact"
            tooltip="Ersetzt die eingetragenen Abholzeiten sofort durch die gebuchten Zeiten."
            disabled={resetting}
            onClick={() => void takeBookedTimes()}
          >
            {resetting ? "Wird übernommen …" : "Gebuchte Zeiten übernehmen"}
          </Button>
        ) : undefined
      }
    >
      <ul className="divide-y divide-gray-100 text-sm text-gray-700">
        {overrides.map((override) => (
          <li
            key={override.weekday}
            className="flex flex-wrap items-center justify-between gap-2 py-2 first:pt-0 last:pb-0"
          >
            <span>
              <span className="font-medium text-gray-900">
                {capitalize(formatWeekdayAdverbs([override.weekday]))}
              </span>
              {pickupOverrideText(
                override,
                done.has(override.weekday),
                startsLater(override),
              )}
            </span>
            {done.has(override.weekday) ? (
              <StatusBadge tone="green" label="Übernommen" />
            ) : null}
          </li>
        ))}
      </ul>
      {later.length > 0 ? (
        <p className="mt-3 text-sm text-gray-600">
          {capitalize(
            formatWeekdayAdverbs(later.map((override) => override.weekday)),
          )}{" "}
          gilt die eingetragene Zeit bis zum genannten Tag weiter. Ab diesem Tag
          setzen Sie sie unter „Betreuungszeiten“ zurück.
        </p>
      ) : null}
      {studentId !== undefined && !canReset && resettable.length > 0 ? (
        <p className="mt-3 text-xs text-gray-500">
          Ändern kann nur, wer Kinder bearbeiten darf.
        </p>
      ) : null}
    </SectionCard>
  );
}

function pickupOverrideText(
  override: PickupOverride,
  done: boolean,
  startsLater: boolean,
): string {
  const from = startsLater ? `ab ${formatDate(override.date)} ` : "";
  if (done) {
    return startsLater
      ? `: ${from}${override.bookedTime} Uhr wie gebucht.`
      : `: jetzt ${override.bookedTime} Uhr wie gebucht.`;
  }
  return `: eingetragen ${override.storedTime} Uhr, ${from}gebucht ${override.bookedTime} Uhr.`;
}

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/** Eine Zeile je Regeltermin und Ende der Lücke (#5497). */
function manualTemplateKey(template: ManualRosterTemplate): string {
  return `${template.templateId}-${template.until ?? "open"}`;
}

// The command rejects today and clamps an earlier effective date to tomorrow.
// Do not offer a bounded removal that has already ended at that boundary.
function earliestRemovalDay(effectiveFrom: string, today: string): string {
  const tomorrow = parseISODate(today);
  tomorrow.setDate(tomorrow.getDate() + 1);
  const earliest = toISODate(tomorrow);
  return effectiveFrom > earliest ? effectiveFrom : earliest;
}

function gapEndsAfter(
  until: string | null,
  firstRemovableDay: string,
): boolean {
  return until === null || until > firstRemovableDay;
}
