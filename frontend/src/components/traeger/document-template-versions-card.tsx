import { Button } from "~/components/ui/button";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type { DocumentTemplateVersion } from "~/lib/traeger-document-templates";

/**
 * Die gespeicherten Versionen einer Vorlage (#4677), die neueste zuerst.
 * Eine Version ändert sich nie; „Text übernehmen“ lädt sie in den Editor,
 * erst Speichern macht daraus eine neue Version.
 */
export function DocumentTemplateVersionsCard({
  versions,
  canEdit,
  disabled = false,
  onLoad,
}: Readonly<{
  versions: readonly DocumentTemplateVersion[];
  canEdit: boolean;
  disabled?: boolean;
  onLoad: (version: DocumentTemplateVersion) => void;
}>) {
  return (
    <SectionCard
      title="Versionen"
      description="Jede gespeicherte Fassung bleibt erhalten. Schreiben, die schon erstellt sind, behalten ihre Version."
    >
      {versions.length === 0 ? (
        <p className="text-sm text-gray-600">
          Noch keine eigene Version. Neue Schreiben nutzen die Standardvorlage.
        </p>
      ) : (
        <ResponsiveActionList>
          {versions.map((version, index) => (
            <li
              key={version.version}
              className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
            >
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <p className="text-sm font-semibold text-gray-900">
                  Version {version.version}
                </p>
                {index === 0 ? (
                  <StatusBadge label="Gilt für neue Schreiben" tone="green" />
                ) : null}
                <p className="w-full text-sm text-gray-600">
                  Gespeichert am{" "}
                  {version.createdAt ? formatDate(version.createdAt) : "–"} ·{" "}
                  {version.subject}
                </p>
              </div>
              {canEdit ? (
                <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
                  <Button
                    type="button"
                    variant="surface"
                    size="compact"
                    aria-label={`Text von Version ${version.version} übernehmen`}
                    disabled={disabled}
                    onClick={() => onLoad(version)}
                  >
                    Text übernehmen
                  </Button>
                </div>
              ) : null}
            </li>
          ))}
        </ResponsiveActionList>
      )}
    </SectionCard>
  );
}
