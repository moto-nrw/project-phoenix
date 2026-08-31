"use client";

import { useState } from "react";
import { useSession } from "next-auth/react";
import { EyeIcon } from "@phosphor-icons/react";
import { Button } from "~/components/ui/button";
import { useShellAuthSafe } from "~/lib/shell-auth-context";
import { mutate } from "~/lib/swr";
import { performEndStaffPreview } from "~/lib/staff-preview-api";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StaffPreviewBanner" });

/**
 * Fester Hinweisstreifen während der Mitarbeiter-Vorschau (#2893). Auf jeder
 * Seite sichtbar: er benennt die gewählte Person, sagt, dass nur Lesen geht,
 * und beendet die Vorschau mit einem Klick.
 *
 * Zweiteilig, damit die Hülle ohne SessionProvider renderbar bleibt (die
 * AppShell wird auch providerlos gerendert): useSession läuft erst im
 * inneren Streifen, den es nur während einer aktiven Vorschau gibt.
 */
export function StaffPreviewBanner() {
  const shellAuth = useShellAuthSafe();
  if (shellAuth?.isPreview !== true) return null;
  return (
    <ActivePreviewBanner
      previewTargetName={shellAuth.previewTargetName}
      previewTargetAccountId={shellAuth.previewTargetAccountId}
    />
  );
}

function ActivePreviewBanner({
  previewTargetName,
  previewTargetAccountId,
}: {
  readonly previewTargetName?: string;
  readonly previewTargetAccountId?: number;
}) {
  const { update } = useSession();
  const [isEnding, setIsEnding] = useState(false);

  const handleEnd = async () => {
    if (isEnding) return;
    setIsEnding(true);
    try {
      await performEndStaffPreview(
        previewTargetAccountId?.toString(),
        update,
        mutate,
      );
    } catch (err) {
      logger.error("staff_preview_end_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    }
    // Volle Neuladung: jede sichtbare Seite muss wieder mit den
    // Admin-Rechten gerendert werden.
    window.location.reload();
  };

  return (
    <div
      role="status"
      className="fixed inset-x-0 top-0 z-50 flex h-12 items-center justify-between gap-3 px-4 text-white shadow-md"
      style={{ backgroundColor: LOCATION_COLORS.SCHOOLYARD }}
    >
      <p className="flex min-w-0 items-center gap-2 text-sm font-medium">
        <EyeIcon
          aria-hidden="true"
          weight="bold"
          className="h-4 w-4 shrink-0"
        />
        <span className="truncate">
          {/* Kleine Bildschirme: kurz, damit der Name nie abgeschnitten wird. */}
          <span className="sm:hidden">
            Vorschau:{" "}
            <span className="font-semibold">
              {previewTargetName ?? "Mitarbeitende Person"}
            </span>
          </span>
          <span className="hidden sm:inline">
            Vorschau: Sie sehen moto wie{" "}
            <span className="font-semibold">
              {previewTargetName ?? "diese Person"}
            </span>
            . Sie können nur lesen.
          </span>
        </span>
      </p>
      <Button
        type="button"
        variant="surface"
        size="compact"
        onClick={() => void handleEnd()}
        isLoading={isEnding}
        loadingText="Wird beendet …"
        className="shrink-0"
      >
        Vorschau beenden
      </Button>
    </div>
  );
}
