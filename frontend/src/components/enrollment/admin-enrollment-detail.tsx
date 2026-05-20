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
import { Button } from "~/components/ui/button";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AdminEnrollmentDetail" });

const STATUS_LABELS: Record<ChildStatus, string> = {
  submitted: "Eingegangen",
  under_review: "In Prüfung",
  approved: "Bestätigt",
  waitlisted: "Warteliste",
  rejected: "Abgelehnt",
  withdrawn: "Zurückgezogen",
  pending_renewal: "Wartet auf Verlängerung",
  auto_renewed: "Vorgemerkt",
  pending_admin_review: "Manuelle Prüfung",
};

const STATUS_COLORS: Record<ChildStatus, { bg: string; text: string }> = {
  submitted: { bg: LOCATION_COLORS.OTHER_ROOM, text: "#FFFFFF" },
  under_review: { bg: LOCATION_COLORS.OTHER_ROOM, text: "#FFFFFF" },
  approved: { bg: LOCATION_COLORS.GROUP_ROOM, text: "#FFFFFF" },
  waitlisted: { bg: LOCATION_COLORS.SCHOOLYARD, text: "#FFFFFF" },
  rejected: { bg: LOCATION_COLORS.HOME, text: "#FFFFFF" },
  withdrawn: { bg: LOCATION_COLORS.UNKNOWN, text: "#FFFFFF" },
  pending_renewal: { bg: LOCATION_COLORS.SCHOOLYARD, text: "#FFFFFF" },
  auto_renewed: { bg: LOCATION_COLORS.OTHER_ROOM, text: "#FFFFFF" },
  pending_admin_review: { bg: LOCATION_COLORS.UNKNOWN, text: "#FFFFFF" },
};

const TERMINAL: ReadonlySet<ChildStatus> = new Set([
  "approved",
  "rejected",
  "withdrawn",
]);

interface ActionDef {
  status: DecisionStatus;
  label: string;
  variant: "primary" | "outline" | "danger" | "success";
}

const ACTIONS: ActionDef[] = [
  { status: "approved", label: "Bestätigen", variant: "success" },
  { status: "waitlisted", label: "Warteliste", variant: "outline" },
  { status: "rejected", label: "Ablehnen", variant: "danger" },
  { status: "under_review", label: "Zur Prüfung", variant: "primary" },
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
      <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]">
        {error ?? "Anmeldung nicht gefunden."}
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <PageHeaderWithSearch
        title={`Anmeldung von ${data.guardian_first_name} ${data.guardian_last_name}`}
        actionButton={
          <Link
            href={
              tenantSlug
                ? `/${tenantSlug}/admin/enrollments`
                : `/admin/enrollments`
            }
            className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            Zur Übersicht
          </Link>
        }
      />
      <p className="-mt-2 text-sm text-gray-600">
        Phase: <strong>{data.phase_name || "Nicht zugeordnet"}</strong>
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

      {error && (
        <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]">
          {error}
        </div>
      )}
      {info && (
        <div className="rounded-2xl border border-[#83CD2D]/20 bg-[#83CD2D]/10 p-4 text-sm text-[#5A8B1F]">
          {info}
        </div>
      )}

      <section className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
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
            <dd className="text-gray-900">
              {data.guardian_phone ?? "Nicht gesetzt"}
            </dd>
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
          const terminal = TERMINAL.has(c.status);
          return (
            <div
              key={c.id}
              className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md"
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
                <StatusBadge status={c.status} />
              </div>

              {!terminal && (
                <div className="mt-4 space-y-2">
                  <label className="block">
                    <span className="text-xs font-medium text-gray-700">
                      Begründung (optional, sichtbar je nach
                      Anmeldephaseneinstellung)
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
                      placeholder="z. B. Geschwisterkind bevorzugt, voll ausgebucht"
                      className="mt-1 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                    />
                  </label>
                  <div className="flex flex-wrap gap-2">
                    {ACTIONS.map((a) => {
                      const isCurrent = c.status === a.status;
                      return (
                        <Button
                          key={a.status}
                          type="button"
                          disabled={busyChildId === c.id || isCurrent}
                          onClick={() => void handleDecide(c.id, a.status)}
                          variant={a.variant}
                          size="sm"
                          className="px-3 py-1.5 text-xs shadow-none disabled:cursor-not-allowed disabled:opacity-40"
                        >
                          {busyChildId === c.id
                            ? "Wird gespeichert..."
                            : a.label}
                        </Button>
                      );
                    })}
                  </div>
                </div>
              )}
              {terminal && (
                <p className="mt-3 text-xs text-gray-500">
                  Diese Entscheidung ist final. Für Promotionen (z. B.
                  Warteliste zu Bestätigt) folgt die volle Workflow-Logik in
                  einer kommenden Version.
                </p>
              )}
            </div>
          );
        })}
      </section>
    </div>
  );
}

function StatusBadge({ status }: Readonly<{ status: ChildStatus }>) {
  const styles = STATUS_COLORS[status];
  return (
    <span
      className="inline-flex rounded-full px-3 py-1 text-xs font-medium"
      style={{ backgroundColor: styles.bg, color: styles.text }}
    >
      {STATUS_LABELS[status]}
    </span>
  );
}
