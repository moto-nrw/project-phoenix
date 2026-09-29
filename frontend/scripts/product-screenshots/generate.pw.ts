import { test } from "@playwright/test";
import { join } from "node:path";

import { loadStackAccess } from "./access";
import { runPipeline } from "./pipeline";
import { loadShotList, selectShots } from "./shot-list";

// Erzeugt die Produkt-Screenshots aus shots.yaml (#3759). Gestartet über
// scripts/product-screenshots.sh; direkt: pnpm run generate:screenshots.
//
//   SHOT_IDS   kommagetrennte Shot-IDs; leer = alle
//   SHOT_OUT   Ausgabeverzeichnis (wird ersetzt)
//   SHOT_VERSION  Versionsname im Manifest, Standard "lokal"

test("Produkt-Screenshots erzeugen", async () => {
  const ids = (process.env.SHOT_IDS ?? "")
    .split(",")
    .map((id) => id.trim())
    .filter(Boolean);
  const shots = selectShots(
    await loadShotList(join(import.meta.dirname, "shots.yaml")),
    ids,
  );
  const outDir = process.env.SHOT_OUT;
  if (!outDir)
    throw new Error(
      "SHOT_OUT fehlt (scripts/product-screenshots.sh setzt es).",
    );

  const manifest = await runPipeline({
    access: loadStackAccess(),
    shots,
    outDir,
    version: process.env.SHOT_VERSION ?? "lokal",
    log: (message) => console.log(message),
  });
  const files = manifest.shots.reduce(
    (sum, shot) => sum + shot.dateien.length,
    0,
  );
  console.log(`${manifest.shots.length} Shots, ${files} Dateien in ${outDir}`);
});
