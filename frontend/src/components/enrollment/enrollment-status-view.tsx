"use client";

import { useCallback, useEffect, useState } from "react";
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

// MOTO brand hex codes via LOCATION_COLORS — see CLAUDE.md §0.
const STATUS_STYLES: Record<
  StatusChild["status"],
  { bg: string; text: string; border: string }
> = {
  submitted: { bg: "#5080D8", text: "#FFFFFF", border: "#5080D8" },
  under_review: { bg: "#5080D8", text: "#FFFFFF", border: "#5080D8" },
  approved: { bg: "#83CD2D", text: "#FFFFFF", border: "#83CD2D" },
  waitlisted: { bg: "#F78C10", text: "#FFFFFF", border: "#F78C10" },
  rejected: { bg: "#FF3130", text: "#FFFFFF", border: "#FF3130" },
  withdrawn: { bg: "#6B7280", text: "#FFFFFF", border: "#6B7280" },
  // Renewal states use amber (opt-in needs action) and a softer blue
  // (opt-out is "we have you on the list").
  pending_renewal: { bg: "#F59E0B", text: "#FFFFFF", border: "#F59E0B" },
  auto_renewed: { bg: "#5080D8", text: "#FFFFFF", border: "#5080D8" },
  pending_admin_review: { bg: "#6B7280", text: "#FFFFFF", border: "#6B7280" },
};

const TERMINAL_STATUSES = new Set<StatusChild["status"]>([
  "approved",
  "rejected",
  "withdrawn",
]);

interface Props {
  readonly token: string;
}

