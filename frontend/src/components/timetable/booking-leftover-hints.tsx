"use client";

import {
  type ReactNode,
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
} from "react";
import { ChevronDown, ChevronUp, TriangleAlert } from "lucide-react";

import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import {
  type BookingLeftover,
  bookingLeftoverKey,
} from "~/lib/booking-leftover";
import { formatDate } from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import {
  formatWeekdayAdverbs,
  removeStudentTemplateDays,
} from "~/lib/roster-impact";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { timetableKeys } from "~/lib/swr/portal-keys";
import { timetableService } from "~/lib/timetable-api";

const logger = createLogger({ component: "BookingLeftoverHints" });

/** Open rows shown before „N weitere anzeigen“, as in the gap hints. */
const INITIAL_LEFTOVER_COUNT = 5;

/**
 * Children a hand-maintained Regeltermin still plans after their booking of
 * those weekdays ended (#5102). The save response of the booking change
 * names them once; this list stays until the school removes the child or
 * keeps it. One neutral card with a row per child and Regeltermin and two
 * equal actions, like the offering gaps (#5181). Kept rows wait folded and
 * can come back. Readers without planning rights see the open rows only. A
 * failed check shows the load error with a retry, never an empty list.
 */
export function BookingLeftoverHints({
  periodId,
  canManage,
  templatesVersion,
}: Readonly<{
  periodId: string | undefined;
  canManage: boolean;
  templatesVersion: unknown;
}>) {
  const toast = useToast();
  const { show: showActionError } = useApiErrorDisplay();
  const removalErrors = useApiFormError();
  const tenantMutate = useTenantMutate();
  const [busyKey, setBusyKey] = useState<string | null>(null);
  const [expanded, setExpanded] = useState(false);
  const [removing, setRemoving] = useState<BookingLeftover | null>(null);
  const [removalBusy, setRemovalBusy] = useState(false);
  const openListId = useId();
  const key = periodId ? timetableKeys.bookingLeftovers(periodId) : null;
  const { data, error } = useSWRAuth(
    key,
    () => timetableService.getBookingLeftovers(periodId!),
    { keepPreviousData: false },
  );
  const reload = useCallback(
    () => (key ? tenantMutate(key) : undefined),
    [key, tenantMutate],
  );
  const loadError = useSwrLoadError(
    error,
    "die Prüfung der Kinder ohne passende Buchung",
    reload,
  );

  const seenVersion = useRef(templatesVersion);
  useEffect(() => {
    if (seenVersion.current === templatesVersion) return;
    const first = seenVersion.current === undefined;
    seenVersion.current = templatesVersion;
    // The first template load runs next to the first hint load already.
    if (!first && templatesVersion !== undefined) void reload();
  }, [templatesVersion, reload]);

  const decide = useCallback(
    async (leftover: BookingLeftover, kept: boolean) => {
      setBusyKey(bookingLeftoverKey(leftover));
      try {
        await timetableService.setBookingLeftoverKept(leftover, kept);
        toast.success(
          kept
            ? `${leftover.studentName} bleibt in „${leftover.templateName}“.`
            : `Der Hinweis zu ${leftover.studentName} wird wieder gezeigt.`,
        );
        await reload();
      } catch (err) {
        logger.error("booking_leftover_decision_failed", {
          template_id: leftover.templateId,
          kept,
          error: err instanceof Error ? err.message : String(err),
        });
        void showActionError(err, {
          object: kept
            ? "das Behalten im Regeltermin"
            : "das Einblenden des Hinweises",
        });
      } finally {
        setBusyKey(null);
      }
    },
    [reload, showActionError, toast],
  );

  const closeRemoval = () => {
    if (removalBusy) return;
    setRemoving(null);
    removalErrors.clear();
  };

  const confirmRemoval = async () => {
    if (!removing) return;
    setRemovalBusy(true);
    removalErrors.clear();
    try {
      await removeStudentTemplateDays({
        templateId: removing.templateId,
        studentId: removing.studentId,
        weekdays: removing.weekdays,
        effectiveFrom: removing.from,
      });
      toast.success(
        `${removing.studentName} ist aus „${removing.templateName}“ entfernt.`,
      );
      setRemoving(null);
      await reload();
    } catch (err) {
      logger.warn("booking_leftover_remove_failed", {
        template_id: removing.templateId,
        error: err instanceof Error ? err.message : String(err),
      });
      await removalErrors.show(err, {
        object: "das Entfernen aus dem Regeltermin",
      });
    } finally {
      setRemovalBusy(false);
    }
  };

  const leftovers = data ?? [];
  const open = leftovers.filter((leftover) => !leftover.kept);
  const kept = canManage ? leftovers.filter((leftover) => leftover.kept) : [];
  if (error) {
    return <LoadErrorAlert error={loadError} actionLayout="responsive" />;
  }
  if (open.length === 0 && kept.length === 0) return null;

  const hiddenCount = Math.max(0, open.length - INITIAL_LEFTOVER_COUNT);
  const visible = expanded ? open : open.slice(0, INITIAL_LEFTOVER_COUNT);

  return (
    <div className="flex flex-col gap-3" data-testid="booking-leftover-hints">
      {open.length > 0 ? (
        <SectionCard
          title="Ohne passende Buchung eingeplant"
          titleBadge={
            <StatusBadge tone="orange" label={`${open.length} offen`} />
          }
          description="Diese Kinder stehen von Hand in einem Regeltermin. Ihre Buchung für diese Tage ist weggefallen."
          leading={
            <TriangleAlert
              className="text-moto-orange-strong mt-0.5 size-5 shrink-0"
              aria-hidden="true"
            />
          }
        >
          <ResponsiveActionList id={openListId}>
            {visible.map((leftover) => (
              <BookingLeftoverRow
                key={bookingLeftoverKey(leftover)}
                leftover={leftover}
              >
                {canManage ? (
                  <OpenLeftoverActions
                    leftover={leftover}
                    busy={busyKey === bookingLeftoverKey(leftover)}
                    onRemove={setRemoving}
                    onKeep={(item) => void decide(item, true)}
                  />
                ) : null}
              </BookingLeftoverRow>
            ))}
          </ResponsiveActionList>
          {hiddenCount > 0 ? (
            <Button
              type="button"
              variant="ghost"
              size="md"
              className="mt-2 min-h-11 w-full px-3 text-sm font-medium"
              aria-expanded={expanded}
              aria-controls={openListId}
              onClick={() => setExpanded((current) => !current)}
            >
              {expanded ? (
                <>
                  Weniger anzeigen
                  <ChevronUp className="size-4" aria-hidden="true" />
                </>
              ) : (
                <>
                  {hiddenCount} weitere anzeigen
                  <ChevronDown className="size-4" aria-hidden="true" />
                </>
              )}
            </Button>
          ) : null}
          {canManage ? null : (
            <p className="mt-3 text-xs text-gray-500">
              Ändern kann nur, wer den Betreuungsplan bearbeiten darf.
            </p>
          )}
        </SectionCard>
      ) : null}
      {kept.length > 0 ? (
        <SectionCard
          title={`Bleibt ohne Buchung eingeplant (${kept.length})`}
          description="Sie haben entschieden: Diese Kinder bleiben im Regeltermin."
          collapsible
          defaultCollapsed
          headingLevel={3}
        >
          <ResponsiveActionList>
            {kept.map((leftover) => (
              <BookingLeftoverRow
                key={bookingLeftoverKey(leftover)}
                leftover={leftover}
              >
                <Button
                  type="button"
                  variant="surface"
                  size="compact"
                  aria-label={`Hinweis wieder zeigen: ${leftover.studentName} in „${leftover.templateName}“`}
                  onClick={() => void decide(leftover, false)}
                  disabled={busyKey === bookingLeftoverKey(leftover)}
                >
                  Hinweis wieder zeigen
                </Button>
              </BookingLeftoverRow>
            ))}
          </ResponsiveActionList>
        </SectionCard>
      ) : null}
      <ConfirmDeleteModal
        isOpen={removing !== null}
        title="Aus dem Regeltermin entfernen?"
        description={
          removing ? (
            <p>
              {removing.studentName} wird ab {formatDate(removing.from)}{" "}
              {formatWeekdayAdverbs(removing.weekdays)} aus{" "}
              <span className="font-medium text-gray-900">
                „{removing.templateName}“
              </span>{" "}
              entfernt. Das gilt auch für schon geplante Termine. Andere Tage
              und vergangene Termine bleiben.
            </p>
          ) : null
        }
        gate={{ mode: "twoStep", firstStepLabel: "Entfernen" }}
        confirmLabel="Jetzt entfernen"
        loadingLabel="Wird entfernt …"
        onConfirm={confirmRemoval}
        onClose={closeRemoval}
        loading={removalBusy}
        error={removalErrors.error}
      />
    </div>
  );
}

