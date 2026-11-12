"use client";

import { FramedList } from "~/components/ui/framed-list";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { AlertTriangle, BellRing, CheckCircle2, RefreshCw } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { SegmentedControl } from "~/components/ui/segmented-control";
import type { SegmentedControlItem } from "~/components/ui/segmented-control";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { formatDate } from "~/lib/date-helpers";
import {
  CHECK_ADDRESS_AT_GUARDIAN,
  correctedAddressCopy,
  mailStatusCopy,
  STOP_SHARED_NOTE,
  TELL_THE_OGS_ABOUT_THE_ADDRESS,
} from "~/lib/email-delivery-status";
import {
  fetchLetterStatus,
  remindUnanswered,
  resendFailedEmails,
  withApiBase,
} from "~/lib/parent-announcements-api";
import type {
  AnnouncementEmailAudience,
  LetterChild,
  LetterRecipient,
  LetterStatus,
} from "~/lib/parent-announcements-api";
import { SpinnerIcon } from "~/components/ui/icons";

/**
 * The two channels of an Elternbrief, side by side (#2384).
 *
 * They are shown as separate columns on purpose: "die E-Mail ist raus" and
 * "jemand hat in moto bestätigt" are different facts. Merging them into one
 * status is what would let a school believe a letter arrived when it did not.
 *
 * The e-mail column says "Zugestellt" only when the mail provider reported it
 * (#5934). A mail the server merely took stays "Versendet"; plain SMTP and the
 * fallback mailbox report nothing, so their mails never get further.
 */
const EMAIL_LABELS: Record<
  LetterRecipient["email_status"],
  { label: string; tone: StatusBadgeTone; title?: string }
> = {
  sent: {
    label: "Versendet",
    tone: "green",
    title:
      "Die E-Mail ist raus. Ob sie im Postfach angekommen ist, weiß moto nicht.",
  },
  pending: { label: "Wird gesendet", tone: "blue" },
  failed: { label: "Fehlgeschlagen", tone: "red" },
  cancelled: { label: "Abgebrochen", tone: "gray" },
  not_sent: { label: "Nicht versendet", tone: "gray" },
};

/**
 * Why nothing was sent. Shown NEXT TO the e-mail status, never instead of it —
 * "kein Portalzugang" is a data gap, not a delivery failure, and a guardian can
 * be reachable by mail while still having no portal access.
 */
const REACHABILITY_LABELS: Record<
  LetterRecipient["reachability"],
  { label: string; tone: StatusBadgeTone; title?: string } | null
> = {
  ok: null,
  no_email: { label: "Keine E-Mail-Adresse", tone: "warning" },
  no_portal: { label: "Kein Portalzugang", tone: "warning" },
  excluded: {
    label: "Abbestellt",
    tone: "gray",
    title: "Diese Person hat E-Mail-Benachrichtigungen abbestellt.",
  },
};

type ChildFilter = "all" | "open";

const CHILD_FILTERS: ReadonlyArray<SegmentedControlItem<ChildFilter>> = [
  { value: "all", label: "Alle Kinder" },
  { value: "open", label: "Nur offene" },
];

const logger = createLogger({ component: "LetterStatusPanel" });

function fullName(first: string, last: string): string {
  return `${first} ${last}`.trim() || "Unbekannt";
}

