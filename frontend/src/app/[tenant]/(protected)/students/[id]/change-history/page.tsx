"use client";

import React, { Suspense, useCallback, useEffect, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  resolveStudentReferrer,
  studentHistoryBack,
} from "../student-referrer";
import { useApiLoadError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import {
  fetchStudent,
  fetchStudentChangeHistory,
  type StudentFieldChange,
} from "~/lib/student-api";
import type { Student } from "~/lib/student-helpers";
import { useStudentHistoryBreadcrumb } from "~/lib/breadcrumb-context";
import { useScrollToTop } from "~/lib/hooks/use-scroll-to-top";
import { createLogger } from "~/lib/logger";
import { formatDate, formatTime } from "~/lib/date-helpers";
import { ChangeHistorySkeleton } from "./page-skeleton";
import { changeHistoryConcept } from "~/lib/concepts/changeHistory";

const logger = createLogger({ component: "StudentChangeHistoryPage" });

// ─── Types ───────────────────────────────────────────────────────────────────

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
  // Krank oder entschuldigt, von der Schulverwaltung in moto schule
  // nachgetragen (#4791).
  school_absence_report: "Meldung der Schulverwaltung",
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

// ─── Table ───────────────────────────────────────────────────────────────────

const CHANGE_COLUMNS: DataTableColumn<StudentFieldChange>[] = [
  {
    key: "changedAt",
    header: "Zeitpunkt",
    className: "whitespace-nowrap tabular-nums",
    stacked: "meta",
    render: (entry) => (
      <span className="text-gray-600">
        {formatDate(entry.changed_at)}
        <span className="ml-1 text-gray-400">
          {formatTime(entry.changed_at)}
        </span>
      </span>
    ),
  },
  {
    key: "editedBy",
    header: "Geändert von",
    className: "whitespace-nowrap font-medium",
    render: (entry) => entry.edited_by,
  },
  {
    key: "field",
    header: "Feld",
    className: "whitespace-nowrap",
    stacked: "title",
    render: (entry) => (
      <span className="text-gray-700">{fieldLabel(entry.field)}</span>
    ),
  },
  {
    key: "old",
    header: "Vorher",
    className: "max-w-xs",
    render: (entry) => (
      <span className="text-gray-500">
        <ValueText value={entry.old_value} />
      </span>
    ),
  },
  {
    key: "new",
    header: "Nachher",
    className: "max-w-xs",
    render: (entry) => (
      <span className="text-gray-700">
        <ValueText value={entry.new_value} />
      </span>
    ),
  },
];

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
  const referrer = resolveStudentReferrer(searchParams.get("from"));

  const [student, setStudent] = useState<Student | null>(null);
  const [entries, setEntries] = useState<StudentFieldChange[] | null>(null);
  const [loading, setLoading] = useState(true);
  // A 404 is its own state, not an error to retry.
  const [notFound, setNotFound] = useState(false);
  const historyLoad = useApiLoadError();
  const showHistoryError = historyLoad.show;
  const clearHistoryError = historyLoad.clear;
  // Bumped by „Wiederholen“ to load the page again.
  const [attempt, setAttempt] = useState(0);

  useStudentHistoryBreadcrumb({ studentName: student?.name, referrer });
  useScrollToTop(studentId);

  const loadStudent = useCallback(async (): Promise<Student | null> => {
    try {
      return await fetchStudent(studentId);
    } catch (err) {
      // The name only decorates the header; without it the page still shows
      // the history under its generic title.
      logger.error("student_fetch_failed", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      return null;
    }
  }, [studentId]);

  const fetchHistory = useCallback(async (): Promise<void> => {
    try {
      setEntries(await fetchStudentChangeHistory(studentId));
      setNotFound(false);
      clearHistoryError();
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setNotFound(true);
        setEntries(null);
        clearHistoryError();
        return;
      }
      logger.error("change_history_fetch_failed", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      setEntries(null);
      await showHistoryError(err, {
        object: "die Liste der Änderungen",
        retry: () => setAttempt((current) => current + 1),
      });
    }
  }, [clearHistoryError, showHistoryError, studentId]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);

    void (async () => {
      const [s] = await Promise.all([loadStudent(), fetchHistory()]);
      if (cancelled) return;
      setStudent(s);
      setLoading(false);
    })();

    return () => {
      cancelled = true;
    };
  }, [loadStudent, fetchHistory, attempt]);

  const displayName = student
    ? (student.name ?? `${student.first_name} ${student.second_name}`)
    : "";
  // Statuszeile: Klasse, Gruppe und die Zahl der geladenen Einträge. Beides
  // steht schon im Datensatz der Seite, es entsteht kein weiterer Request.
  // Ohne geladene Einträge steht keine "0 Einträge" neben dem Ladefehler
  // (#2517).
  const entryCount = entries?.length ?? 0;
  const entryLine = entries
    ? `${entryCount} ${entryCount === 1 ? "Eintrag" : "Einträge"}`
    : null;
  const studentMeta = student
    ? [student.school_class, student.group_name, entryLine]
        .filter(Boolean)
        .join(" · ")
    : "";
  const failed = notFound || historyLoad.error !== null;
  const back = studentHistoryBack(studentId, referrer, failed);

  return (
    <>
      {/* Der Entitätskopf ist die Kopfkarte der Seite. */}
      <TenantPage
        back
        {...back}
        leading={<ConceptIconTile concept={changeHistoryConcept} size="page" />}
        title={displayName || "Änderungsverlauf"}
        stats={studentMeta || entryLine}
        statsLoading={loading}
        loading={loading}
        error={notFound ? "Kind nicht gefunden." : historyLoad.error}
        empty={
          !loading && !failed && entryCount === 0
            ? {
                title: "Noch keine Änderungen erfasst.",
                description:
                  "Sobald jemand Angaben zu diesem Kind ändert, erscheint der Eintrag hier.",
              }
            : null
        }
      >
        <SectionCard
          title="Änderungsverlauf"
          description="Wer hat wann welche Angaben zu diesem Kind geändert"
          concept={changeHistoryConcept}
        >
          <DataTable
            columns={CHANGE_COLUMNS}
            rows={entries ?? []}
            getRowKey={(entry) => entry.id}
            stackedOnMobile
          />
        </SectionCard>
      </TenantPage>
    </>
  );
}