/**
 * The two equal decisions of an open row, the same kit buttons as the save
 * response's list (care-plan-impact.tsx); the accessible names add the child
 * and the Regeltermin so a screen reader tells the rows apart.
 */
function OpenLeftoverActions({
  leftover,
  busy,
  onRemove,
  onKeep,
}: Readonly<{
  leftover: BookingLeftover;
  busy: boolean;
  onRemove: (leftover: BookingLeftover) => void;
  onKeep: (leftover: BookingLeftover) => void;
}>) {
  const about = `${leftover.studentName} in „${leftover.templateName}“`;
  return (
    <>
      <Button
        type="button"
        variant="surface"
        size="compact"
        aria-label={`Entfernen: ${about}`}
        onClick={() => onRemove(leftover)}
        disabled={busy}
      >
        Entfernen
      </Button>
      <Button
        type="button"
        variant="surface"
        size="compact"
        aria-label={`Bleibt so: ${about}`}
        onClick={() => onKeep(leftover)}
        disabled={busy}
      >
        Bleibt so
      </Button>
    </>
  );
}

/** One child in one Regeltermin: who, where, which days, since when. */
function BookingLeftoverRow({
  leftover,
  children,
}: Readonly<{
  leftover: BookingLeftover;
  children?: ReactNode;
}>) {
  return (
    <li className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center">
      <div className="min-w-0">
        <p className="text-sm font-semibold text-gray-900">
          {leftover.studentName}
        </p>
        <p className="mt-0.5 text-sm text-gray-600">
          „{leftover.templateName}“ · {formatWeekdayAdverbs(leftover.weekdays)}{" "}
          · ohne passende Buchung ab {formatDate(leftover.from)}
        </p>
      </div>
      {children ? (
        <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
          {children}
        </div>
      ) : null}
    </li>
  );
}
