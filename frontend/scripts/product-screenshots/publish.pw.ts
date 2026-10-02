import { test } from "@playwright/test";

import { berlinTodayISO } from "../../src/lib/date-helpers";
import { drivePublisher } from "./publish-drive";

// Lädt eine fertige Ausgabe der Pipeline nach Google Drive (#3759). Läuft nur
// in CI (.github/workflows/product-screenshots.yml), nach einem erfolgreichen
// Lauf von generate.pw.ts; lokal wird nichts hochgeladen.
//
//   SHOT_OUT       Ausgabeverzeichnis der Pipeline
//   SHOT_VERSION   Release-Version ohne `v`; muss im Manifest stehen
//   DRIVE_CLIENT_ID, DRIVE_CLIENT_SECRET, DRIVE_REFRESH_TOKEN, DRIVE_FOLDER_ID
//                  aus scripts/product-screenshots-drive-wizard.sh

function required(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`${name} fehlt.`);
  return value;
}

test("Produkt-Screenshots nach Drive hochladen", async () => {
  // Ein großer Lauf lädt einige hundert Dateien je Ordner.
  test.setTimeout(3_600_000);
  const outDir = required("SHOT_OUT");
  const release = { version: required("SHOT_VERSION"), date: berlinTodayISO() };
  const publisher = drivePublisher({
    credentials: {
      clientId: required("DRIVE_CLIENT_ID"),
      clientSecret: required("DRIVE_CLIENT_SECRET"),
      refreshToken: required("DRIVE_REFRESH_TOKEN"),
    },
    folderId: required("DRIVE_FOLDER_ID"),
    log: (message) => console.log(message),
  });
  await publisher.publish(outDir, release);
  console.log(
    `Version ${release.version} (${release.date}) nach Drive hochgeladen`,
  );
});
