"use client";

import React, { Suspense, useCallback, useEffect, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { BackButton } from "~/components/ui/back-button";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useStudentHistoryBreadcrumb } from "~/lib/breadcrumb-context";
import { useScrollToTop } from "~/lib/hooks/use-scroll-to-top";
import { createLogger } from "~/lib/logger";
import { formatDate, formatTime } from "~/lib/date-helpers";
import { ChangeHistorySkeleton } from "./page-skeleton";

const logger = createLogger({ component: "StudentChangeHistoryPage" });

// ─── Types ───────────────────────────────────────────────────────────────────

interface Student {
  id: string;
  first_name: string;
  second_name: string;
  name?: string;
  school_class: string;
  group_id?: string;
  group_name?: string;
}

interface ChangeEntry {
  id: string;
  field: string;
  old_value: string;
  new_value: string;
  edited_by: string;
  changed_at: string;
}

type ErrorCode = "forbidden" | "not_found" | "generic";

/** Unterzeile der Kopfkarte, wenn weder Klasse noch Gruppe bekannt sind. */
const CHANGE_HISTORY_DESCRIPTION =
  "Wer hat wann welche Angaben zu diesem Kind geändert.";

const ERROR_MESSAGES: Record<ErrorCode, string> = {
  forbidden:
    "Sie können den Änderungsverlauf nur für Kinder Ihrer betreuten Gruppen einsehen.",
  not_found: "Kind nicht gefunden.",
  generic: "Fehler beim Laden des Änderungsverlaufs.",
};

// German labels for the tracked fields. The backend stores the raw field token
// and already-formatted German values; only the field name is mapped here.
const FIELD_LABELS: Record<string, string> = {
  status: "Status",
  supervisor_notes: "Betreuer-Notizen",
  extra_info: "Zusatzinformationen",
  health_info: "Gesundheitsinformationen",
  pickup_status: "Abholung",
  pickup_schedule: "Dauerhafte Gehzeiten",
  departure_days: "Wochenplan (Abholung)",
  departure_companion_note: "Geht außerdem mit",
  document: "Dokument",
};

// Document rows carry their category in the field name ("document_attest") so
// the backend can filter the history by the same per-category permissions the
// Dokumente tab uses. The readable category is already part of the value, so
// every variant renders under one label.
function fieldLabel(field: string): string {
  if (field.startsWith("document_")) {
    return "Dokument";
  }
  return FIELD_LABELS[field] ?? field;
}

// ─── Value cell ──────────────────────────────────────────────────────────────

function ValueText({ value }: { readonly value: string }) {
  if (value.trim() === "") {
    return <span className="text-gray-400 italic">leer</span>;
  }
  return <span className="break-words whitespace-pre-wrap">{value}</span>;
}

// ─── Page ────────────────────────────────────────────────────────────────────

export default function StudentChangeHistoryPage() {
  return (
    <Suspense fallback={<ChangeHistorySkeleton />}>
      <StudentChangeHistoryPageContent />
    </Suspense>
  );
}

