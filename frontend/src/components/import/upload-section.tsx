import { Check } from "lucide-react";
import { Dropzone } from "~/components/ui/dropzone";

interface UploadSectionProps {
  readonly isLoading: boolean;
  readonly uploadedFile: File | null;
  /**
   * Überschrift der Karte. Der Eröffnungssalden-Import (#2132) hat einen
   * Schritt mehr (Stichtag und Begründung), deshalb ist die Schrittnummer
   * überschreibbar. Ohne Angabe bleibt der bisherige Text stehen.
   */
  readonly title?: string;
  readonly onFileSelect: (file: File) => void;
}

export function UploadSection({
  isLoading,
  uploadedFile,
  title = "Schritt 2: Datei hochladen",
  onFileSelect,
}: UploadSectionProps) {
  return (
    <div className="moto-content-surface rounded-xl border p-6">
      <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
        <svg
          className="text-moto-green-strong h-5 w-5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
          />
        </svg>
        {title}
      </h3>

      <Dropzone
        accept=".csv,.xlsx"
        legend="Bereich zum Hochladen einer Datei"
        label="Datei hierher ziehen"
        dragLabel="Datei hier ablegen..."
        buttonLabel="Datei auswählen"
        loading={isLoading}
        loadingLabel="Datei wird analysiert..."
        onFile={onFileSelect}
      />

      {uploadedFile && !isLoading ? (
        <p className="mt-3 flex items-center gap-2 text-sm text-gray-600">
          <Check
            className="text-moto-green-strong h-4 w-4 shrink-0"
            aria-hidden="true"
          />
          <span className="min-w-0 truncate">{uploadedFile.name}</span>
        </p>
      ) : null}
    </div>
  );
}
