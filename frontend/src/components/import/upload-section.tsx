import React, { useRef } from "react";
import { Check, FileText, Upload } from "lucide-react";

interface UploadSectionProps {
  readonly isDragging: boolean;
  readonly isLoading: boolean;
  readonly uploadedFile: File | null;
  readonly onDragEnter: (e: React.DragEvent) => void;
  readonly onDragLeave: (e: React.DragEvent) => void;
  readonly onDragOver: (e: React.DragEvent) => void;
  readonly onDrop: (e: React.DragEvent) => void;
  readonly onFileSelect: (file: File) => void;
}

export function UploadSection({
  isDragging,
  isLoading,
  uploadedFile,
  onDragEnter,
  onDragLeave,
  onDragOver,
  onDrop,
  onFileSelect,
}: UploadSectionProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleZoneClick = () => {
    fileInputRef.current?.click();
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      fileInputRef.current?.click();
    }
  };
  return (
    <div className="rounded-xl border border-gray-100 bg-white p-6">
      <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
        <Upload className="text-moto-green-hover h-5 w-5" aria-hidden="true" />
        Schritt 2: Datei hochladen
      </h3>

      {/* Hidden file input */}
      <input
        ref={fileInputRef}
        type="file"
        accept=".csv,.xlsx"
        tabIndex={-1}
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) onFileSelect(file);
        }}
        className="sr-only"
        aria-label="Datei auswählen"
      />

      {/* Drop zone using native fieldset for accessibility */}
      <fieldset
        className={`relative m-0 rounded-xl border-2 border-dashed p-12 text-center transition-all duration-300 ${
          isDragging
            ? "border-moto-green-hover bg-moto-green-soft"
            : "border-gray-300 bg-gray-50 hover:border-gray-400"
        }`}
        onDragEnter={onDragEnter}
        onDragLeave={onDragLeave}
        onDragOver={onDragOver}
        onDrop={onDrop}
      >
        <legend className="sr-only">
          Datei-Upload-Bereich für Drag-and-Drop
        </legend>
        {/* Native button overlay - handles click/keyboard, covers entire zone */}
        <button
          type="button"
          onClick={handleZoneClick}
          onKeyDown={handleKeyDown}
          aria-label="Datei hochladen - ziehen Sie eine Datei hierher oder klicken Sie zum Auswählen"
          className="focus:ring-moto-green-hover absolute inset-0 z-10 cursor-pointer rounded-xl bg-transparent focus:ring-2 focus:ring-offset-2 focus:outline-none"
        />

        {/* Visual content - non-interactive, pointer-events handled by button above */}
        {isLoading ? (
          <div className="pointer-events-none flex flex-col items-center gap-4">
            <div className="border-t-moto-green-hover h-12 w-12 animate-spin rounded-full border-4 border-gray-300"></div>
            <p className="text-sm text-gray-600">Datei wird analysiert...</p>
          </div>
        ) : (
          <div className="pointer-events-none flex flex-col items-center gap-4">
            <Upload
              className={`h-16 w-16 transition-colors ${isDragging ? "text-moto-green-hover" : "text-gray-400"}`}
              aria-hidden="true"
            />
            <div>
              <p className="mb-1 text-lg font-medium text-gray-900">
                {isDragging ? "Datei hier ablegen..." : "Datei hierher ziehen"}
              </p>
              <p className="text-sm text-gray-500">oder</p>
            </div>
            <span className="inline-flex items-center gap-2 rounded-lg bg-gray-900 px-6 py-3 text-white">
              <FileText className="h-5 w-5" aria-hidden="true" />
              Datei auswählen
            </span>
            {uploadedFile && (
              <div className="flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-4 py-2 text-sm text-gray-600">
                <Check
                  className="text-moto-green-hover h-4 w-4"
                  aria-hidden="true"
                />
                {uploadedFile.name}
              </div>
            )}
          </div>
        )}
      </fieldset>
    </div>
  );
}
