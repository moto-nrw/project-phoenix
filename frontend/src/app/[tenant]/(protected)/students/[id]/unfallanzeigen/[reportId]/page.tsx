"use client";

import { useEffect, useRef, useState } from "react";
import { useParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { AccidentReportCreateModal } from "~/components/students/accident-report-create-modal";
import {
  AccidentReportFormFields,
  accidentReportDraftErrors,
  draftFromReport,
  fieldsFromDraft,
  type AccidentReportDraft,
} from "~/components/students/accident-report-form";
import { accidentReportPath } from "~/components/students/dokumente-tab";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import {
  SignaturePad,
  type SignatureValue,
} from "~/components/ui/signature-pad";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { formatDate, formatTime } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  studentAccidentReportsService,
  type AccidentReportDetails,
} from "~/lib/student-accident-reports-api";
import {
  studentDocumentsService,
  type StudentDocument,
  type StudentDocumentList,
} from "~/lib/student-documents-api";
import { useSWRAuth } from "~/lib/swr";
import { useTenantRouter } from "~/lib/tenant-router";

// Unfallanzeige (#4771): one accident report of a child. The page shows what
// was recorded, lets the leadership sign it (drawn signature or the signed
// paper copy) and offers the PDF in the layout of the official form. A signed
// report never changes again; „Korrektur anlegen“ records a new version.

const logger = createLogger({ component: "AccidentReportPage" });

function stamp(iso: string | null): string {
  if (!iso) return "–";
  return `${formatDate(iso)}, ${formatTime(iso)} Uhr`;
}

function orDash(value: string): string {
  return value.trim() ? value : "–";
}

export default function AccidentReportPage() {
  const params = useParams();
  const studentId = params.id as string;
  const reportId = params.reportId as string;
  const router = useTenantRouter();
  const { data: session } = useSession();
  const canManage = hasPermission(session, "student_accidents:manage");

  const { data, isLoading, error, mutate } = useSWRAuth<StudentDocumentList>(
    `student-documents-${studentId}`,
    () => studentDocumentsService.list(studentId),
  );
  const load = useApiLoadError();
  const showLoadError = load.show;
  const clearLoadError = load.clear;
  useEffect(() => {
    if (error) {
      void showLoadError(error, {
        object: "die Unfallanzeige",
        retry: () => void mutate(),
      });
    } else {
      clearLoadError();
    }
  }, [error, mutate, showLoadError, clearLoadError]);

  const document: StudentDocument | undefined = data?.documents.find(
    (doc) => doc.id === reportId && doc.accidentReport,
  );
  const report = document?.accidentReport ?? null;
  const backHref = `/students/${studentId}?tab=dokumente`;

  const [editing, setEditing] = useState<AccidentReportDraft | null>(null);
  const [correcting, setCorrecting] = useState(false);

  return (
    <TenantPage
      title={
        report?.supersedesId ? "Unfallanzeige (Korrektur)" : "Unfallanzeige"
      }
      back
      backHref={backHref}
      backLabel="Zurück zum Kinderprofil"
      loading={isLoading}
      error={
        load.error ??
        (data && !report
          ? "Diese Unfallanzeige gibt es nicht mehr. Sie finden alle Unfallanzeigen im Kinderprofil unter Dokumente."
          : null)
      }
      stats={
        report ? (
          <TenantPageStats
            items={[
              { value: stamp(report.occurredAt), label: "Unfall" },
              ...(report.version > 0
                ? [{ value: String(report.version), label: "Fassung" }]
                : []),
            ]}
          />
        ) : undefined
      }
      actions={
        report ? (
          <a
            href={studentAccidentReportsService.pdfUrl(studentId, reportId)}
            className="inline-flex"
          >
            <Button type="button" variant="secondary" size="md" tabIndex={-1}>
              PDF herunterladen
            </Button>
          </a>
        ) : undefined
      }
      overlays={
        report && canManage && report.status === "signed" ? (
          <AccidentReportCreateModal
            isOpen={correcting}
            onClose={() => setCorrecting(false)}
            studentId={studentId}
            correctsReportId={reportId}
            initialDraft={draftFromReport(report)}
            onCreated={(id) => {
              setCorrecting(false);
              void mutate();
              router.push(accidentReportPath(studentId, id));
            }}
          />
        ) : undefined
      }
    >
      {report && document && (
        <div className="space-y-6">
          {report.supersededById && (
            <Alert
              type="info"
              message="Zu dieser Unfallanzeige gibt es eine Korrektur. Diese Fassung bleibt unverändert erhalten."
              action={
                <ButtonLink
                  href={accidentReportPath(studentId, report.supersededById)}
                  variant="secondary"
                  size="md"
                >
                  Korrektur öffnen
                </ButtonLink>
              }
            />
          )}
          <SignatureSection
            studentId={studentId}
            reportId={reportId}
            document={document}
            report={report}
            canManage={canManage && !editing}
            onSigned={() => void mutate()}
            onCorrect={() => setCorrecting(true)}
          />
          {editing ? (
            <EditReportSection
              studentId={studentId}
              reportId={reportId}
              draft={editing}
              onDraftChange={setEditing}
              onDone={() => {
                setEditing(null);
                void mutate();
              }}
              onCancel={() => setEditing(null)}
            />
          ) : (
            <ReportDetails
              report={report}
              onEdit={
                canManage && report.status === "open"
                  ? () => setEditing(draftFromReport(report))
                  : undefined
              }
            />
          )}
        </div>
      )}
    </TenantPage>
  );
}