function StudentChangeHistoryPageContent() {
  const params = useParams();
  const searchParams = useSearchParams();
  const studentId = params.id as string;
  const referrer = searchParams.get("from") ?? "/students/search";

  const [student, setStudent] = useState<Student | null>(null);
  const [entries, setEntries] = useState<ChangeEntry[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [errorCode, setErrorCode] = useState<ErrorCode | null>(null);

  useStudentHistoryBreadcrumb({ studentName: student?.name, referrer });
  useScrollToTop(studentId);

  const fetchStudent = useCallback(async (): Promise<Student | null> => {
    try {
      const res = await fetch(`/api/students/${studentId}`);
      if (!res.ok) return null;
      const body = (await res.json()) as { data?: Student };
      return body.data ?? null;
    } catch (err) {
      logger.error("student_fetch_failed", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      return null;
    }
  }, [studentId]);

  const fetchHistory = useCallback(async (): Promise<void> => {
    try {
      const res = await fetch(`/api/students/${studentId}/change-history`);
      if (res.status === 403) {
        setErrorCode("forbidden");
        setEntries(null);
        return;
      }
      if (res.status === 404) {
        setErrorCode("not_found");
        return;
      }
      if (!res.ok) {
        setErrorCode("generic");
        return;
      }
      const body = (await res.json()) as { data: ChangeEntry[] };
      setEntries(body.data ?? []);
      setErrorCode(null);
    } catch (err) {
      logger.error("change_history_fetch_failed", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      setErrorCode("generic");
    }
  }, [studentId]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);

    void (async () => {
      const [s] = await Promise.all([fetchStudent(), fetchHistory()]);
      if (cancelled) return;
      setStudent(s);
      setLoading(false);
    })();

    return () => {
      cancelled = true;
    };
  }, [fetchStudent, fetchHistory]);

  const displayName = student
    ? (student.name ?? `${student.first_name} ${student.second_name}`)
    : "";
  // Statuszeile: Klasse, Gruppe und die Zahl der geladenen Einträge. Beides
  // steht schon im Datensatz der Seite, es entsteht kein weiterer Request.
  const entryCount = entries?.length ?? 0;
  const studentMeta = student
    ? [
        student.school_class,
        student.group_name,
        `${entryCount} ${entryCount === 1 ? "Eintrag" : "Einträge"}`,
      ]
        .filter(Boolean)
        .join(" · ")
    : "";
  // Im Fehlerfall führt der Rückweg auf die Liste, sonst auf die Kindakte in
  // den Reiter, aus dem diese Unterseite geöffnet wurde.
  const backReferrer =
    errorCode !== null
      ? referrer
      : `/students/${studentId}?from=${referrer}&tab=historie`;

  return (
    <>
      {/* Mobiler Rückweg; auf dem Desktop führt die Breadcrumb zurück. */}
      <BackButton referrer={backReferrer} />

      {/* Der Entitätskopf ist die Kopfkarte der Seite. */}
      <TenantPage
        leading={<ConceptIconTile concept="changeHistory" variant="page" />}
        title={displayName || "Änderungsverlauf"}
        stats={studentMeta || CHANGE_HISTORY_DESCRIPTION}
        statsLoading={loading}
        loading={loading}
        error={errorCode !== null ? ERROR_MESSAGES[errorCode] : null}
        empty={
          !loading && errorCode === null && entryCount === 0
            ? {
                title: "Noch keine Änderungen erfasst.",
                description:
                  "Sobald jemand Angaben zu diesem Kind ändert, erscheint der Eintrag hier.",
              }
            : null
        }
      >
        {/* bodyClassName hebt die Kartenpolsterung auf: die Tabelle bringt ihre
            eigene Zellpolsterung mit und läuft bis zur Kartenkante. */}
        <SectionCard
          title="Änderungsverlauf"
          description="Wer hat wann welche Angaben zu diesem Kind geändert"
          leading={
            <ConceptIconTile concept="changeHistory" variant="section" />
          }
          bodyClassName="mt-4 -mx-5 -mb-5"
        >
          <>
            {/* Desktop table */}
            <div className="hidden md:block">
              <table className="w-full">
                <thead>
                  <tr className="border-b border-gray-100 text-left text-xs font-medium text-gray-500">
                    <th className="px-6 py-3">Zeitpunkt</th>
                    <th className="px-6 py-3">Geändert von</th>
                    <th className="px-6 py-3">Feld</th>
                    <th className="px-6 py-3">Vorher</th>
                    <th className="px-6 py-3">Nachher</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-50">
                  {(entries ?? []).map((entry) => (
                    <tr key={entry.id} className="align-top text-sm">
                      <td className="px-6 py-3 whitespace-nowrap text-gray-600 tabular-nums">
                        {formatDate(entry.changed_at)}
                        <span className="ml-1 text-gray-400">
                          {formatTime(entry.changed_at)}
                        </span>
                      </td>
                      <td className="px-6 py-3 font-medium whitespace-nowrap text-gray-900">
                        {entry.edited_by}
                      </td>
                      <td className="px-6 py-3 whitespace-nowrap text-gray-700">
                        {fieldLabel(entry.field)}
                      </td>
                      <td className="max-w-xs px-6 py-3 text-gray-500">
                        <ValueText value={entry.old_value} />
                      </td>
                      <td className="max-w-xs px-6 py-3 text-gray-700">
                        <ValueText value={entry.new_value} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Mobile card layout */}
            <div className="divide-y divide-gray-100 md:hidden">
              {(entries ?? []).map((entry) => (
                <div key={entry.id} className="px-4 py-3">
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium text-gray-900">
                      {fieldLabel(entry.field)}
                    </span>
                    <span className="text-xs text-gray-500 tabular-nums">
                      {formatDate(entry.changed_at)}{" "}
                      {formatTime(entry.changed_at)}
                    </span>
                  </div>
                  <div className="mt-1 text-xs text-gray-500">
                    von {entry.edited_by}
                  </div>
                  <div className="mt-2 space-y-1 text-sm">
                    <div className="flex gap-2">
                      <span className="shrink-0 text-xs text-gray-400">
                        Vorher:
                      </span>
                      <span className="text-gray-500">
                        <ValueText value={entry.old_value} />
                      </span>
                    </div>
                    <div className="flex gap-2">
                      <span className="shrink-0 text-xs text-gray-400">
                        Nachher:
                      </span>
                      <span className="text-gray-700">
                        <ValueText value={entry.new_value} />
                      </span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </>
        </SectionCard>
      </TenantPage>
    </>
  );
}
