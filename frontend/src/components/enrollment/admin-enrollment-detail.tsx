"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  type AdminRequestSummary,
  type ChildStatus,
  type DecisionStatus,
  decideAdminChild,
  getAdminRequest,
} from "~/lib/enrollment-admin-api";
import { useTenantSlugSafe } from "~/components/tenant/tenant-provider";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AdminEnrollmentDetail" });

const STATUS_LABELS: Record<ChildStatus, string> = {
  submitted: "Eingegangen",
  under_review: "In Prüfung",
  approved: "Bestätigt",
  waitlisted: "Warteliste",
  rejected: "Abgelehnt",
  withdrawn: "Zurückgezogen",
};

const STATUS_COLORS: Record<ChildStatus, { bg: string; text: string }> = {
  submitted: { bg: "#5080D8", text: "#FFFFFF" },
  under_review: { bg: "#5080D8", text: "#FFFFFF" },
  approved: { bg: "#83CD2D", text: "#FFFFFF" },
  waitlisted: { bg: "#F78C10", text: "#FFFFFF" },
  rejected: { bg: "#FF3130", text: "#FFFFFF" },
  withdrawn: { bg: "#6B7280", text: "#FFFFFF" },
};

const TERMINAL: ReadonlySet<ChildStatus> = new Set([
  "approved",
  "rejected",
  "withdrawn",
]);

interface ActionDef {
  status: DecisionStatus;
  label: string;
  bgColor: string;
}

const ACTIONS: ActionDef[] = [
  { status: "approved", label: "Bestätigen", bgColor: "#83CD2D" },
  { status: "waitlisted", label: "Warteliste", bgColor: "#F78C10" },
  { status: "rejected", label: "Ablehnen", bgColor: "#FF3130" },
  { status: "under_review", label: "Zur Prüfung", bgColor: "#5080D8" },
];

interface Props {
  readonly requestId: string;
}

