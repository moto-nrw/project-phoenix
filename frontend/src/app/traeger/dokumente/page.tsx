"use client";

// Dokumente des Trägers für Eltern und Schulen (#4692): Übersicht aller
// hochgeladenen Dokumente mit ihren Freigaben. Von hier aus hochladen und ein
// Dokument öffnen, um es freizugeben.

import { FileTextIcon, UploadSimpleIcon } from "@phosphor-icons/react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import {
  documentFileFacts,
  releaseAudienceLabel,
  schoolNamesOf,
  type SchoolNames,
} from "~/components/traeger/traeger-document-format";
import { TraegerDocumentUploadModal } from "~/components/traeger/traeger-document-upload-modal";
import { useTraegerDocuments } from "~/components/traeger/use-traeger-documents";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { FileTypeIcon } from "~/components/ui/file-type-icon";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import { useToast } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { formatBerlinDate } from "~/lib/date-helpers";
import type { TraegerDocument } from "~/lib/traeger-documents-api";
import { traegerPath } from "~/lib/traeger-url";

/** Das Recht, das Hochladen und Freigeben öffnet (Rolle Dokumente, #5992). */
const MANAGE_RIGHT = "traeger_documents:manage";

function columns(schools: SchoolNames): DataTableColumn<TraegerDocument>[] {
  return [
    {
      key: "title",
      header: "Dokument",
      render: (document) => (
        <span className="flex min-w-0 items-center gap-3">
          <FileTypeIcon
            contentType={document.contentType}
            className="h-5 w-5 shrink-0 text-gray-400"
          />
          <span className="min-w-0">
            <span className="block truncate font-medium text-gray-900">
              {document.title}
            </span>
            {/* On a phone the file name wraps and stays whole; from sm on
                only the name shortens. Type and size never break. */}
            <span className="block text-sm text-gray-500 sm:flex sm:min-w-0">
              <span className="wrap-anywhere sm:truncate">
                {document.fileName}
              </span>
              <span className="whitespace-nowrap sm:shrink-0">
                &nbsp;· {documentFileFacts(document)}
              </span>
            </span>
          </span>
        </span>
      ),
      sortValue: (document) => document.title.toLowerCase(),
      stacked: "title",
    },
    {
      key: "releases",
      header: "Freigegeben für",
      render: (document) =>
        document.releases.length === 0 ? (
          <StatusBadge label="Noch niemand" tone="gray" />
        ) : (
          <span className="block space-y-0.5">
            {document.releases.map((release) => (
              <span key={release.id} className="block text-sm text-gray-700">
                {releaseAudienceLabel(release, schools)}
              </span>
            ))}
          </span>
        ),
      sortValue: (document) => document.releases.length,
      stacked: "field",
    },
    {
      key: "reads",
      header: "Geöffnet",
      render: (document) => (
        <span className="text-sm text-gray-700 tabular-nums">
          {document.reads === 0 ? "–" : `${document.reads}-mal`}
        </span>
      ),
      sortValue: (document) => document.reads,
      stacked: "hidden",
    },
    {
      key: "created",
      header: "Hochgeladen",
      render: (document) => (
        <span className="text-sm text-gray-700 tabular-nums">
          {formatBerlinDate(document.createdAt)}
        </span>
      ),
      sortValue: (document) => document.createdAt,
      stacked: "hidden",
    },
  ];
}

export default function TraegerDocumentsPage() {
  const router = useRouter();
  const { data: session } = useSession();
  const { success } = useToast();
  const canManage = hasPermission(session ?? null, MANAGE_RIGHT);
  const { data, isLoading, loadError, mutate } = useTraegerDocuments();
  // The release column names the schools; without them it would read alike.
  const { schools, error: schoolsError } = useTraegerSchools();
  const schoolsPending = schools === null && !schoolsError;
  const [uploading, setUploading] = useState(false);
  const documents = data ?? [];
  const names = useMemo(() => schoolNamesOf(schools ?? []), [schools]);
  const tableColumns = useMemo(() => columns(names), [names]);
  const released = documents.filter(
    (document) => document.releases.length > 0,
  ).length;

  return (
    <TenantPage
      title="Dokumente"
      info="Hier teilen Sie vorhandene Dokumente und Listen mit Eltern und Schulen, zum Beispiel eine Hausordnung oder einen Elternbrief. Ein hochgeladenes Dokument sieht noch niemand. Erst wenn Sie es freigeben, finden Eltern es in der Eltern-App und die OGS-Leitung in moto unter „Dateien“. Hochladen, freigeben und löschen darf, wer die Rolle „Dokumente“ hat."
      stats={
        documents.length > 0 ? (
          <span className="flex flex-wrap items-center gap-2">
            <span>
              {documents.length === 1
                ? "1 Dokument"
                : `${documents.length} Dokumente`}
            </span>
            <StatusBadge label={`${released} freigegeben`} tone="green" />
          </span>
        ) : null
      }
      statsLoading={isLoading}
      error={loadError ?? schoolsError}
      actions={
        canManage ? (
          <Button
            type="button"
            variant="primary"
            size="md"
            className="gap-2"
            onClick={() => setUploading(true)}
          >
            <UploadSimpleIcon className="h-4 w-4" aria-hidden="true" />
            Dokument hochladen
          </Button>
        ) : undefined
      }
      overlays={
        <TraegerDocumentUploadModal
          isOpen={uploading}
          onClose={() => setUploading(false)}
          onUploaded={(document) => {
            setUploading(false);
            success(`„${document.title}“ ist hochgeladen.`);
            void mutate();
            router.push(traegerPath(`/traeger/dokumente/${document.id}`));
          }}
        />
      }
    >
      <DataTable
        columns={tableColumns}
        rows={documents}
        getRowKey={(document) => document.id}
        isLoading={isLoading || schoolsPending}
        loadingRowCount={3}
        stackedOnMobile
        stackedLayout="row"
        stackedDetailKey="releases"
        onRowClick={(document) =>
          router.push(traegerPath(`/traeger/dokumente/${document.id}`))
        }
        emptyState={
          <EmptyState
            icon={<FileTextIcon className="h-12 w-12" aria-hidden="true" />}
            title="Noch kein Dokument."
            description={
              canManage
                ? "Laden Sie oben ein Dokument hoch. Danach geben Sie es für Eltern oder Schulen frei."
                : "Sobald jemand mit der Rolle „Dokumente“ ein Dokument hochlädt, steht es hier."
            }
          />
        }
      />
    </TenantPage>
  );
}