export function EnrollmentStatusView({ token }: Props) {
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
    return <p className="text-sm text-gray-500">Status wird geladen...</p>;
  }

  if (notFound) {
    return (
      <div className="rounded-2xl border border-red-200 bg-red-50 p-6 text-center">
        <h1 className="text-xl font-semibold text-red-800">
          Status-Link ungültig
        </h1>
        <p className="mt-2 text-sm text-red-700">
          Der Link ist abgelaufen oder ungültig. Bitte überprüfe die URL aus
          deiner Bestätigungs-E-Mail oder wende dich an die OGS.
        </p>
      </div>
    );
  }

  if (!status) {
    return (
      <div className="rounded-2xl border border-red-200 bg-red-50 p-6 text-sm text-red-800">
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

  const pendingRenewalCount = status.children.filter(
    (c) => c.status === "pending_renewal",
  ).length;
  const autoRenewedCount = status.children.filter(
    (c) => c.status === "auto_renewed",
  ).length;
  const showOptInBanner = pendingRenewalCount > 0;
  const showOptOutBanner = !showOptInBanner && autoRenewedCount > 0;

  return (
    <div className="space-y-6">
      <header className="moto-content-surface rounded-2xl border p-6 shadow-sm">
        <h1 className="text-2xl font-semibold text-gray-900">
          Status deiner Anmeldung
        </h1>
        <p className="mt-2 text-sm text-gray-600">
          Eingegangen am {submittedDate}
        </p>
      </header>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800">
          {error}
        </div>
      )}
      {info && (
        <div className="rounded-lg border border-green-200 bg-green-50 p-4 text-sm text-green-800">
          {info}
        </div>
      )}

      {showOptInBanner && (
        <section className="rounded-2xl border border-[#F59E0B] bg-[#FEF3C7] p-6 shadow-sm">
          <h2 className="text-lg font-semibold text-gray-900">
            Bestätigung erforderlich
          </h2>
          <p className="mt-2 text-sm text-gray-700">
            Wir haben Ihre Anmeldung für den nächsten Zeitraum vorbereitet.
            Damit Ihr Kind dabei sein kann, bestätigen Sie bitte aktiv die
            Anmeldung. Ohne Bestätigung läuft die Anmeldung zur Frist ab.
          </p>
          <div className="mt-4 flex flex-wrap gap-3">
            <button
              type="button"
              onClick={() => void handleConfirmRenewal()}
              disabled={confirmingRenewal}
              className="rounded-lg bg-[#83CD2D] px-4 py-2 text-sm font-medium text-white shadow hover:bg-[#76b829] disabled:opacity-50"
            >
              {confirmingRenewal ? "Wird bestätigt..." : "Anmeldung bestätigen"}
            </button>
            <button
              type="button"
              onClick={() => void handleWithdraw()}
              disabled={withdrawingChild === "__all__"}
              className="rounded-lg border border-[#FF3130] px-4 py-2 text-sm font-medium text-[#FF3130] hover:bg-[#FF3130]/10 disabled:opacity-50"
            >
              {withdrawingChild === "__all__"
                ? "Wird abgelehnt..."
                : "Anmeldung ablehnen"}
            </button>
          </div>
        </section>
      )}
      {showOptOutBanner && (
        <section className="rounded-2xl border border-[#5080D8] bg-[#EFF6FF] p-6 shadow-sm">
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
              className="rounded-lg border border-[#FF3130] px-4 py-2 text-sm font-medium text-[#FF3130] hover:bg-[#FF3130]/10 disabled:opacity-50"
            >
              {withdrawingChild === "__all__"
                ? "Wird abgemeldet..."
                : "Abmelden"}
            </button>
          </div>
        </section>
      )}

      <section className="moto-content-surface space-y-4 rounded-2xl border p-6 shadow-sm">
        <div className="flex items-center justify-between gap-4">
          <h2 className="text-lg font-semibold text-gray-900">Kinder</h2>
        </div>
        <ul className="space-y-3">
          {status.children.map((c) => {
            const styles = STATUS_STYLES[c.status];
            const canWithdraw = !TERMINAL_STATUSES.has(c.status);
            return (
              <li key={c.id} className="rounded-lg border border-gray-200 p-4">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <p className="font-medium text-gray-900">
                      {c.first_name} {c.last_name}
                    </p>
                    {c.status_reason && (
                      <p className="mt-1 text-sm text-gray-600">
                        {c.status_reason}
                      </p>
                    )}
                  </div>
                  <span
                    className="inline-flex items-center rounded-full px-3 py-1 text-xs font-medium"
                    style={{
                      backgroundColor: styles.bg,
                      color: styles.text,
                    }}
                  >
                    {STATUS_LABELS[c.status]}
                  </span>
                </div>
                {canWithdraw && (
                  <div className="mt-3 flex justify-end">
                    <button
                      type="button"
                      onClick={() => void handleWithdraw(c.id)}
                      disabled={withdrawingChild === c.id}
                      className="text-sm font-medium text-[#FF3130] hover:underline disabled:opacity-50"
                    >
                      {withdrawingChild === c.id
                        ? "Wird zurückgezogen..."
                        : "Anmeldung zurückziehen"}
                    </button>
                  </div>
                )}
                {c.status === "approved" && (
                  <p className="mt-3 text-xs text-gray-500">
                    Diese Anmeldung wurde bereits bestätigt. Für Änderungen
                    wende dich bitte direkt an die OGS.
                  </p>
                )}
              </li>
            );
          })}
        </ul>
      </section>

      <section className="moto-content-surface space-y-4 rounded-2xl border p-6 shadow-sm">
        <div className="flex items-center justify-between gap-4">
          <h2 className="text-lg font-semibold text-gray-900">
            Kontaktdaten der Erziehungsberechtigten
          </h2>
          {allEditable && !editing && (
            <button
              type="button"
              onClick={() => setEditing(true)}
              className="text-sm font-medium text-[#5080D8] hover:underline"
            >
              Bearbeiten
            </button>
          )}
        </div>
        {!editing ? (
          <dl className="space-y-2 text-sm text-gray-700">
            <div>
              <dt className="text-xs text-gray-500 uppercase">Name</dt>
              <dd>
                {status.guardian_first_name} {status.guardian_last_name}
              </dd>
            </div>
            <div>
              <dt className="text-xs text-gray-500 uppercase">E-Mail</dt>
              <dd>{status.guardian_email}</dd>
            </div>
            <div>
              <dt className="text-xs text-gray-500 uppercase">Telefon</dt>
              <dd>{status.guardian_phone ?? "—"}</dd>
            </div>
          </dl>
        ) : (
          <form onSubmit={handleEdit} className="space-y-3 text-sm">
            <div className="grid gap-3 sm:grid-cols-2">
              <label className="block">
                <span className="text-xs font-medium text-gray-700">
                  Vorname
                </span>
                <input
                  type="text"
                  value={editFirstName}
                  onChange={(e) => setEditFirstName(e.target.value)}
                  required
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2"
                />
              </label>
              <label className="block">
                <span className="text-xs font-medium text-gray-700">
                  Nachname
                </span>
                <input
                  type="text"
                  value={editLastName}
                  onChange={(e) => setEditLastName(e.target.value)}
                  required
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2"
                />
              </label>
            </div>
            <label className="block">
              <span className="text-xs font-medium text-gray-700">
                Telefon (optional)
              </span>
              <input
                type="tel"
                value={editPhone}
                onChange={(e) => setEditPhone(e.target.value)}
                className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2"
              />
            </label>
            <p className="text-xs text-gray-500">
              Die E-Mail-Adresse kann hier nicht geändert werden. Wende dich
              dafür bitte direkt an die OGS.
            </p>
            <div className="flex gap-2">
              <button
                type="submit"
                disabled={savingEdit}
                className="rounded-md bg-[#83CD2D] px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
              >
                {savingEdit ? "Speichert..." : "Speichern"}
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
                className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
              >
                Abbrechen
              </button>
            </div>
          </form>
        )}
        {!allEditable && !editing && (
          <p className="text-xs text-gray-500">
            Änderungen sind nur möglich, solange noch keine Entscheidung
            getroffen wurde. Für Änderungen wende dich bitte an die OGS.
          </p>
        )}
      </section>

      {!allWithdrawn && (
        <section className="moto-content-surface rounded-2xl border p-6 shadow-sm">
          <h2 className="text-lg font-semibold text-gray-900">
            Komplette Anmeldung zurückziehen
          </h2>
          <p className="mt-2 text-sm text-gray-600">
            Du kannst die gesamte Anmeldung zurückziehen, solange noch keine
            Entscheidung getroffen wurde. Bereits bestätigte Kinder bleiben
            davon unberührt — wende dich dafür an die OGS.
          </p>
          <button
            type="button"
            onClick={() => void handleWithdraw()}
            disabled={withdrawingChild === "__all__"}
            className="mt-4 rounded-md border border-[#FF3130] px-4 py-2 text-sm font-medium text-[#FF3130] hover:bg-red-50 disabled:opacity-50"
          >
            {withdrawingChild === "__all__"
              ? "Wird zurückgezogen..."
              : "Anmeldung zurückziehen"}
          </button>
        </section>
      )}
    </div>
  );
}