export function LetterStatusPanel({
  announcementId,
  canAct,
  emailAudience,
  apiBase,
}: {
  readonly announcementId: string;
  /** The routes of the portal that shows the letter; default the OGS's. */
  readonly apiBase?: string;
  /** Reminders and resends are only offered while the letter is live. */
  readonly canAct: boolean;
  readonly emailAudience: AnnouncementEmailAudience;
}) {
  const [status, setStatus] = useState<LetterStatus | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const latestRequest = useRef(0);
  const loadError = useApiLoadError();
  const showLoadError = loadError.show;
  const clearLoadError = loadError.clear;
  const { show: showError } = useApiErrorDisplay();
  const { success: toastSuccess } = useToast();
  const [busy, setBusy] = useState<"remind" | "resend" | null>(null);
  const [childFilter, setChildFilter] = useState<ChildFilter>("all");
  // „Wiederholen“ ruft Laden bzw. die Aktion mit dem dann aktuellen Stand auf.
  const latestLoadRef = useRef<() => void>(() => undefined);
  const latestActionRef = useRef<(action: "remind" | "resend") => void>(
    () => undefined,
  );

  // Rejects on failure: after an action the caller shows it as a reload of a
  // saved change (#6652).
  const readStatus = useCallback(async () => {
    const request = ++latestRequest.current;
    try {
      const next = await withApiBase(
        fetchLetterStatus,
        announcementId,
        apiBase,
      );
      if (request !== latestRequest.current) return;
      setStatus(next);
      setLoadFailed(false);
      clearLoadError();
    } catch (error) {
      if (request !== latestRequest.current) return;
      throw error;
    }
  }, [announcementId, apiBase, clearLoadError]);
  // Never rejects: a failed load shows its error in place.
  const load = useCallback(async () => {
    const request = latestRequest.current + 1;
    try {
      await readStatus();
    } catch (error) {
      if (request !== latestRequest.current) return;
      logger.error("letter_status_load_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      setLoadFailed(true);
      void showLoadError(error, {
        object: "die Übersicht zum Elternbrief",
        retry: () => latestLoadRef.current(),
      });
    }
  }, [readStatus, showLoadError]);
  const afterAction = useRefreshAfterWrite(
    "die Anzeige der Übersicht zum Elternbrief",
    readStatus,
  );
  useLayoutEffect(() => {
    latestLoadRef.current = () => void load();
  });

  const invalidateRequest = useCallback(() => {
    latestRequest.current++;
  }, []);
  useEffect(() => {
    void load();
    return invalidateRequest;
  }, [load, invalidateRequest]);

  const children = useMemo(() => {
    const all = status?.children ?? [];
    // "Nur offene" means what the reminder button reaches: confirmable and not
    // yet confirmed. A child nobody can confirm for is not "offen".
    return childFilter === "open"
      ? all.filter((c) => c.can_confirm && !c.fulfilled)
      : all;
  }, [status, childFilter]);

  const runAction = async (action: "remind" | "resend") => {
    if (busy !== null) return;
    setBusy(action);
    let count: number;
    try {
      count =
        action === "remind"
          ? await withApiBase(remindUnanswered, announcementId, apiBase)
          : await withApiBase(resendFailedEmails, announcementId, apiBase);
    } catch (error) {
      logger.error("letter_status_action_failed", {
        action,
        error: error instanceof Error ? error.message : String(error),
      });
      void showError(error, {
        object:
          action === "remind"
            ? "das Senden der Erinnerung"
            : "das erneute Senden der E-Mails",
        retry: () => latestActionRef.current(action),
      });
      setBusy(null);
      return;
    }
    toastSuccess(
      action === "remind"
        ? count === 0
          ? "Alle Kinder sind bestätigt. Es wurde niemand erinnert."
          : `${count} ${count === 1 ? "Bezugsperson ist" : "Bezugspersonen sind"} erinnert.`
        : count === 0
          ? "Es gab keine fehlgeschlagenen E-Mails."
          : `${count} ${count === 1 ? "E-Mail wird" : "E-Mails werden"} erneut gesendet.`,
    );
    await afterAction.refresh();
    setBusy(null);
  };
  useLayoutEffect(() => {
    latestActionRef.current = (action) => void runAction(action);
  });

  if (loadFailed && !status) {
    // Bis der Katalogtext da ist, bleibt die Ladezeile stehen.
    return loadError.error ? (
      <LoadErrorAlert error={loadError.error} />
    ) : (
      <div
        aria-busy="true"
        className="flex items-center gap-2 text-sm text-gray-500"
      >
        <SpinnerIcon className="h-4 w-4" />
        Status wird geladen …
      </div>
    );
  }
  if (!status) {
    return (
      <div
        aria-busy="true"
        className="flex items-center gap-2 text-sm text-gray-500"
      >
        <SpinnerIcon className="h-4 w-4" />
        Status wird geladen …
      </div>
    );
  }

  const s = status.summary;
  const hasFailures = s.emails_failed > 0;
  // A marked address stays out of the resend, or the button would do nothing.
  const resendable = s.emails_resendable ?? s.emails_failed;

  return (
    <div className="space-y-4">
      {/* The number a school acts on is "wie viele Kinder fehlen noch", so it
          leads. Everything else is supporting detail. */}
      <SectionCard
        title="Bestätigungen"
        icon={CheckCircle2}
        bodyClassName="compact:mt-3 mt-4 space-y-3"
      >
        <div className="flex flex-wrap items-baseline gap-x-2">
          <span className="text-3xl font-semibold text-gray-900 tabular-nums">
            {s.children_fulfilled}
          </span>
          <span className="text-lg text-gray-500 tabular-nums">
            / {s.children_confirmable}
          </span>
          <span className="text-sm text-gray-600">Kindern bestätigt</span>
        </div>
        <p className="mt-1 text-sm text-gray-600">
          {s.children_open === 0
            ? "Alle Kinder sind bestätigt, bei denen das möglich ist."
            : `${s.children_open} ${s.children_open === 1 ? "Kind wartet" : "Kinder warten"} noch auf eine Bestätigung.`}
        </p>
        {/* The denominator is the CONFIRMABLE children, not the reach. Without
            this line a school-wide letter to 116 children reads "0 / 6" and
            looks nearly done, when in truth 110 children have nobody who could
            ever confirm. A reminder does not fix those — portal access does. */}
        {s.children_without_portal > 0 && (
          <p className="mt-1 text-sm text-gray-600">
            Der Brief erreicht {s.children_total}{" "}
            {s.children_total === 1 ? "Kind" : "Kinder"}. Für{" "}
            {s.children_without_portal}{" "}
            {s.children_without_portal === 1 ? "Kind" : "Kinder"} ist keine
            Bestätigung möglich, weil keine Bezugsperson einen
            Elternportal-Zugang hat.
          </p>
        )}

        <div className="mt-4">
          <DataGrid columns={4}>
            <DataField label="E-Mails versendet">
              <span className="tabular-nums">{s.emails_sent}</span>
            </DataField>
            {(s.emails_delivered ?? 0) > 0 && (
              <DataField label="Davon zugestellt">
                <span className="tabular-nums">{s.emails_delivered}</span>
              </DataField>
            )}
            {(s.emails_delayed ?? 0) > 0 && (
              <DataField label="Verzögert">
                <span className="tabular-nums">{s.emails_delayed}</span>
              </DataField>
            )}
            {(s.emails_address_check ?? 0) > 0 && (
              <DataField label="Adresse prüfen">
                <span className="tabular-nums">{s.emails_address_check}</span>
              </DataField>
            )}
            {s.emails_pending > 0 && (
              <DataField label="Wird gesendet">
                <span className="tabular-nums">{s.emails_pending}</span>
              </DataField>
            )}
            {hasFailures && (
              <DataField label="Fehlgeschlagen">
                <span className="tabular-nums">{s.emails_failed}</span>
              </DataField>
            )}
            {s.without_email > 0 && (
              <DataField label="Ohne E-Mail-Adresse">
                <span className="tabular-nums">{s.without_email}</span>
              </DataField>
            )}
            {s.without_portal > 0 && (
              <DataField label="Ohne Portalzugang">
                <span className="tabular-nums">{s.without_portal}</span>
              </DataField>
            )}
          </DataGrid>
        </div>

        {(s.without_email > 0 || s.without_portal > 0) && (
          <p className="mt-3 flex items-start gap-2 text-xs text-gray-600">
            <AlertTriangle
              className="text-moto-amber mt-0.5 h-3.5 w-3.5 shrink-0"
              aria-hidden
            />
            <span>
              {emailAudience === "all_contacts" ? (
                s.without_email > 0 ? (
                  "Bei manchen Bezugspersonen fehlt die E-Mail-Adresse. Bezugspersonen ohne Portalzugang erhalten den Brief per E-Mail."
                ) : (
                  "Bezugspersonen ohne Portalzugang erhalten den Brief per E-Mail."
                )
              ) : (
                <>
                  {s.without_email > 0 && s.without_portal > 0
                    ? "Bei manchen Bezugspersonen fehlen E-Mail-Adresse oder Portalzugang."
                    : s.without_email > 0
                      ? "Bei manchen Bezugspersonen fehlt die E-Mail-Adresse."
                      : "Bei manchen Bezugspersonen fehlt der Portalzugang."}{" "}
                  Tragen Sie die fehlenden Daten bei der Bezugsperson ein.
                  Senden Sie den Brief danach noch einmal.
                </>
              )}
            </span>
          </p>
        )}
      </SectionCard>

      {((s.emails_address_check ?? 0) > 0 ||
        (s.emails_stopped ?? 0) > 0 ||
        (s.emails_provider_blocked ?? 0) > 0 ||
        (s.emails_delayed ?? 0) > 0) && (
        <Alert
          type="warning"
          message={[
            (s.emails_address_check ?? 0) > 0
              ? `Bei ${s.emails_address_check} ${s.emails_address_check === 1 ? "Adresse" : "Adressen"} geht keine Mail raus. ${apiBase === undefined ? CHECK_ADDRESS_AT_GUARDIAN : TELL_THE_OGS_ABOUT_THE_ADDRESS}`
              : null,
            (s.emails_stopped ?? 0) > 0
              ? `An ${s.emails_stopped} ${s.emails_stopped === 1 ? "Person" : "Personen"} gehen keine Mails, weil die Schule sie gestoppt hat.`
              : null,
            (s.emails_provider_blocked ?? 0) > 0
              ? `Bei ${s.emails_provider_blocked} ${s.emails_provider_blocked === 1 ? "Adresse" : "Adressen"} geht eine neue Mail gerade nicht raus. Den Grund sehen Sie beim Empfänger.`
              : null,
            (s.emails_delayed ?? 0) > 0
              ? "Verzögerte Mails kommen später an. Sie werden nicht erneut gesendet."
              : null,
          ]
            .filter(Boolean)
            .join(" ")}
        />
      )}

      {canAct && (
        <div className="flex flex-wrap gap-2">
          {/* Two problems, two buttons: "hat nicht bestätigt" and "die Mail kam
              nie an" need different fixes. */}
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => void runAction("remind")}
            disabled={busy !== null || s.children_open === 0}
            disabledReason={
              s.children_open === 0
                ? "Für jedes Kind liegt schon eine Bestätigung vor."
                : undefined
            }
            tooltip="Schickt sofort eine E-Mail an Eltern, die noch nicht bestätigt haben."
            className="gap-1.5"
          >
            <BellRing className="h-4 w-4" aria-hidden />
            {busy === "remind" ? "Wird gesendet …" : "Offene erinnern"}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => void runAction("resend")}
            disabled={busy !== null || resendable === 0}
            disabledReason={
              resendable === 0
                ? "Es gibt keine fehlgeschlagene E-Mail, die moto noch einmal senden kann."
                : undefined
            }
            tooltip="Schickt die E-Mails, die nicht ankamen, sofort noch einmal."
            className="gap-1.5"
          >
            <RefreshCw className="h-4 w-4" aria-hidden />
            {busy === "resend"
              ? "Wird gesendet …"
              : "Fehlgeschlagene erneut senden"}
          </Button>
        </div>
      )}

      {/* Ein Neuladen ist gescheitert: der alte Stand bleibt sichtbar, der
          Fehler steht darüber. Nach einer Aktion sagt er, dass sie
          gespeichert ist (#6652). */}
      <LoadErrorAlert error={loadFailed ? loadError.error : null} />
      <LoadErrorAlert error={afterAction.error} className="mb-4" />

      <section>
        <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
          <h3 className="text-sm font-semibold text-gray-900">Kinder</h3>
          <SegmentedControl
            items={CHILD_FILTERS}
            value={childFilter}
            onChange={setChildFilter}
            ariaLabel="Kinder filtern"
          />
        </div>
        {children.length === 0 ? (
          <p className="text-sm text-gray-500">
            {childFilter === "open"
              ? "Alle Kinder sind bestätigt, bei denen das möglich ist."
              : "Dieser Brief erreicht derzeit kein Kind."}
          </p>
        ) : (
          <FramedList>
            {children.map((child) => (
              <ChildRow key={child.student_id} child={child} />
            ))}
          </FramedList>
        )}
      </section>

      <section>
        <h3 className="mb-2 text-sm font-semibold text-gray-900">Empfänger</h3>
        {status.recipients.length === 0 ? (
          <p className="text-sm text-gray-500">Noch keine Empfänger erfasst.</p>
        ) : (
          <FramedList>
            {status.recipients.map((r, index) => (
              <RecipientRow
                key={`${r.email ?? "ohne"}-${index}`}
                recipient={r}
              />
            ))}
          </FramedList>
        )}
      </section>
    </div>
  );
}

