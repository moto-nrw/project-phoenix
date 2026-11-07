"use client";

// Rückfrage vor dem Löschen eines Träger-Dokuments (#5993). Sie nennt jede
// Freigabe, die mit dem Dokument endet, sagt, dass auch gespeicherte Links
// nichts mehr öffnen, und was als Nachweis bleibt. Die Datei ist danach weg,
// deshalb verlangt die Rückfrage das eingetippte Wort LÖSCHEN.

import {
  releaseAudienceLabel,
  type SchoolNames,
} from "./traeger-document-format";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { FramedList } from "~/components/ui/framed-list";
import type { FormErrorInput } from "~/components/ui/form-error";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { parentsConcept } from "~/lib/concepts/parents";
import { schoolsConcept } from "~/lib/concepts/schools";
import type { TraegerDocument } from "~/lib/traeger-documents-api";

interface TraegerDocumentDeleteModalProps {
  readonly isOpen: boolean;
  readonly document: Pick<TraegerDocument, "title" | "releases">;
  readonly schools: SchoolNames;
  readonly onConfirm: () => void;
  readonly onClose: () => void;
  readonly loading: boolean;
  readonly error: FormErrorInput;
}

export function TraegerDocumentDeleteModal({
  isOpen,
  document,
  schools,
  onConfirm,
  onClose,
  loading,
  error,
}: TraegerDocumentDeleteModalProps) {
  const released = document.releases.length > 0;
  return (
    <ConfirmDeleteModal
      isOpen={isOpen}
      onClose={onClose}
      onConfirm={onConfirm}
      title="Dokument löschen?"
      description={
        <div className="space-y-3">
          <p>
            <span className="font-medium text-gray-900">
              „{document.title}“
            </span>{" "}
            wird gelöscht. Danach ist die Datei weg, auch hier im Träger-Büro.
          </p>
          {released ? (
            <div className="space-y-2">
              <FramedList
                maxHeight="sm"
                header={
                  <p className="text-sm font-medium text-gray-900">
                    Diese Freigaben enden sofort
                  </p>
                }
              >
                {document.releases.map((release) => (
                  <li
                    key={release.id}
                    className="flex items-center gap-3 px-4 py-2.5"
                  >
                    <MotoConceptIcon
                      concept={
                        release.audience === "parents"
                          ? parentsConcept
                          : schoolsConcept
                      }
                      size={18}
                      className="shrink-0"
                    />
                    <span className="min-w-0 text-sm text-gray-900">
                      {releaseAudienceLabel(release, schools)}
                    </span>
                  </li>
                ))}
              </FramedList>
              <p>
                Eltern und Schulen finden das Dokument dann nicht mehr. Auch ein
                gespeicherter Link öffnet es nicht mehr.
              </p>
            </div>
          ) : (
            <p>Das Dokument ist für niemanden freigegeben.</p>
          )}
          <p>
            Im Nachweis bleibt, wer es wann freigegeben, geöffnet und gelöscht
            hat.
          </p>
        </div>
      }
      gate={{
        mode: "textConfirm",
        expected: "LÖSCHEN",
        inputId: "confirm-delete-traeger-document",
        label: "Tippen Sie zur Bestätigung LÖSCHEN ein:",
        placeholder: "LÖSCHEN",
      }}
      confirmLabel="Endgültig löschen"
      loadingLabel="Wird gelöscht …"
      loading={loading}
      error={error}
    />
  );
}