function ReportDetails({
  report,
  onEdit,
}: Readonly<{ report: AccidentReportDetails; onEdit?: () => void }>) {
  return (
    <SectionCard
      title="Angaben zum Unfall"
      description="Die Angaben folgen dem Vordruck der Unfallkasse."
      action={
        onEdit ? (
          <Button type="button" variant="secondary" size="md" onClick={onEdit}>
            Ändern
          </Button>
        ) : undefined
      }
    >
      <DataGrid>
        <DataField label="Unfallzeitpunkt">
          {stamp(report.occurredAt)}
        </DataField>
        <DataField label="Unfallort">{report.location}</DataField>
        <DataField label="Was ist passiert?" fullWidth>
          <span className="whitespace-pre-line">{report.description}</span>
        </DataField>
        <DataField label="Verletzte Körperteile">
          {orDash(report.injuredBodyParts)}
        </DataField>
        <DataField label="Art der Verletzung">{report.injuryKind}</DataField>
        <DataField label="Erste Hilfe">{orDash(report.firstAid)}</DataField>
        <DataField label="Arzt oder Krankenhaus">
          {report.doctorVisited ? report.doctor : "Kein Arztbesuch"}
        </DataField>
        <DataField label="Zeugen">{orDash(report.witnesses)}</DataField>
        <DataField label="Benachrichtigte Eltern">
          {report.parentsNotified
            ? `${report.parentsNotified}${
                report.parentsNotifiedAt
                  ? `, ${stamp(report.parentsNotifiedAt)}`
                  : ""
              }`
            : "Noch nicht benachrichtigt"}
        </DataField>
        <DataField label="Aufsichtsperson">{report.supervisor}</DataField>
      </DataGrid>
    </SectionCard>
  );
}