function ChildRow({ child }: { readonly child: LetterChild }) {
  return (
    <li className="flex flex-wrap items-center justify-between gap-2 px-4 py-3">
      <div className="min-w-0">
        <p className="truncate text-sm text-gray-900">
          {fullName(child.first_name, child.last_name)}
          {child.school_class && (
            <span className="ml-2 text-xs text-gray-500">
              {child.school_class}
            </span>
          )}
        </p>
        {child.fulfilled && child.acknowledged_by && (
          <p className="text-xs text-gray-500">
            Bestätigt von {child.acknowledged_by}
            {child.acknowledged_at &&
              ` am ${formatDate(child.acknowledged_at)}`}
          </p>
        )}
      </div>
      {child.fulfilled ? (
        <StatusBadge label="Bestätigt" tone="green" />
      ) : child.can_confirm ? (
        <StatusBadge label="Offen" tone="warning" />
      ) : (
        <StatusBadge
          label="Keine Bestätigung möglich"
          tone="gray"
          title="Keine Bezugsperson dieses Kindes hat einen Elternportal-Zugang. Eine Erinnerung ändert daran nichts."
        />
      )}
    </li>
  );
}

/**
 * The result belongs to the address the mail went to. A mark belongs to the
 * current address, so it never reads as the result of the old one. After a
 * correction the row also says what the current address allows.
 */
