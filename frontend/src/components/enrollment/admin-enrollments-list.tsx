"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  type AdminRequestSummary,
  type ChildStatus,
  listAdminRequests,
} from "~/lib/enrollment-admin-api";
import { listPhases, type Phase } from "~/lib/enrollment-phase-api";
import { useTenantSlugSafe } from "~/components/tenant/tenant-provider";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AdminEnrollmentsList" });

const STATUS_LABELS: Record<ChildStatus, string> = {
  submitted: "Eingegangen",
  under_review: "In Prüfung",
  approved: "Bestätigt",
  waitlisted: "Warteliste",
  rejected: "Abgelehnt",
  withdrawn: "Zurückgezogen",
};

// Brand-color hexes per LOCATION_COLORS in lib/location-helper.ts.
const STATUS_COLORS: Record<ChildStatus, { bg: string; text: string }> = {
  submitted: { bg: "#5080D8", text: "#FFFFFF" },
  under_review: { bg: "#5080D8", text: "#FFFFFF" },
  approved: { bg: "#83CD2D", text: "#FFFFFF" },
  waitlisted: { bg: "#F78C10", text: "#FFFFFF" },
  rejected: { bg: "#FF3130", text: "#FFFFFF" },
  withdrawn: { bg: "#6B7280", text: "#FFFFFF" },
};

const ALL_FILTER_VALUE = "all";

export function AdminEnrollmentsList() {
  const tenantSlug = useTenantSlugSafe();
  const [phases, setPhases] = useState<Phase[]>([]);
  const [requests, setRequests] = useState<AdminRequestSummary[]>([]);
  const [phaseFilter, setPhaseFilter] = useState<string>(ALL_FILTER_VALUE);
  const [statusFilter, setStatusFilter] = useState<string>(ALL_FILTER_VALUE);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const [phasesData, requestsData] = await Promise.all([
          listPhases(),
          listAdminRequests({
            phaseId: phaseFilter !== ALL_FILTER_VALUE ? phaseFilter : undefined,
            childStatus:
              statusFilter !== ALL_FILTER_VALUE
                ? (statusFilter as ChildStatus)
                : undefined,
          }),
        ]);
        if (cancelled) return;
        setPhases(phasesData);
        setRequests(requestsData);
      } catch (err) {
        if (cancelled) return;
        const message =
          err instanceof Error ? err.message : "Unbekannter Fehler";
        logger.error("admin_enrollments_load_failed", { error: message });
        setError(message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [phaseFilter, statusFilter]);

  const totals = useMemo(() => {
    const counts: Partial<Record<ChildStatus, number>> = {};
    for (const r of requests) {
      for (const c of r.children) {
        counts[c.status] = (counts[c.status] ?? 0) + 1;
      }
    }
    return counts;
  }, [requests]);

  if (loading) {
    return (
      <p className="text-sm text-gray-500">Anmeldungen werden geladen...</p>
    );
  }

  return (
    <div className="space-y-4">
      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {error}
        </div>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <label className="text-sm">
          <span className="mr-2 font-medium text-gray-700">Phase:</span>
          <select
            value={phaseFilter}
            onChange={(e) => setPhaseFilter(e.target.value)}
            className="rounded-md border-gray-300 px-3 py-2 text-sm shadow-sm"
          >
            <option value={ALL_FILTER_VALUE}>Alle Phasen</option>
            {phases.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
        </label>
        <label className="text-sm">
          <span className="mr-2 font-medium text-gray-700">Status:</span>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="rounded-md border-gray-300 px-3 py-2 text-sm shadow-sm"
          >
            <option value={ALL_FILTER_VALUE}>Alle</option>
            {(Object.keys(STATUS_LABELS) as ChildStatus[]).map((s) => (
              <option key={s} value={s}>
                {STATUS_LABELS[s]}
              </option>
            ))}
          </select>
        </label>

        <div className="ml-auto flex flex-wrap gap-2 text-xs text-gray-600">
          {(Object.keys(STATUS_LABELS) as ChildStatus[]).map((s) => {
            const count = totals[s] ?? 0;
            if (count === 0) return null;
            return (
              <span
                key={s}
                className="rounded-full px-2 py-0.5 font-medium"
                style={{
                  backgroundColor: STATUS_COLORS[s].bg,
                  color: STATUS_COLORS[s].text,
                }}
              >
                {STATUS_LABELS[s]}: {count}
              </span>
            );
          })}
        </div>
      </div>

      {requests.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-gray-300 bg-gray-50 p-6 text-center text-sm text-gray-500">
          Keine Anmeldungen für diese Filter.
        </div>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-gray-200">
          <table className="min-w-full divide-y divide-gray-200 text-sm">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-3 py-2 text-left text-xs font-semibold text-gray-600">
                  Eltern
                </th>
                <th className="px-3 py-2 text-left text-xs font-semibold text-gray-600">
                  Phase
                </th>
                <th className="px-3 py-2 text-left text-xs font-semibold text-gray-600">
                  Eingegangen
                </th>
                <th className="px-3 py-2 text-left text-xs font-semibold text-gray-600">
                  Kinder
                </th>
                <th className="px-3 py-2 text-right text-xs font-semibold text-gray-600">
                  Aktionen
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 bg-white">
              {requests.map((r) => (
                <tr key={r.id}>
                  <td className="px-3 py-2 align-top">
                    <p className="font-medium text-gray-900">
                      {r.guardian_first_name} {r.guardian_last_name}
                    </p>
                    <p className="text-xs text-gray-500">{r.guardian_email}</p>
                  </td>
                  <td className="px-3 py-2 align-top text-gray-700">
                    {r.phase_name || "—"}
                  </td>
                  <td className="px-3 py-2 align-top text-xs text-gray-600">
                    {new Date(r.submitted_at).toLocaleString("de-DE", {
                      day: "2-digit",
                      month: "2-digit",
                      year: "numeric",
                      hour: "2-digit",
                      minute: "2-digit",
                    })}
                  </td>
                  <td className="px-3 py-2 align-top">
                    <ul className="space-y-1">
                      {r.children.map((c) => (
                        <li
                          key={c.id}
                          className="flex items-center gap-2 text-xs"
                        >
                          <span className="text-gray-900">
                            {c.first_name} {c.last_name}
                          </span>
                          <span
                            className="inline-flex rounded-full px-2 py-0.5 text-[10px] font-medium"
                            style={{
                              backgroundColor: STATUS_COLORS[c.status].bg,
                              color: STATUS_COLORS[c.status].text,
                            }}
                          >
                            {STATUS_LABELS[c.status]}
                          </span>
                        </li>
                      ))}
                    </ul>
                  </td>
                  <td className="px-3 py-2 text-right align-top">
                    <Link
                      href={
                        tenantSlug
                          ? `/${tenantSlug}/admin/enrollments/${r.id}`
                          : `/admin/enrollments/${r.id}`
                      }
                      className="text-xs font-medium text-[#5080D8] hover:underline"
                    >
                      Details →
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
