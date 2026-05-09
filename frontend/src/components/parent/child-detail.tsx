"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { type Child, type ChildStatus, listMyChildren } from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ChildDetail" });

const STATUS_LABEL: Record<ChildStatus, string> = {
  pending: "Bestätigt",
  active: "Aktiv",
  inactive: "Beendet",
  alumnus: "Ehemalig",
};

const STATUS_COLORS: Record<ChildStatus, { bg: string; text: string }> = {
  pending: { bg: "#5080D8", text: "#FFFFFF" },
  active: { bg: "#83CD2D", text: "#FFFFFF" },
  inactive: { bg: "#6B7280", text: "#FFFFFF" },
  alumnus: { bg: "#6B7280", text: "#FFFFFF" },
};

function formatDate(iso: string | undefined): string {
  if (!iso) return "—";
  try {
    return new Date(iso).toLocaleDateString("de-DE", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
    });
  } catch {
    return iso;
  }
}

interface Props {
  readonly studentId: string;
}

/**
 * Read-only stub of the per-child page. Reuses /api/parent/me/children
 * (cheap — the list is small and already cached on the dashboard) and
 * filters client-side. A future commit can swap to a dedicated
 * /api/parent/children/{id} endpoint if/when there's enough per-child
 * data to justify the round-trip.
 */
export function ChildDetail({ studentId }: Props) {
  const [child, setChild] = useState<Child | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await listMyChildren();
      const match = list.find((c) => c.student_id === studentId) ?? null;
      setChild(match);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("child_detail_load_failed", {
        error: message,
        student_id: studentId,
      });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [studentId]);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) {
    return <p className="text-sm text-gray-500">Wird geladen...</p>;
  }
  if (error) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800">
        {error}
      </div>
    );
  }
  if (!child) {
    return (
      <div className="space-y-3">
        <Link
          href="/parents"
          className="text-sm text-[#5080D8] hover:underline"
        >
          ← Zur Übersicht
        </Link>
        <div className="rounded-lg border border-gray-200 bg-white p-6 text-sm text-gray-600 shadow-sm">
          Kein Kind mit dieser Kennung gefunden. Vermutlich gehört es zu einem
          Schulwechsel oder die Anmeldung wurde zurückgezogen.
        </div>
      </div>
    );
  }

  const status = STATUS_COLORS[child.status] ?? STATUS_COLORS.inactive;

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <Link
          href="/parents"
          className="text-sm text-[#5080D8] hover:underline"
        >
          ← Zur Übersicht
        </Link>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h1 className="text-2xl font-semibold text-gray-900">
              {child.first_name} {child.last_name}
            </h1>
            <p className="mt-1 text-sm text-gray-600">
              {child.school_name}
              {child.school_class ? ` · ${child.school_class}` : ""}
            </p>
          </div>
          <span
            className="rounded-full px-3 py-1 text-xs font-medium"
            style={{ backgroundColor: status.bg, color: status.text }}
          >
            {STATUS_LABEL[child.status]}
          </span>
        </div>
      </header>

      <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm">
        <h2 className="text-base font-semibold text-gray-900">Betreuung</h2>
        <dl className="mt-3 grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-xs text-gray-500 uppercase">Status</dt>
            <dd className="text-gray-900">{STATUS_LABEL[child.status]}</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 uppercase">Schule</dt>
            <dd className="text-gray-900">{child.school_name}</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 uppercase">Klasse</dt>
            <dd className="text-gray-900">{child.school_class || "—"}</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 uppercase">
              Betreuungszeitraum
            </dt>
            <dd className="text-gray-900">
              {child.enrolled_from
                ? `${formatDate(child.enrolled_from)} – ${formatDate(child.enrolled_until)}`
                : "—"}
            </dd>
          </div>
        </dl>
      </section>

      <p className="text-xs text-gray-500">
        Weitere Funktionen — Anwesenheit, Abmeldungen, Mitteilungen — folgen in
        einer kommenden Version.
      </p>
    </div>
  );
}