function recipientMailCopy(
  recipient: LetterRecipient,
  addressChanged: boolean,
) {
  if (addressChanged) {
    const resendReaches =
      recipient.email_status === "failed" &&
      !recipient.address_mark &&
      !recipient.stopped &&
      recipient.provider_block !== "active" &&
      recipient.provider_block !== "unreadable";
    const corrected = resendReaches
      ? correctedAddressCopy(recipient.mail_outcome)
      : null;
    if (corrected) return corrected;
    // The current address refused an earlier mail of this letter.
    if (recipient.provider_block === "active") {
      return mailStatusCopy("provider_blocked", "", false, "active");
    }
  }
  return mailStatusCopy(
    recipient.mail_outcome,
    addressChanged ? "" : recipient.address_mark,
    recipient.stopped,
    recipient.provider_block,
  );
}

function RecipientRow({ recipient }: { readonly recipient: LetterRecipient }) {
  // Fall back rather than crash: the backend may add a status this build does
  // not know yet, and an unknown value must not take the whole page down.
  const mailedTo = recipient.mailed_to ?? recipient.email;
  const addressChanged = !!recipient.email && mailedTo !== recipient.email;
  const mailCopy = recipientMailCopy(recipient, addressChanged);
  const email = mailCopy
    ? { label: mailCopy.label, tone: mailCopy.tone, title: undefined }
    : (EMAIL_LABELS[recipient.email_status] ?? {
        label: recipient.email_status,
        tone: "gray" as StatusBadgeTone,
      });
  // The note says it in plain words; the raw server text stays out then.
  const note =
    recipient.mail_outcome === "accepted" ? null : mailCopy?.explanation;
  const portal = REACHABILITY_LABELS[recipient.reachability] ?? null;
  return (
    <li className="flex flex-wrap items-center justify-between gap-2 px-4 py-3">
      <div className="min-w-0">
        <p className="truncate text-sm text-gray-900">
          {fullName(recipient.first_name, recipient.last_name)}
        </p>
        {mailedTo && (
          <p className="truncate text-xs text-gray-500">{mailedTo}</p>
        )}
        {addressChanged && (
          <p className="truncate text-xs text-gray-500">
            Aktuelle Adresse: {recipient.email}
            {recipient.address_mark ? " (Adresse prüfen)" : ""}
          </p>
        )}
        {note && <p className="text-xs text-gray-600">{note}</p>}
        {recipient.stopped && recipient.stop_shared && (
          <p className="text-xs text-gray-600">{STOP_SHARED_NOTE}</p>
        )}
        {recipient.last_error && !mailCopy && (
          <p
            className="truncate text-xs text-gray-500"
            title={recipient.last_error}
          >
            {recipient.last_error}
          </p>
        )}
      </div>
      <div className="flex flex-wrap items-center gap-1.5">
        <StatusBadge
          label={email.label}
          tone={email.tone}
          title={email.title}
        />
        {portal && (
          <StatusBadge
            label={portal.label}
            tone={portal.tone}
            title={portal.title}
          />
        )}
      </div>
    </li>
  );
}
