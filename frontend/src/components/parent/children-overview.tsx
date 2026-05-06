"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  type Child,
  type ChildStatus,
  groupBySchool,
  listMyChildren,
} from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ChildrenOverview" });

const STATUS_LABEL: Record<ChildStatus, string> = {
  pending: "Anmeldung läuft",
  active: "Aktiv",
  inactive: "Beendet",
  alumnus: "Ehemalig",
};

// Status colors — sourced from LOCATION_COLORS via the slice 1 admin
// detail page. Inlined here as hex literals for Tailwind JIT.
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

interface SchoolGroupProps {
  readonly schoolName: string;
  readonly students: readonly Child[];
}

function SchoolGroup({ schoolName, students }: SchoolGroupProps) {
  return (
    <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm">
      <h2 className="text-base font-semibold text-gray-900">{schoolName}</h2>
      <ul className="mt-3 space-y-3">
        {students.map((c) => {
          const status = STATUS_COLORS[c.status] ?? STATUS_COLORS.inactive;
          return (
            <li key={`${c.tenant_id}-${c.student_id}`}>
              <Link
                href={`/parents/children/${c.student_id}`}
                className="flex items-center justify-between rounded-xl border border-gray-100 px-4 py-3 transition-colors hover:bg-gray-50"
              >
                <div>
                  <p className="text-sm font-medium text-gray-900">
                    {c.first_name} {c.last_name}
                  </p>
                  <p className="text-xs text-gray-600">
                    {c.school_class ? `${c.school_class} · ` : ""}
                    {c.enrolled_from
                      ? `Betreuung ${formatDate(c.enrolled_from)} – ${formatDate(c.enrolled_until)}`
                      : "Anmeldung läuft"}
                  </p>
                </div>
                <span
                  className="rounded-full px-3 py-1 text-xs font-medium"
                  style={{ backgroundColor: status.bg, color: status.text }}
                >
                  {STATUS_LABEL[c.status]}
                </span>
              </Link>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

export function ChildrenOverview() {
  const [children, setChildren] = useState<Child[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await listMyChildren();
      setChildren(list);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("children_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

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

  const groups = groupBySchool(children);

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">
            Übersicht Ihrer Kinder
          </h1>
          <p className="mt-1 text-sm text-gray-600">
            {children.length === 0
              ? "Noch keine Kinder angemeldet."
              : children.length === 1
                ? "1 Kind angemeldet."
                : `${children.length} Kinder angemeldet.`}
          </p>
        </div>
        <Link
          href="/parents/enroll"
          className="rounded-md bg-[#83CD2D] px-4 py-2 text-sm font-medium text-white shadow-sm transition-opacity hover:opacity-90"
        >
          + Neue Anmeldung
        </Link>
      </header>

      {groups.length === 0 ? (
        <div className="rounded-2xl border border-gray-200 bg-white p-8 text-center text-sm text-gray-600 shadow-sm">
          Sobald eine Anmeldung an einer Schule bestätigt wurde, erscheint Ihr
          Kind hier.
        </div>
      ) : (
        <div className="space-y-4">
          {groups.map((g) => (
            <SchoolGroup
              key={g.schoolSlug}
              schoolName={g.schoolName}
              students={g.children}
            />
          ))}
        </div>
      )}
    </div>
  );
}
