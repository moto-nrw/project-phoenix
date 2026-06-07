"use client";

import { useCallback, useEffect, useState } from "react";
import {
  AlertTriangle,
  Check,
  Clock,
  Mail,
  Pencil,
  ShieldCheck,
  UserRound,
} from "lucide-react";
import {
  confirmRenewal,
  fetchStatus,
  patchStatus,
  withdrawStatus,
  type StatusChild,
  type StatusResponse,
} from "~/lib/enrollment-submission-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentStatusView" });

const STATUS_LABELS: Record<StatusChild["status"], string> = {
  submitted: "Eingegangen",
  under_review: "In Prüfung",
  approved: "Bestätigt",
  waitlisted: "Warteliste",
  rejected: "Abgelehnt",
  withdrawn: "Zurückgezogen",
  pending_renewal: "Bestätigung erforderlich",
  auto_renewed: "Vorgemerkt",
  pending_admin_review: "In Prüfung",
};

const STATUS_STYLES: Record<
  StatusChild["status"],
  { dot: string; text: string; bg: string }
> = {
  submitted: { dot: "#5080D8", text: "#374151", bg: "#F3F4F6" },
  under_review: { dot: "#5080D8", text: "#374151", bg: "#F3F4F6" },
  approved: { dot: "#83CD2D", text: "#5A8E1F", bg: "#83CD2D1A" },
  waitlisted: { dot: "#F78C10", text: "#7C4A03", bg: "#F78C101A" },
  rejected: { dot: "#FF3130", text: "#9F1F1E", bg: "#FF31301A" },
  withdrawn: { dot: "#6B7280", text: "#374151", bg: "#F3F4F6" },
  pending_renewal: { dot: "#F78C10", text: "#7C4A03", bg: "#F78C101A" },
  auto_renewed: { dot: "#5080D8", text: "#374151", bg: "#F3F4F6" },
  pending_admin_review: { dot: "#6B7280", text: "#374151", bg: "#F3F4F6" },
};

const TERMINAL_STATUSES = new Set<StatusChild["status"]>([
  "approved",
  "rejected",
  "withdrawn",
]);

interface Props {
  readonly token: string;
  readonly justSubmitted?: boolean;
}

