/**
 * Wörter und Zahlen der geteilten Dokumente (#4692), für den Träger und für
 * die Schule. Eine Freigabe heißt immer nach den Menschen, die das Dokument
 * sehen: „Eltern der Gruppe Sonnenblumen (OGS Nord)“, „Alle Schulen“.
 */
import { formatBytes } from "~/lib/files-api";
import type {
  TraegerDocumentRelease,
  TraegerDocumentSchool,
} from "~/lib/traeger-documents-api";

/** Die Art der Datei in einem Wort. */
export function documentKindLabel(contentType: string): string {
  if (contentType === "application/pdf") return "PDF";
  if (contentType.includes("wordprocessingml")) return "Word";
  if (contentType.includes("spreadsheetml")) return "Excel";
  if (contentType.includes("presentationml")) return "PowerPoint";
  if (contentType.startsWith("image/")) return "Bild";
  return "Datei";
}

/** „PDF · 240 KB“ */
export function documentFileFacts(document: {
  readonly contentType: string;
  readonly sizeBytes: number;
}): string {
  return `${documentKindLabel(document.contentType)} · ${formatBytes(document.sizeBytes)}`;
}

/** Die Namen der Schulen des Trägers, nach ID. */
export type SchoolNames = ReadonlyMap<string, string>;

export function schoolNamesOf(
  schools: readonly Pick<TraegerDocumentSchool, "id" | "name">[],
): SchoolNames {
  return new Map(schools.map((school) => [school.id, school.name]));
}

/** Für wen eine Freigabe gilt, aus Sicht des Trägers. */
export function releaseAudienceLabel(
  release: TraegerDocumentRelease,
  schools: SchoolNames,
): string {
  const school =
    release.schoolId === null
      ? null
      : (schools.get(release.schoolId) ?? "eine Schule");
  if (release.audience === "schools") {
    return school ?? "Alle Schulen";
  }
  if (release.groupId !== null) {
    return `Eltern der Gruppe „${release.groupName}“ (${school ?? "eine Schule"})`;
  }
  return school === null ? "Eltern an allen Schulen" : `Eltern: ${school}`;
}

/** Für wen eine Freigabe gilt, aus Sicht der eigenen Schule. */
export function schoolReleaseAudienceLabel(
  release: TraegerDocumentRelease,
): string {
  if (release.audience === "schools") {
    return release.schoolId === null
      ? "Alle Schulen des Trägers"
      : "Ihre Schule";
  }
  if (release.groupId !== null) {
    return `Eltern der Gruppe „${release.groupName}“`;
  }
  return release.schoolId === null
    ? "Eltern an allen Schulen"
    : "Eltern Ihrer Schule";
}
