"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { AlertTriangle, BellRing, CheckCircle2, RefreshCw } from "lucide-react";

import { Button } from "~/components/ui/button";
import { InfoCard } from "~/components/ui/info-card";
import { StatusBadge } from "~/components/ui/status-badge";
import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { SegmentedControl } from "~/components/ui/segmented-control";
import type { SegmentedControlItem } from "~/components/ui/segmented-control";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { formatDate } from "~/lib/date-helpers";
import {
  fetchLetterStatus,
  remindUnanswered,
  resendFailedEmails,
} from "~/lib/parent-announcements-api";
import type {
  LetterChild,
  LetterRecipient,
  LetterStatus,
} from "~/lib/parent-announcements-api";

/**
 * The two channels of an Elternbrief, side by side (#2384).
 *
 * They are shown as separate columns on purpose: "die E-Mail ist raus" and
 * "jemand hat in moto bestätigt" are different facts. Merging them into one
 * status is what would let a school believe a letter arrived when it did not.
 *
 * The e-mail column says "Versendet", never "Zugestellt" — the backend only
 * knows the mail was handed to the mail server. Real delivery confirmation
 * needs provider webhooks (#1937).
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
  no_email: { label: "Keine E-Mail-Adresse", tone: "orange" },
  no_portal: { label: "Kein Portalzugang", tone: "orange" },
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

function fullName(first: string, last: string): string {
  return `${first} ${last}`.trim() || "Unbekannt";
}

export function LetterStatusPanel({
  announcementId,
  canAct,
}: {
  readonly announcementId: string;
  /** Reminders and resends are only offered while the letter is live. */
  readonly canAct: boolean;
}) {
  const [status, setStatus] = useState<LetterStatus | null>(null);
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState<"remind" | "resend" | null>(null);
  const [childFilter, setChildFilter] = useState<ChildFilter>("all");

  const load = useCallback(async () => {
    try {
      setStatus(await fetchLetterStatus(announcementId));
      setLoadError("");
    } catch (error) {
      setLoadError(
        getApiErrorMessage(
          error,
          "laden",
          "Status",
          "Status konnte nicht geladen werden.",
        ),
      );
    }
  }, [announcementId]);

  useEffect(() => {
    void load();
  }, [load]);

  const children = useMemo(() => {
    const all = status?.children ?? [];
    // "Nur offene" means what the reminder button reaches: confirmable and not
    // yet confirmed. A child nobody can confirm for is not "offen".
    return childFilter === "open"
      ? all.filter((c) => c.can_confirm && !c.fulfilled)
      : all;
  }, [status, childFilter]);

  const runAction = async (action: "remind" | "resend") => {
    setBusy(action);
    setNotice("");
    try {
      const count =
        action === "remind"
          ? await remindUnanswered(announcementId)
          : await resendFailedEmails(announcementId);
      setNotice(
        action === "remind"
          ? count === 0
            ? "Alle Kinder sind bestätigt. Es wurde niemand erinnert."
            : `${count} Bezugspersonen erinnert.`
          : count === 0
            ? "Es gab keine fehlgeschlagenen E-Mails."
            : `${count} E-Mails werden erneut gesendet.`,
      );
      await load();
    } catch (error) {
      setLoadError(
        getApiErrorMessage(
          error,
          action === "remind" ? "senden" : "senden",
          action === "remind" ? "Erinnerung" : "E-Mail",
          "Aktion konnte nicht ausgeführt werden.",
        ),
      );
    } finally {
      setBusy(null);
    }
  };

  if (loadError && !status) {
    return (
      <div className="rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm text-gray-700">
        {loadError}
      </div>
    );
  }
  if (!status) {
    return <div className="text-sm text-gray-500">Status wird geladen …</div>;
  }

  const s = status.summary;
  const hasFailures = s.emails_failed > 0;

  return (
    <div className="space-y-4">
      {/* The number a school acts on is "wie viele Kinder fehlen noch", so it
          leads. Everything else is supporting detail. */}
      <InfoCard
        title="Bestätigungen"
        icon={<CheckCircle2 className="h-5 w-5" aria-hidden />}
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

        <dl className="mt-4 flex flex-wrap gap-x-6 gap-y-2 text-sm">
          <div>
            <dt className="text-xs text-gray-500">E-Mails versendet</dt>
            <dd className="text-gray-900 tabular-nums">{s.emails_sent}</dd>
          </div>
          {s.emails_pending > 0 && (
            <div>
              <dt className="text-xs text-gray-500">Wird gesendet</dt>
              <dd className="text-gray-900 tabular-nums">{s.emails_pending}</dd>
            </div>
          )}
          {hasFailures && (
            <div>
              <dt className="text-xs text-gray-500">Fehlgeschlagen</dt>
              <dd className="text-gray-900 tabular-nums">{s.emails_failed}</dd>
            </div>
          )}
          {s.without_email > 0 && (
            <div>
              <dt className="text-xs text-gray-500">Ohne E-Mail-Adresse</dt>
              <dd className="text-gray-900 tabular-nums">{s.without_email}</dd>
            </div>
          )}
          {s.without_portal > 0 && (
            <div>
              <dt className="text-xs text-gray-500">Ohne Portalzugang</dt>
              <dd className="text-gray-900 tabular-nums">{s.without_portal}</dd>
            </div>
          )}
        </dl>

        {(s.without_email > 0 || s.without_portal > 0) && (
          <p className="mt-3 flex items-start gap-2 text-xs text-gray-600">
            <AlertTriangle
              className="text-moto-amber mt-0.5 h-3.5 w-3.5 shrink-0"
              aria-hidden
            />
            <span>
              {s.without_email > 0 && s.without_portal > 0
                ? "Bei manchen Bezugspersonen fehlen E-Mail-Adresse oder Portalzugang."
                : s.without_email > 0
                  ? "Bei manchen Bezugspersonen fehlt die E-Mail-Adresse."
                  : "Bei manchen Bezugspersonen fehlt der Portalzugang."}{" "}
              Tragen Sie die fehlenden Daten bei der Bezugsperson ein. Senden
              Sie den Brief danach noch einmal.
            </span>
          </p>
        )}
      </InfoCard>

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
            disabled={busy !== null || !hasFailures}
            className="gap-1.5"
          >
            <RefreshCw className="h-4 w-4" aria-hidden />
            {busy === "resend"
              ? "Wird gesendet …"
              : "Fehlgeschlagene erneut senden"}
          </Button>
        </div>
      )}

      {notice && (
        <p className="rounded-lg border border-gray-200 bg-gray-50 p-3 text-sm text-gray-700">
          {notice}
        </p>
      )}
      {loadError && status && (
        <p className="text-moto-red-strong text-sm">{loadError}</p>
      )}

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
          <ul className="divide-y divide-gray-100 rounded-lg border border-gray-200 bg-white">
            {children.map((child) => (
              <ChildRow key={child.student_id} child={child} />
            ))}
          </ul>
        )}
      </section>

      <section>
        <h3 className="mb-2 text-sm font-semibold text-gray-900">Empfänger</h3>
        {status.recipients.length === 0 ? (
          <p className="text-sm text-gray-500">Noch keine Empfänger erfasst.</p>
        ) : (
          <ul className="divide-y divide-gray-100 rounded-lg border border-gray-200 bg-white">
            {status.recipients.map((r, index) => (
              <RecipientRow
                key={`${r.email ?? "ohne"}-${index}`}
                recipient={r}
              />
            ))}
          </ul>
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
        <StatusBadge label="Offen" tone="orange" />
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

function RecipientRow({ recipient }: { readonly recipient: LetterRecipient }) {
  // Fall back rather than crash: the backend may add a status this build does
  // not know yet, and an unknown value must not take the whole page down.
  const email = EMAIL_LABELS[recipient.email_status] ?? {
    label: recipient.email_status,
    tone: "gray" as StatusBadgeTone,
  };
  const portal = REACHABILITY_LABELS[recipient.reachability] ?? null;
  return (
    <li className="flex flex-wrap items-center justify-between gap-2 px-4 py-3">
      <div className="min-w-0">
        <p className="truncate text-sm text-gray-900">
          {fullName(recipient.first_name, recipient.last_name)}
        </p>
        {recipient.email && (
          <p className="truncate text-xs text-gray-500">{recipient.email}</p>
        )}
        {recipient.last_error && (
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