export function AdminEnrollmentDetail({ requestId }: Props) {
  const tenantSlug = useTenantSlugSafe();
  const [data, setData] = useState<AdminRequestSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);
  const [busyChildId, setBusyChildId] = useState<string | null>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const fresh = await getAdminRequest(requestId);
      setData(fresh);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("admin_enrollment_detail_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [requestId]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleDecide = async (childId: string, status: DecisionStatus) => {
    if (!data) return;
    const reason = (reasons[childId] ?? "").trim();
    setBusyChildId(childId);
    setError(null);
    setInfo(null);
    try {
      await decideAdminChild(requestId, childId, status, reason || undefined);
      setInfo(
        `Entscheidung gespeichert: ${STATUS_LABELS[status as ChildStatus]}`,
      );
      await load();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("admin_enrollment_decide_failed", {
        error: message,
        request_id: requestId,
        child_id: childId,
        status,
      });
      setError(message);
    } finally {
      setBusyChildId(null);
    }
  };

  if (loading) {
    return <p className="text-sm text-gray-500">Wird geladen...</p>;
  }
  if (!data) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800">
        {error ?? "Anmeldung nicht gefunden."}
      </div>
    );
  }

  return (
    <>
      <header className="space-y-2">
        <Link
          href={
            tenantSlug
              ? `/${tenantSlug}/admin/enrollments`
              : `/admin/enrollments`
          }
          className="text-sm text-[#5080D8] hover:underline"
        >
          ← Zur Übersicht
        </Link>
        <h1 className="text-2xl font-semibold text-gray-900">
          Anmeldung von {data.guardian_first_name} {data.guardian_last_name}
        </h1>
        <p className="text-sm text-gray-600">
          Phase: <strong>{data.phase_name || "—"}</strong>
          {" · "}
          Eingegangen am{" "}
          {new Date(data.submitted_at).toLocaleString("de-DE", {
            day: "2-digit",
            month: "long",
            year: "numeric",
            hour: "2-digit",
            minute: "2-digit",
          })}
        </p>
      </header>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {error}
        </div>
      )}
      {info && (
        <div className="rounded-lg border border-green-200 bg-green-50 p-3 text-sm text-green-800">
          {info}
        </div>
      )}

      <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm">
        <h2 className="text-base font-semibold text-gray-900">
          Erziehungsberechtigte/r
        </h2>
        <dl className="mt-3 grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-xs text-gray-500 uppercase">Name</dt>
            <dd className="text-gray-900">
              {data.guardian_first_name} {data.guardian_last_name}
            </dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 uppercase">E-Mail</dt>
            <dd className="text-gray-900">{data.guardian_email}</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 uppercase">Telefon</dt>
            <dd className="text-gray-900">{data.guardian_phone ?? "—"}</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 uppercase">Status-Link</dt>
            <dd>
              <code className="rounded bg-gray-100 px-2 py-1 text-xs break-all">
                /enroll/status/{data.status_token}
              </code>
            </dd>
          </div>
        </dl>
      </section>

      <section className="space-y-3">
        <h2 className="text-base font-semibold text-gray-900">Kinder</h2>
        {data.children.map((c) => {
          const styles = STATUS_COLORS[c.status];
          const terminal = TERMINAL.has(c.status);
          return (
            <div
              key={c.id}
              className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm"
            >
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <h3 className="text-base font-semibold text-gray-900">
                    {c.first_name} {c.last_name}
                  </h3>
                  <p className="mt-1 text-xs text-gray-600">
                    Geburtsdatum: {c.date_of_birth}
                    {c.target_grade_level
                      ? ` · ${c.target_grade_level}. Klasse`
                      : ""}
                  </p>
                  {c.status_reason && (
                    <p className="mt-2 text-sm text-gray-700">
                      <span className="text-xs font-medium text-gray-500">
                        Begründung:{" "}
                      </span>
                      {c.status_reason}
                    </p>
                  )}
                  {c.reviewed_at && (
                    <p className="mt-1 text-xs text-gray-500">
                      Letzte Entscheidung:{" "}
                      {new Date(c.reviewed_at).toLocaleString("de-DE", {
                        day: "2-digit",
                        month: "2-digit",
                        year: "numeric",
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </p>
                  )}
                </div>
                <span
                  className="rounded-full px-3 py-1 text-xs font-medium"
                  style={{ backgroundColor: styles.bg, color: styles.text }}
                >
                  {STATUS_LABELS[c.status]}
                </span>
              </div>

              {!terminal && (
                <div className="mt-4 space-y-2">
                  <label className="block">
                    <span className="text-xs font-medium text-gray-700">
                      Begründung (optional, sichtbar je nach Phaseneinstellung)
                    </span>
                    <textarea
                      value={reasons[c.id] ?? ""}
                      onChange={(e) =>
                        setReasons((prev) => ({
                          ...prev,
                          [c.id]: e.target.value,
                        }))
                      }
                      rows={2}
                      placeholder="z. B. Geschwisterkind bevorzugt, voll ausgebucht..."
                      className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
                    />
                  </label>
                  <div className="flex flex-wrap gap-2">
                    {ACTIONS.map((a) => {
                      const isCurrent = c.status === a.status;
                      return (
                        <button
                          key={a.status}
                          type="button"
                          disabled={busyChildId === c.id || isCurrent}
                          onClick={() => void handleDecide(c.id, a.status)}
                          className="rounded-md px-3 py-1.5 text-xs font-medium text-white transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
                          style={{ backgroundColor: a.bgColor }}
                        >
                          {busyChildId === c.id
                            ? "Wird gespeichert..."
                            : a.label}
                        </button>
                      );
                    })}
                  </div>
                </div>
              )}
              {terminal && (
                <p className="mt-3 text-xs text-gray-500">
                  Diese Entscheidung ist final. Für Promotionen (z. B.
                  Warteliste → Bestätigt) folgt die volle Workflow- Logik in
                  einer kommenden Version.
                </p>
              )}
            </div>
          );
        })}
      </section>
    </>
  );
}