function EditReportSection({
  studentId,
  reportId,
  draft,
  onDraftChange,
  onDone,
  onCancel,
}: Readonly<{
  studentId: string;
  reportId: string;
  draft: AccidentReportDraft;
  onDraftChange: (draft: AccidentReportDraft) => void;
  onDone: () => void;
  onCancel: () => void;
}>) {
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formError = useApiFormError(formRef);

  const save = async () => {
    const errors = accidentReportDraftErrors(draft);
    if (Object.keys(errors).length > 0) {
      formError.invalid("Bitte füllen Sie die markierten Felder aus.", errors);
      return;
    }
    setSaving(true);
    formError.clear();
    try {
      await studentAccidentReportsService.update(
        studentId,
        reportId,
        fieldsFromDraft(draft),
      );
      onDone();
    } catch (err) {
      logger.error("accident_report_update_failed", {
        student_id: studentId,
        report_id: reportId,
        error: err instanceof Error ? err.message : String(err),
      });
      await formError.show(err, { object: "die Unfallanzeige" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard title="Angaben zum Unfall">
      <form
        ref={formRef}
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
        className="space-y-4"
      >
        <FormErrorAlert message={formError.error} />
        <AccidentReportFormFields
          draft={draft}
          onChange={onDraftChange}
          fieldError={formError.fieldError}
          disabled={saving}
        />
        <EditActions onCancel={onCancel} saving={saving} saveType="submit" />
      </form>
    </SectionCard>
  );
}

function SignatureSection({
  studentId,
  reportId,
  document,
  report,
  canManage,
  onSigned,
  onCorrect,
}: Readonly<{
  studentId: string;
  reportId: string;
  document: StudentDocument;
  report: AccidentReportDetails;
  canManage: boolean;
  onSigned: () => void;
  onCorrect: () => void;
}>) {
  const [signature, setSignature] = useState<SignatureValue | null>(null);
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);
  const sectionRef = useRef<HTMLDivElement>(null);
  const formError = useApiFormError(sectionRef);

  const run = async (action: () => Promise<void>, failure: string) => {
    setBusy(true);
    formError.clear();
    try {
      await action();
      setSignature(null);
      onSigned();
    } catch (err) {
      logger.error(failure, {
        student_id: studentId,
        report_id: reportId,
        error: err instanceof Error ? err.message : String(err),
      });
      await formError.show(err, { object: "die Unterschrift" });
    } finally {
      setBusy(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  };

  if (report.status === "signed") {
    return (
      <SectionCard
        title="Unterschrift"
        action={
          canManage && !report.supersededById ? (
            <Button
              type="button"
              variant="secondary"
              size="md"
              onClick={onCorrect}
            >
              Korrektur anlegen
            </Button>
          ) : undefined
        }
      >
        <div className="flex flex-wrap items-center gap-3">
          <StatusBadge tone="green" label="Unterschrieben" />
          <span className="text-sm text-gray-700">
            {report.signedDigitally
              ? `Digital unterschrieben am ${stamp(report.signedAt)}.`
              : `Papierexemplar hochgeladen am ${stamp(report.signedAt)}.`}
          </span>
          {document.hasFile && (
            <a
              href={studentDocumentsService.downloadUrl(studentId, reportId)}
              className="text-moto-blue text-sm font-medium underline-offset-2 hover:underline"
            >
              Papierexemplar herunterladen
            </a>
          )}
        </div>
        <p className="mt-3 text-sm text-gray-600">
          Die Unfallanzeige lässt sich nicht mehr ändern. Bei einem Fehler legen
          Sie eine Korrektur an.
        </p>
      </SectionCard>
    );
  }

  return (
    <SectionCard
      title="Unterschrift"
      description={
        canManage
          ? "Die Leitung unterschreibt die Unfallanzeige. Danach lässt sie sich nicht mehr ändern."
          : "Die Leitung unterschreibt die Unfallanzeige."
      }
    >
      <div ref={sectionRef} className="space-y-4">
        <div className="flex items-center gap-3">
          <StatusBadge tone="orange" label="Offen" />
          <span className="text-sm text-gray-700">
            Noch nicht unterschrieben.
          </span>
        </div>
        <FormErrorAlert message={formError.error} />
        {canManage && (
          <>
            <SignaturePad
              id="unfall-unterschrift"
              value={signature}
              onChange={setSignature}
              disabled={busy}
            />
            <Button
              type="button"
              variant="success"
              size="md"
              disabled={busy || !signature}
              onClick={() => {
                if (!signature) return;
                void run(
                  () =>
                    studentAccidentReportsService.sign(
                      studentId,
                      reportId,
                      signature,
                    ),
                  "accident_report_sign_failed",
                );
              }}
            >
              {busy ? "Wird gespeichert…" : "Unterschreiben"}
            </Button>
            <div className="border-t border-gray-100 pt-4">
              <p className="text-sm text-gray-600">
                Oder: Drucken Sie das PDF aus, lassen Sie es unterschreiben und
                laden Sie es hier hoch.
              </p>
              <Button
                type="button"
                variant="ghost"
                size="compact"
                className="text-moto-blue mt-2"
                disabled={busy}
                onClick={() => fileRef.current?.click()}
              >
                Unterschriebenes Papierexemplar hochladen
              </Button>
              <input
                ref={fileRef}
                type="file"
                accept=".pdf,.png,.jpg,.jpeg"
                className="hidden"
                aria-label="Unterschriebenes Papierexemplar auswählen"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (!file) return;
                  void run(
                    () =>
                      studentAccidentReportsService.uploadPaperCopy(
                        studentId,
                        reportId,
                        file,
                      ),
                    "accident_report_paper_upload_failed",
                  );
                }}
              />
            </div>
          </>
        )}
      </div>
    </SectionCard>
  );
}
