"use client";

import { useState } from "react";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "RecoveryCodesDisplay" });

const PRIMARY_GREEN = "#83CD2D";
const PRIMARY_BLUE = "#5080D8";

interface RecoveryCodesDisplayProps {
  readonly codes: readonly string[];
  readonly onConfirm: () => void;
  readonly title?: string;
  readonly intro?: string;
}

export function RecoveryCodesDisplay({
  codes,
  onConfirm,
  title = "Wiederherstellungscodes",
  intro = "Speichern Sie diese Codes an einem sicheren Ort. Jeder Code kann genau einmal anstelle eines E-Mail-Codes verwendet werden, falls Sie keinen Zugriff auf Ihre E-Mail haben.",
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

  return (
    <div className="space-y-5 text-left">
      <div>
        <h2 className="mb-1 text-xl font-semibold text-gray-900">{title}</h2>
        <p className="text-sm text-gray-600">{intro}</p>
      </div>

      <div
        className="rounded-lg border border-gray-200 bg-gray-50 p-4"
        aria-label="Liste der Wiederherstellungscodes"
      >
        <ul className="grid grid-cols-1 gap-2 font-mono text-base text-gray-900 sm:grid-cols-2">
          {codes.map((code) => (
            <li key={code} className="rounded bg-white px-3 py-2 shadow-sm">
              {code}
            </li>
          ))}
        </ul>
      </div>

      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => {
            void handleCopy();
          }}
          className="rounded-lg px-4 py-2 text-sm font-medium text-white transition-all hover:opacity-90 focus:outline-none active:scale-95"
          style={{ backgroundColor: PRIMARY_BLUE }}
        >
          {copied ? "Kopiert!" : "In Zwischenablage kopieren"}
        </button>
        <button
          type="button"
          onClick={handleDownload}
          className="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition-all hover:bg-gray-50 focus:outline-none active:scale-95"
        >
          {downloaded ? "Heruntergeladen ✓" : "Als .txt herunterladen"}
        </button>
      </div>

      <div className="rounded-lg bg-amber-50 p-3 text-sm text-amber-800">
        <strong>Wichtig:</strong> Diese Codes werden nur jetzt einmalig
        angezeigt. Wenn Sie diese Seite verlassen, sind sie nicht erneut
        einsehbar. Sie können später jederzeit neue Codes generieren.
      </div>

      <label className="flex items-start gap-2 select-none">
        <input
          type="checkbox"
          checked={confirmed}
          onChange={(e) => setConfirmed(e.target.checked)}
          className="mt-1 h-4 w-4 rounded border-gray-300"
          style={{ accentColor: PRIMARY_GREEN }}
        />
        <span className="text-sm text-gray-700">
          Ich habe die Codes gespeichert und kann darauf zugreifen, falls ich
          keinen Zugriff auf meine E-Mail habe.
        </span>
      </label>

      <button
        type="button"
        onClick={onConfirm}
        disabled={!confirmed}
        className="w-full rounded-xl px-8 py-2.5 text-sm font-semibold text-white transition-all duration-200 hover:opacity-90 focus:outline-none active:scale-95 disabled:cursor-not-allowed disabled:opacity-50"
        style={{ backgroundColor: PRIMARY_GREEN }}
      >
        Weiter zum Dashboard
      </button>
    </div>
  );
}
