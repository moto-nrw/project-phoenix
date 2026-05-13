"use client";

import { useState } from "react";
import { Alert, Button } from "~/components/ui";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "RecoveryCodesDisplay" });

interface RecoveryCodesDisplayProps {
  readonly codes: readonly string[];
  readonly onConfirm: () => void;
  readonly title?: string;
  readonly intro?: string;
}

export function RecoveryCodesDisplay({
  codes,
  onConfirm,
  title = "Wiederherstellungscodes sichern",
  intro = "Speichern Sie diese 10 Codes sicher. Jeder gilt einmal, falls Sie keinen E-Mail-Zugriff haben.",
}: RecoveryCodesDisplayProps) {
  const [confirmed, setConfirmed] = useState(false);
  const [copied, setCopied] = useState(false);
  const [downloaded, setDownloaded] = useState(false);

  const codesText = codes.join("\n");

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(codesText);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      logger.warn("clipboard_copy_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    }
  };

  const handleDownload = () => {
    const blob = new Blob([codesText + "\n"], {
      type: "text/plain;charset=utf-8",
    });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "moto-wiederherstellungscodes.txt";
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
    setDownloaded(true);
  };

  const handlePrint = () => {
    window.print();
  };

  return (
    <div className="space-y-5 text-left">
      <div className="space-y-1">
        <h2 className="text-xl font-semibold text-gray-900">{title}</h2>
        <p className="text-sm leading-relaxed text-gray-600">{intro}</p>
      </div>

      <Alert
        type="warning"
        message="Diese Codes werden nur einmalig angezeigt."
      />

      <div
        className="rounded-lg border border-gray-200 bg-gray-50 p-3"
        aria-label="Liste der Wiederherstellungscodes"
      >
        <ol className="grid grid-cols-2 gap-1.5 font-mono text-xs text-gray-900">
          {codes.map((code, index) => (
            <li
              key={code}
              className="flex items-center gap-1.5 rounded bg-white px-2 py-1.5 shadow-sm"
            >
              <span className="w-4 shrink-0 text-[10px] text-gray-400 tabular-nums">
                {(index + 1).toString().padStart(2, "0")}
              </span>
              <span className="truncate">{code}</span>
            </li>
          ))}
        </ol>
      </div>

      <div className="flex flex-wrap justify-center gap-2">
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => {
            void handleCopy();
          }}
        >
          {copied ? "Kopiert" : "Kopieren"}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={handleDownload}
        >
          {downloaded ? "Heruntergeladen" : "Herunterladen"}
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={handlePrint}>
          Drucken
        </Button>
      </div>

      <label className="flex items-start gap-2 select-none">
        <input
          type="checkbox"
          checked={confirmed}
          onChange={(e) => setConfirmed(e.target.checked)}
          className="mt-0.5 h-4 w-4 rounded border-gray-300"
          style={{ accentColor: LOCATION_COLORS.GROUP_ROOM }}
        />
        <span className="text-sm text-gray-700">
          Ich habe die Codes sicher gespeichert.
        </span>
      </label>

      <Button
        type="button"
        variant="primary"
        size="base"
        className="w-full"
        onClick={onConfirm}
        disabled={!confirmed}
      >
        Los geht&apos;s
      </Button>
    </div>
  );
}