export function EnrollmentStatusView({ token, justSubmitted = false }: Props) {
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [savingEdit, setSavingEdit] = useState(false);
  const [withdrawingChild, setWithdrawingChild] = useState<string | null>(null);
  const [confirmingRenewal, setConfirmingRenewal] = useState(false);

  const [editFirstName, setEditFirstName] = useState("");
  const [editLastName, setEditLastName] = useState("");
  const [editPhone, setEditPhone] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setNotFound(false);
    try {
      const result = await fetchStatus(token);
      if (!result) {
        setNotFound(true);
        return;
      }
      setStatus(result);
      setEditFirstName(result.guardian_first_name);
      setEditLastName(result.guardian_last_name);
      setEditPhone(result.guardian_phone ?? "");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("status_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [token]);

  useEffect(() => {
    void load();
  }, [load]);

  const allEditable =
    !!status &&
    !status.withdrawn_at &&
    status.children.length > 0 &&
    status.children.every((c) => c.status === "submitted");

  const handleEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!status) return;
    setSavingEdit(true);
    setError(null);
    setInfo(null);
    try {
      await patchStatus(token, {
        guardian_first_name: editFirstName.trim(),
        guardian_last_name: editLastName.trim(),
        guardian_phone: editPhone.trim() || undefined,
      });
      setInfo("Änderungen gespeichert.");
      setEditing(false);
      await load();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("status_edit_failed", { error: message });
      setError(message);
    } finally {
      setSavingEdit(false);
    }
  };

  const handleConfirmRenewal = async () => {
    if (!status) return;
    setConfirmingRenewal(true);
    setError(null);
    setInfo(null);
    try {
      const confirmed = await confirmRenewal(token);
      setInfo(
        confirmed === 1
          ? "Anmeldung bestätigt. Die Schulleitung prüft die Anmeldung."
          : `${confirmed} Anmeldungen bestätigt. Die Schulleitung prüft sie nun.`,
      );
      await load();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("status_confirm_renewal_failed", { error: message });
      setError(message);
    } finally {
      setConfirmingRenewal(false);
    }
  };

  const handleWithdraw = async (childID?: string) => {
    if (!status) return;
    const confirmMessage = childID
      ? "Möchtest du diese Anmeldung wirklich zurückziehen?"
      : "Möchtest du die gesamte Anmeldung zurückziehen?";
    if (!window.confirm(confirmMessage)) return;
    setWithdrawingChild(childID ?? "__all__");
    setError(null);
    setInfo(null);
    try {
      await withdrawStatus(token, childID);
      setInfo(
        childID
          ? "Anmeldung für dieses Kind zurückgezogen."
          : "Anmeldung zurückgezogen.",
      );
      await load();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("status_withdraw_failed", { error: message });
      setError(message);
    } finally {
      setWithdrawingChild(null);
    }
  };

  if (loading) {
    return (
      <div className="moto-content-surface rounded-xl border p-5 text-sm font-medium text-gray-600 shadow-sm sm:p-6">
        Status wird geladen…
      </div>
    );
  }

  if (notFound) {
    return (
      <div className="moto-content-surface rounded-xl border border-[#FF3130]/30 bg-[#FF3130]/5 p-5 text-center shadow-sm sm:p-6">
        <h1 className="text-xl font-semibold text-[#CC2626]">
          Status-Link ungültig
        </h1>
        <p className="mt-2 text-sm text-[#CC2626]">
          Der Link ist abgelaufen oder ungültig. Bitte prüfen Sie die URL aus
          Ihrer Bestätigungs-E-Mail oder wenden Sie sich an die OGS.
        </p>
      </div>
    );
  }

  if (!status) {
    return (
      <div className="moto-content-surface rounded-xl border border-[#FF3130]/30 bg-[#FF3130]/5 p-5 text-sm text-[#CC2626] shadow-sm sm:p-6">
        {error ?? "Status konnte nicht geladen werden."}
      </div>
    );
  }

  const submittedDate = new Date(status.submitted_at).toLocaleDateString(
    "de-DE",
    { day: "2-digit", month: "long", year: "numeric" },
  );

  const allWithdrawn =
    !!status.withdrawn_at ||
    status.children.every((c) => c.status === "withdrawn");
  const hasMultipleChildren = status.children.length > 1;

  const pendingRenewalCount = status.children.filter(
    (c) => c.status === "pending_renewal",
  ).length;
  const autoRenewedCount = status.children.filter(
    (c) => c.status === "auto_renewed",
  ).length;
  const showOptInBanner = pendingRenewalCount > 0;
  const showOptOutBanner = !showOptInBanner && autoRenewedCount > 0;

  return (
    <div className="mx-auto max-w-5xl space-y-5 sm:space-y-6">
      <section className="moto-content-surface overflow-hidden rounded-3xl border shadow-sm">
        <div className="grid lg:grid-cols-[minmax(0,1fr)_22rem]">
          <div className="p-5 sm:p-8 lg:p-10">
            <div
              className={`flex h-12 w-12 items-center justify-center rounded-xl sm:h-14 sm:w-14 ${
                justSubmitted
                  ? "bg-[#83CD2D]/15 text-[#5A8E1F]"
                  : "moto-content-surface border text-gray-600 shadow-sm"
              }`}
            >
              {justSubmitted ? (
                <Check className="h-7 w-7" aria-hidden="true" />
              ) : (
                <Clock className="h-7 w-7" aria-hidden="true" />
              )}
            </div>
            <p className="mt-6 text-sm font-semibold tracking-wide text-[#5080D8] uppercase">
              {justSubmitted ? "Anmeldung eingegangen" : "Status"}
            </p>
            <h1 className="mt-2 max-w-2xl text-3xl font-bold tracking-tight text-wrap text-gray-900 sm:text-4xl">
              {justSubmitted
                ? "Danke. Ihre Anmeldung wurde übermittelt."
                : "Status Ihrer Anmeldung"}
            </h1>
            <p className="mt-4 max-w-2xl text-base leading-7 text-gray-600">
              {justSubmitted
                ? "Die OGS prüft die Angaben. Den aktuellen Stand sehen Sie auf dieser Seite. Speichern Sie den Link am besten direkt oder bewahren Sie die Bestätigungs-E-Mail auf."
                : `Ihre Anmeldung ist am ${submittedDate} eingegangen. Hier sehen Sie den aktuellen Stand, Ihre Kontaktdaten und die angemeldeten Kinder.`}
            </p>
          </div>
          <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 sm:p-8 lg:border-t-0 lg:border-l">
            <h2 className="text-lg font-semibold text-gray-900">
              {justSubmitted ? "Was jetzt passiert" : "Aktueller Stand"}
            </h2>
            <ol className="mt-5 space-y-4 text-sm text-gray-600">
              <li className="flex gap-3">
                <StepNumber>1</StepNumber>
                <span>
                  {justSubmitted
                    ? "Die OGS sieht Ihre Anmeldung in der Verwaltung."
                    : "Die Anmeldung liegt der OGS vor."}
                </span>
              </li>
              <li className="flex gap-3">
                <StepNumber>2</StepNumber>
                <span>
                  Bei Rückfragen meldet sich die OGS über die angegebene
                  E-Mail-Adresse.
                </span>
              </li>
              <li className="flex gap-3">
                <StepNumber>3</StepNumber>
                <span>
                  Sobald eine Entscheidung vorliegt, sehen Sie diese im Bereich
                  „Kinder“ und erhalten eine E-Mail.
                </span>
              </li>
            </ol>
          </aside>
        </div>
      </section>

      <section className="grid gap-3 sm:grid-cols-3">
        <StatusSummaryCard
          icon={<Clock className="h-5 w-5" aria-hidden="true" />}
          label="Eingegangen"
          value={submittedDate}
        />
        <StatusSummaryCard
          icon={<ShieldCheck className="h-5 w-5" aria-hidden="true" />}
          label="Kinder"
          value={String(status.children.length)}
        />
        <StatusSummaryCard
          icon={<Mail className="h-5 w-5" aria-hidden="true" />}
          label="Kontakt"
          value={status.guardian_email}
        />
      </section>

      {error && (
        <div className="rounded-2xl border border-[#FF3130]/30 bg-[#FF3130]/5 p-4 text-sm text-[#CC2626]">
          {error}
        </div>
      )}
      {info && (
        <div className="rounded-2xl border border-[#83CD2D]/30 bg-[#83CD2D]/5 p-4 text-sm text-[#5BA01F]">
          {info}
        </div>
      )}

      {showOptInBanner && (
        <section className="moto-content-surface rounded-xl border p-5 shadow-sm sm:p-6">
          <h2 className="text-lg font-semibold text-gray-900">
            Bestätigung erforderlich
          </h2>
          <p className="mt-2 text-sm text-gray-700">
            Wir haben Ihre Anmeldung für den nächsten Zeitraum vorbereitet.
            Damit Ihr Kind dabei sein kann, bestätigen Sie bitte aktiv die
            Anmeldung. Ohne Bestätigung läuft die Anmeldung zur Frist ab.
          </p>
          <div className="mt-4 flex flex-col gap-3 sm:flex-row sm:flex-wrap">
            <button
              type="button"
              onClick={() => void handleConfirmRenewal()}
              disabled={confirmingRenewal}
              className="h-10 rounded-lg bg-gray-900 px-4 text-sm font-semibold text-white shadow-sm hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
            >
              {confirmingRenewal ? "Wird bestätigt…" : "Anmeldung bestätigen"}
            </button>
            <button
              type="button"
              onClick={() => void handleWithdraw()}
              disabled={withdrawingChild === "__all__"}
              className="h-10 rounded-lg border border-gray-200 bg-white px-4 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
            >
              {withdrawingChild === "__all__"
                ? "Wird abgelehnt…"
                : "Anmeldung ablehnen"}
            </button>
          </div>
        </section>
      )}
      {showOptOutBanner && (
        <section className="moto-content-surface rounded-xl border p-5 shadow-sm sm:p-6">
          <h2 className="text-lg font-semibold text-gray-900">
            Anmeldung wurde verlängert
          </h2>
          <p className="mt-2 text-sm text-gray-700">
            Ihre Anmeldung für den nächsten Zeitraum wurde automatisch
            übernommen. Sie müssen nichts weiter tun. Falls Sie nicht teilnehmen
            möchten, melden Sie sich bitte bis zur Frist ab.
          </p>
          <div className="mt-4">
            <button
              type="button"
              onClick={() => void handleWithdraw()}
              disabled={withdrawingChild === "__all__"}
              className="h-10 w-full rounded-lg border border-gray-200 bg-white px-4 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50 sm:w-auto"
            >
              {withdrawingChild === "__all__" ? "Wird abgemeldet…" : "Abmelden"}
            </button>
          </div>
        </section>
      )}

      <section className="moto-content-surface space-y-4 rounded-xl border p-5 shadow-sm sm:p-6">
        <div className="flex items-center justify-between gap-4">
          <div>
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              Anmeldungen
            </p>
            <h2 className="mt-1 text-xl font-semibold text-gray-900">Kinder</h2>
          </div>
        </div>
        <ul className="space-y-3">
          {status.children.map((c) => {
            const canWithdraw =
              hasMultipleChildren && !TERMINAL_STATUSES.has(c.status);
            return (
              <li
                key={c.id}
                className="rounded-xl border border-gray-200 bg-white p-4"
              >
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="flex items-start gap-3">
                    <span className="moto-content-surface flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border text-gray-600 shadow-sm">
                      <UserRound className="h-5 w-5" aria-hidden="true" />
                    </span>
                    <div className="min-w-0">
                      <p className="font-semibold break-words text-gray-900">
                        {c.first_name} {c.last_name}
                      </p>
                      {c.status_reason && (
                        <p className="mt-1 text-sm text-gray-600">
                          {c.status_reason}
                        </p>
                      )}
                    </div>
                  </div>
                  <StatusPill status={c.status} />
                </div>
                <div className="mt-4 flex flex-col gap-3 border-t border-gray-100 pt-3 sm:flex-row sm:items-center sm:justify-between">
                  <p className="text-sm text-gray-500">
                    {c.status === "approved"
                      ? "Diese Anmeldung wurde bereits bestätigt."
                      : canWithdraw
                        ? "Noch keine endgültige Entscheidung getroffen."
                        : "Diese Anmeldung kann nicht mehr online geändert werden."}
                  </p>
                  {canWithdraw && (
                    <button
                      type="button"
                      onClick={() => void handleWithdraw(c.id)}
                      disabled={withdrawingChild === c.id}
                      className="h-9 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50 sm:w-auto"
                    >
                      {withdrawingChild === c.id
                        ? "Wird zurückgezogen…"
                        : "Dieses Kind zurückziehen"}
                    </button>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      </section>

      <section className="moto-content-surface space-y-4 rounded-xl border p-5 shadow-sm sm:p-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              Kontakt
            </p>
            <h2 className="mt-1 text-xl font-semibold text-gray-900">
              Erziehungsberechtigte
            </h2>
          </div>
          {allEditable && !editing && (
            <button
              type="button"
              onClick={() => setEditing(true)}
              className="inline-flex h-9 w-full items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:w-auto"
            >
              <Pencil className="h-4 w-4" aria-hidden="true" />
              Bearbeiten
            </button>
          )}
        </div>
        {!editing ? (
          <dl className="grid gap-3 text-sm text-gray-700 sm:grid-cols-3">
            <div className="rounded-xl border border-gray-200 bg-white p-4">
              <dt className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Name
              </dt>
              <dd className="mt-1 font-semibold text-gray-900">
                {status.guardian_first_name} {status.guardian_last_name}
              </dd>
            </div>
            <div className="rounded-xl border border-gray-200 bg-white p-4">
              <dt className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                E-Mail
              </dt>
              <dd className="mt-1 font-semibold break-all text-gray-900">
                {status.guardian_email}
              </dd>
            </div>
            <div className="rounded-xl border border-gray-200 bg-white p-4">
              <dt className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Telefon
              </dt>
              <dd className="mt-1 font-semibold text-gray-900">
                {status.guardian_phone ?? "Nicht angegeben"}
              </dd>
            </div>
          </dl>
        ) : (
          <form onSubmit={handleEdit} className="space-y-4 text-sm">
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="block">
                <span className="text-sm font-semibold text-gray-700">
                  Vorname
                </span>
                <input
                  type="text"
                  value={editFirstName}
                  onChange={(e) => setEditFirstName(e.target.value)}
                  required
                  className="mt-2 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none"
                />
              </label>
              <label className="block">
                <span className="text-sm font-semibold text-gray-700">
                  Nachname
                </span>
                <input
                  type="text"
                  value={editLastName}
                  onChange={(e) => setEditLastName(e.target.value)}
                  required
                  className="mt-2 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none"
                />
              </label>
            </div>
            <label className="block">
              <span className="text-sm font-semibold text-gray-700">
                Telefon, optional
              </span>
              <input
                type="tel"
                value={editPhone}
                onChange={(e) => setEditPhone(e.target.value)}
                className="mt-2 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none"
              />
            </label>
            <p className="text-sm text-gray-500">
              Die E-Mail-Adresse kann hier nicht geändert werden. Bitte wenden
              Sie sich dafür direkt an die OGS.
            </p>
            <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap">
              <button
                type="submit"
                disabled={savingEdit}
                className="h-10 rounded-lg bg-gray-900 px-4 text-sm font-semibold text-white shadow-sm hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
              >
                {savingEdit ? "Speichert…" : "Speichern"}
              </button>
              <button
                type="button"
                onClick={() => {
                  setEditing(false);
                  setEditFirstName(status.guardian_first_name);
                  setEditLastName(status.guardian_last_name);
                  setEditPhone(status.guardian_phone ?? "");
                }}
                disabled={savingEdit}
                className="h-10 rounded-lg border border-gray-200 bg-white px-4 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
              >
                Abbrechen
              </button>
            </div>
          </form>
        )}
        {!allEditable && !editing && (
          <p className="text-sm text-gray-500">
            Änderungen sind nur möglich, solange noch keine Entscheidung
            getroffen wurde. Für Änderungen wende dich bitte an die OGS.
          </p>
        )}
      </section>

      {!allWithdrawn && (
        <section className="moto-content-surface rounded-xl border p-5 shadow-sm sm:p-6">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="flex gap-3">
              <span className="moto-content-surface flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border text-gray-600 shadow-sm">
                <AlertTriangle className="h-5 w-5" aria-hidden="true" />
              </span>
              <div>
                <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  Anmeldung verwalten
                </p>
                <h2 className="mt-1 text-xl font-semibold text-gray-900">
                  {hasMultipleChildren
                    ? "Gesamte Anmeldung zurückziehen"
                    : "Anmeldung zurückziehen"}
                </h2>
                <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
                  Sie können die Anmeldung zurückziehen, solange noch keine
                  Entscheidung getroffen wurde. Bereits bestätigte Kinder
                  bleiben unverändert. Wenden Sie sich dafür bitte direkt an die
                  OGS.
                </p>
              </div>
            </div>
            <button
              type="button"
              onClick={() => void handleWithdraw()}
              disabled={withdrawingChild === "__all__"}
              className="h-10 w-full shrink-0 rounded-lg border border-gray-200 bg-white px-4 text-sm font-semibold text-gray-700 shadow-sm hover:border-[#FF3130]/40 hover:bg-[#FF3130]/5 hover:text-[#9F1F1E] focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50 sm:w-auto"
            >
              {withdrawingChild === "__all__"
                ? "Wird zurückgezogen…"
                : hasMultipleChildren
                  ? "Gesamte Anmeldung zurückziehen"
                  : "Anmeldung zurückziehen"}
            </button>
          </div>
        </section>
      )}
    </div>
  );
}

function StepNumber({ children }: { readonly children: React.ReactNode }) {
  return (
    <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gray-900 text-xs font-semibold text-white">
      {children}
    </span>
  );
}

function StatusPill({ status }: { readonly status: StatusChild["status"] }) {
  const styles = STATUS_STYLES[status];
  return (
    <span
      className="inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-semibold"
      style={{
        backgroundColor: styles.bg,
        color: styles.text,
      }}
    >
      <span
        className="h-2 w-2 rounded-full"
        style={{ backgroundColor: styles.dot }}
      />
      {STATUS_LABELS[status]}
    </span>
  );
}

function StatusSummaryCard({
  icon,
  label,
  value,
}: {
  readonly icon: React.ReactNode;
  readonly label: string;
  readonly value: string;
}) {
  return (
    <div className="moto-content-surface rounded-xl border p-4 shadow-sm">
      <div className="flex items-center gap-3">
        <span className="moto-content-surface flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border text-gray-600 shadow-sm">
          {icon}
        </span>
        <div className="min-w-0">
          <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
            {label}
          </p>
          <p className="truncate text-sm font-semibold text-gray-900">
            {value}
          </p>
        </div>
      </div>
    </div>
  );
}
