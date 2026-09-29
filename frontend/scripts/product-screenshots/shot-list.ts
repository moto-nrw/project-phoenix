import { readFile } from "node:fs/promises";
import { parse } from "yaml";
import { z } from "zod";

import { DEFAULT_DEVICES, DEVICE_IDS, type DeviceId } from "./devices";

// Die Shot-Liste (#3759): YAML im Repo, damit ein Shot ohne Code-Änderung
// hinzukommt oder wegfällt. Das Schema ist strikt: ein unbekanntes Feld ist
// fast immer ein Tippfehler (`geraet` statt `geraete`) und würde sonst still
// ignoriert, also bricht das Laden ab.

const SHOT_ID = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

const stepSchema = z.union([
  z.strictObject({ klicken: z.string().min(1) }),
  z.strictObject({ warten_auf: z.string().min(1) }),
  z.strictObject({
    scrollen: z.union([z.number().int().nonnegative(), z.string().min(1)]),
  }),
]);

const shotSchema = z
  .strictObject({
    id: z
      .string()
      .regex(
        SHOT_ID,
        "id muss ein Kebab-Slug aus a-z, 0-9 und Bindestrichen sein (Umlaute als ae/oe/ue)",
      ),
    titel: z.string().min(1),
    portal: z.enum(["tenant", "eltern"]),
    rolle: z.enum(["admin", "betreuer", "eltern"]),
    pfad: z
      .string()
      .regex(/^\/(?!\/)[^\s]*$/, "pfad muss mit genau einem / beginnen"),
    geraete: z.array(z.enum(DEVICE_IDS)).min(1).optional(),
    vorbereitung: z.array(stepSchema).optional(),
  })
  .superRefine((shot, ctx) => {
    const expectedRoles =
      shot.portal === "eltern" ? ["eltern"] : ["admin", "betreuer"];
    if (!expectedRoles.includes(shot.rolle)) {
      ctx.addIssue({
        code: "custom",
        path: ["rolle"],
        message: `Im Portal ${shot.portal} ist nur ${expectedRoles.join(" oder ")} erlaubt`,
      });
    }
    if (shot.geraete && new Set(shot.geraete).size !== shot.geraete.length) {
      ctx.addIssue({
        code: "custom",
        path: ["geraete"],
        message: "geraete darf kein Gerät doppelt enthalten",
      });
    }
  });

const shotListSchema = z
  .strictObject({ shots: z.array(shotSchema).min(1) })
  .superRefine((list, ctx) => {
    const seen = new Set<string>();
    list.shots.forEach((shot, index) => {
      if (seen.has(shot.id)) {
        ctx.addIssue({
          code: "custom",
          path: ["shots", index, "id"],
          message: `id ${shot.id} kommt mehrfach vor`,
        });
      }
      seen.add(shot.id);
    });
  });

export type Step = z.infer<typeof stepSchema>;
export type Shot = z.infer<typeof shotSchema>;

function formatIssues(source: string, error: z.ZodError): string {
  const lines = error.issues.map((issue) => {
    const path = issue.path.length > 0 ? issue.path.join(".") : "(Wurzel)";
    return `  ${path}: ${issue.message}`;
  });
  return `Ungültige Shot-Liste ${source}:\n${lines.join("\n")}`;
}

/** Parst und validiert den YAML-Text. Wirft mit allen Fehlern auf einmal. */
export function parseShotList(yamlText: string, source = "shots.yaml"): Shot[] {
  let raw: unknown;
  try {
    raw = parse(yamlText);
  } catch (error) {
    throw new Error(
      `Shot-Liste ${source} ist kein gültiges YAML: ${error instanceof Error ? error.message : String(error)}`,
      { cause: error },
    );
  }
  const result = shotListSchema.safeParse(raw);
  if (!result.success) {
    throw new Error(formatIssues(source, result.error));
  }
  return result.data.shots;
}

export async function loadShotList(path: string): Promise<Shot[]> {
  return parseShotList(await readFile(path, "utf8"), path);
}

/** Geräte eines Shots: die eigene Angabe oder der Standard seines Portals. */
export function devicesFor(shot: Shot): readonly DeviceId[] {
  return shot.geraete ?? DEFAULT_DEVICES[shot.portal];
}

/**
 * Wählt Shots nach ID aus. Ohne IDs kommen alle zurück; eine unbekannte ID
 * bricht ab, statt einen leeren Lauf als Erfolg zu melden.
 */
export function selectShots(
  shots: readonly Shot[],
  ids: readonly string[],
): Shot[] {
  if (ids.length === 0) return [...shots];
  const known = new Set(shots.map((shot) => shot.id));
  const unknown = ids.filter((id) => !known.has(id));
  if (unknown.length > 0) {
    throw new Error(
      `Unbekannte Shot-ID: ${unknown.join(", ")}. Bekannt: ${[...known].join(", ")}`,
    );
  }
  const wanted = new Set(ids);
  return shots.filter((shot) => wanted.has(shot.id));
}
