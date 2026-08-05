"use client";

import { useCallback, useState } from "react";
import type { ReactNode } from "react";
import { Download, FileText, FileSpreadsheet, FileType2 } from "lucide-react";
import {
  type AdminRequestSummary,
  type EnrollmentRequestExportFormat,
  exportStudentEnrollmentRequests,
  listStudentEnrollmentRequests,
} from "~/lib/enrollment-admin-api";
import { useSWRAuth } from "~/lib/swr";
import { useToast } from "~/contexts/ToastContext";
import { Button } from "~/components/ui/button";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";
import {
  ChildExtraFields,
  ChildOfferingAdjustment,
  ChildOfferings,
  RequestExtraSection,
  formatDateTime,
  formatPlainDate,
} from "~/components/enrollment/admin-enrollment-detail";
import { ChildStatusBadge } from "~/components/enrollment/child-status-badge";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentEnrollmentsTab" });

const EXPORT_FORMATS: Array<{
  format: EnrollmentRequestExportFormat;
  label: string;
  icon: ReactNode;
}> = [
  { format: "pdf", label: "PDF", icon: <FileText className="h-4 w-4" /> },
  { format: "docx", label: "DOCX", icon: <FileType2 className="h-4 w-4" /> },
  {
    format: "xlsx",
    label: "XLSX",
    icon: <FileSpreadsheet className="h-4 w-4" />,
  },
];

interface StudentEnrollmentsTabProps {
  studentId: string;
}

export function StudentEnrollmentsTab({
  studentId,
}: StudentEnrollmentsTabProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [exporting, setExporting] =
    useState<EnrollmentRequestExportFormat | null>(null);

  const {
    data: requests = [],
    error,
    isLoading,
    mutate,
  } = useSWRAuth(`student-enrollments-${studentId}`, () =>
    listStudentEnrollmentRequests(studentId),
  );

  const handleExport = useCallback(
    async (format: EnrollmentRequestExportFormat) => {
      setExporting(format);
      try {
        await exportStudentEnrollmentRequests(studentId, format);
        toastSuccess("Export wurde erstellt.");
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Export fehlgeschlagen";
        logger.error("student_enrollments_export_failed", {
          error: message,
          student_id: studentId,
          format,
        });
        toastError(message);
      } finally {
        setExporting(null);
      }
    },
    [studentId, toastError, toastSuccess],
  );

  return (
    <section className="moto-content-surface rounded-2xl border p-4 shadow-sm backdrop-blur-sm sm:p-6">
      <ConceptSectionHeader
        className="mb-4"
        title="Anmeldungen"
        concept="enrollments"
        subtitle="Angaben aus der Halbjahresanmeldung, ohne Erziehungsberechtigte."
        actions={
          !isLoading && !error && requests.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {EXPORT_FORMATS.map((item) => (
                <Button
                  key={item.format}
                  type="button"
                  variant="outline"
                  size="compact"
                  disabled={exporting !== null}
                  onClick={() => void handleExport(item.format)}
                >
                  {exporting === item.format ? (
                    <Download className="h-4 w-4 animate-pulse" aria-hidden />
                  ) : (
                    item.icon
                  )}
                  {item.label}
                </Button>
              ))}
            </div>
          ) : undefined
        }
      />

      {isLoading ? (
        <p className="py-6 text-center text-sm text-gray-500">
          Anmeldungen werden geladen…
        </p>
      ) : error ? (
        <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-lg border p-3 text-sm">
          Anmeldungen konnten nicht geladen werden.
        </div>
      ) : requests.length === 0 ? (
        <div className="rounded-xl border border-gray-200 bg-gray-50/70 p-4 text-sm text-gray-600">
          Für dieses Kind ist keine angenommene oder übernommene
          Online-Anmeldung verknüpft.
        </div>
      ) : (
        <div className="space-y-4">
          {requests.map((request) => (
            <EnrollmentRequestCard
              key={request.id}
              request={request}
              onChanged={() => void mutate()}
            />
          ))}
        </div>
      )}
    </section>
  );
}

function EnrollmentRequestCard({
  onChanged,
  request,
}: Readonly<{ request: AdminRequestSummary; onChanged: () => void }>) {
  const submittedAt = formatDateTime(request.submitted_at, {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });

  return (
    <article className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      <div className="flex flex-col gap-3 border-b border-gray-100 p-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
            Anmeldung
          </p>
          <h4 className="mt-1 text-base font-semibold text-gray-900">
            {request.phase_name || "Nicht zugeordnet"}
          </h4>
          <p className="mt-1 text-sm text-gray-600">
            Eingegangen am {submittedAt}
          </p>
        </div>
        <span className="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-700">
          {request.children.length} Kind
          {request.children.length === 1 ? "" : "er"}
        </span>
      </div>

      <div className="space-y-4 p-4">
        {request.children.map((child) => (
          <section
            key={child.id}
            className="rounded-xl border border-gray-100 bg-gray-50/70 p-3"
          >
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h5 className="text-sm font-semibold text-gray-900">
                  {child.first_name} {child.last_name}
                </h5>
                <div className="mt-2 flex flex-wrap gap-2 text-xs text-gray-600">
                  <span className="rounded-full bg-white px-2.5 py-1 font-medium text-gray-700">
                    Geburtsdatum: {formatPlainDate(child.date_of_birth)}
                  </span>
                  {child.target_grade_level ? (
                    <span className="rounded-full bg-white px-2.5 py-1 font-medium text-gray-700">
                      {child.target_grade_level}. Klasse
                    </span>
                  ) : null}
                </div>
              </div>
              <ChildStatusBadge status={child.status} />
            </div>

            {child.status_reason ? (
              <div className="mt-3 rounded-lg border border-gray-100 bg-white px-3 py-2 text-sm text-gray-700">
                <span className="text-xs font-medium text-gray-500">
                  Begründung:{" "}
                </span>
                {child.status_reason}
              </div>
            ) : null}

            <ChildOfferings offerings={child.offerings} />
            {child.status === "approved" ? (
              <ChildOfferingAdjustment
                requestId={request.id}
                phaseId={request.phase_id}
                child={child}
                onSaved={onChanged}
              />
            ) : null}
            <ChildExtraFields
              child={child}
              schemaFields={request.schema_fields}
            />
          </section>
        ))}

        <RequestExtraSection request={request} />
      </div>
    </article>
  );
}
