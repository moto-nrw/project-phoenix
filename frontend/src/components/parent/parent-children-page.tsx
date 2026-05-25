"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { ArrowRight, Users } from "lucide-react";
import { type Child, listMyChildren } from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ParentChildrenPage" });

function formatDate(iso: string | undefined): string {
  if (!iso) return "Noch offen";
  return new Intl.DateTimeFormat("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  }).format(new Date(iso));
}

function formatServiceRange(child: Child): string {
  if (!child.enrolled_from && !child.enrolled_until) return "Zeitraum offen";
  return `${formatDate(child.enrolled_from)} bis ${formatDate(child.enrolled_until)}`;
}

function getInitials(child: Child): string {
  return `${child.first_name.at(0) ?? ""}${child.last_name.at(0) ?? ""}`.toUpperCase();
}

export function ParentChildrenPage() {
  const [children, setChildren] = useState<Child[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setChildren(await listMyChildren());
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.warn("parent_children_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) {
    return <ParentChildrenSkeleton />;
  }

  if (error) {
    return (
      <div className="mx-auto max-w-7xl">
        <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-5 text-sm text-[#CC2626] shadow-sm">
          Die Kinderübersicht konnte nicht geladen werden. Bitte aktualisieren
          Sie die Seite oder versuchen Sie es später erneut.
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <section className="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
        <div className="p-5 sm:p-6 lg:p-8">
          <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
            Meine Kinder
          </p>
          <h1 className="mt-1 text-3xl font-semibold text-gray-900 sm:text-4xl">
            Kinderübersicht
          </h1>
          <p className="mt-3 max-w-3xl text-sm leading-6 text-gray-600 sm:text-base">
            Wählen Sie ein Kind aus, um Betreuung, Abholberechtigte und
            Neuigkeiten zu öffnen.
          </p>
        </div>
      </section>

      <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
        {children.length === 0 ? (
          <EmptyChildren />
        ) : (
          <div className="grid gap-3 lg:grid-cols-2">
            {children.map((child) => (
              <ChildCard key={child.student_id} child={child} />
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

function ChildCard({ child }: Readonly<{ child: Child }>) {
  const name = `${child.first_name} ${child.last_name}`;
  return (
    <Link
      href={`/parents/children/${child.student_id}`}
      className="group rounded-2xl border border-gray-200 bg-gray-50/70 p-4 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      <div className="flex items-center gap-4">
        <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-[#83CD2D]/15 text-base font-semibold text-[#4A7A15]">
          {getInitials(child) || (
            <Users className="h-6 w-6" aria-hidden="true" />
          )}
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0">
              <h2 className="text-base font-semibold break-words text-gray-900">
                {name}
              </h2>
              <p className="text-sm break-words text-gray-600">
                {child.school_name}
                {child.school_class ? `, ${child.school_class}` : ""}
              </p>
            </div>
          </div>
          <p className="mt-2 text-sm leading-5 break-words text-gray-500">
            Betreuung {formatServiceRange(child)}
          </p>
        </div>
        <ArrowRight
          className="hidden h-4 w-4 shrink-0 text-gray-400 transition-colors group-hover:text-gray-700 sm:block"
          aria-hidden="true"
        />
      </div>
    </Link>
  );
}

function EmptyChildren() {
  return (
    <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 p-8 text-center">
      <span className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-white text-gray-500 shadow-sm">
        <Users className="h-5 w-5" aria-hidden="true" />
      </span>
      <h2 className="mt-3 text-sm font-semibold text-gray-900">
        Noch keine Kinder
      </h2>
      <p className="mt-1 text-sm leading-6 text-gray-600">
        Sobald ein Kind bestätigt ist, erscheint es hier.
      </p>
    </div>
  );
}

function ParentChildrenSkeleton() {
  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <div className="h-64 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm" />
      <div className="grid gap-3 lg:grid-cols-2">
        {[0, 1, 2, 3].map((item) => (
          <div
            key={item}
            className="h-32 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm"
          />
        ))}
      </div>
    </div>
  );
}
