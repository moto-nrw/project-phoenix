# Bundle-Referenzen nach unten nachziehen (#5483)

## Ergebnis und Ausgangsstand

Gemessenes Development: `2446b9837128419bb7fdc98da871d0848e813834`, einschließlich
#5463. Frontend-Quellbaum: `fad016f09c0e0342e81ea2e9ad76a83b024c23c9`.
Zwei aufeinanderfolgende kalte Linux-Analysen messen alle 193 Routen
**byte-identisch**: maximale Abweichung 0 B / 0 %. Das ist beobachtete
Wiederholbarkeit dieses Stands, keine Garantie für beliebige Plattformen.

54 Routen liegen aktuell mehr als 5 % unter ihrer Referenz. Der historische
[CI-Lauf 41283980408](https://github.com/moto-nrw/project-phoenix/actions/runs/41283980408)
auf `02729196ab7f092c4ccfdcab359ab6d6ec8fb9e8` identifiziert die 57 Routen aus
dem Issue. Seine Werte dienen **nur zur Routenauswahl**, nicht als neue Referenz.
Drei davon liegen heute knapp weniger als 5 % darunter: `/operator/organizations`,
`/operator/persons`, `/operator/schools/[id]/settings`. Alle 57 sind weiterhin
kleiner als ihre alte Referenz und werden auf den aktuellen Messwert gesenkt.

Jede neue Obergrenze ist `legacyCeiling(referenceBytes)`, auf diesen 57 Routen
also `round(referenceBytes × 1,02)`. Keine Referenz und keine Obergrenze steigt;
alle anderen Budgets bleiben unverändert, einschließlich expliziter Ausnahmen.
Der Development-Messstand überschreitet keine bisherige absolute Obergrenze.
Die Analyse zählt alle Client-Chunks, auch dynamisch geladene. Gemeinsame
Chunks zählen je Route; Routensummen sind keine Gesamtgröße der Anwendung.

[Maschinenlesbare Evidenz](bundle-ratchet-5483-evidence.json): vollständige
Routenwerte beider Wiederholungen, historische Auswahlmessung, alte und neue
Policy je betroffener Route, Quellidentität und Image-Digest.
Vorherige Ursachenklärung: [#5453](bundle-growth-5453.md).

## Linux-Umgebung und Reproduktion

Node 24.22.2, pnpm 10.34.5, Next 16.4.1, Linux aarch64, Debian Bookworm.
Image: `node@sha256:c86ecead9e55a5210ac1727de38a4ca840bf03dcaf34dc9749addbf383b1920c`.
Keine lokalen Env-Dateien, keine App-Server und keine Zugangsdaten.
Die öffentlichen Buildwerte entsprechen `.github/workflows/lint.yml:frontend-build`:
`NEXT_PUBLIC_APP_ENV=test`, API-URLs `http://localhost:8080`,
`NEXTAUTH_URL=http://localhost:3000`, beide Tenant-Domains `localhost`,
Portal-Hostnames `{operator,parents,schule,caterer,traeger}.localhost:3000`.
Die beiden Auth-/Metrikwerte sind ausschließlich die öffentlichen Wegwerfwerte
des CI-Jobs; `AUTH_JWT_EXPIRY=15m`, `AUTH_JWT_REFRESH_EXPIRY=168h`,
`SKIP_ENV_VALIDATION=true`, `NEXT_TELEMETRY_DISABLED=1`.

1. `git archive --format=tar 2446b9837128419bb7fdc98da871d0848e813834 frontend`
   in ein eigenes temporäres Verzeichnis entpacken und nach `/source` mounten.
2. Den Container ausschließlich über `~/bin/agent-heavy -- docker run --rm …`
   mit obigem Image und expliziten CI-Buildwerten starten. Keine Builds parallel.
3. Im archivierten `frontend/`: `corepack enable`,
   `corepack prepare pnpm@10.34.5 --activate`, `pnpm install --frozen-lockfile`.
4. Zweimal nacheinander: ausschließlich das eigene `.next` entfernen,
   `pnpm exec next experimental-analyze --output` ausführen und direkt danach
   die komprimierten Routensummen mit dem archivierten Parser exportieren:

   ```js
   import { writeFileSync } from "node:fs";
   import { readRouteBundles } from "./scripts/perf/bundle-routes.mjs";
   const routes = Object.fromEntries(
     readRouteBundles().map(({ route, compressed }) => [route, compressed]),
   );
   writeFileSync("/source/run-N.json", JSON.stringify(routes, null, 2));
   ```

Beide Läufe nutzen denselben Quellbaum, Lockfile, Parser und dieselbe Umgebung;
beide starten ohne `.next`. Die Installation wird wiederverwendet, nicht der
Turbopack-Buildcache. Das temporäre Archiv enthält keinen warmen Cache anderer
Sessions. Keine macOS-Messung wird als Referenz eingesetzt.

Die Referenzen stammen aus Linux ARM64. Die abschließende Linux-x64-PR-CI prüft
die Budgets und das eigene Delta mit jeweils gleichartigen kalten Head-/Base-
Messungen. Unterschiede zwischen ARM64 und x64 sind **nicht** durch die beiden
Wiederholungen abgedeckt. Frühere Messungen in #5453 zeigten 9–10 B Unterschied;
das ist Kontext, keine universelle Plattformtoleranz oder Erlaubnis zum Erhöhen.

## Dauerhafte Pflege und Rauschgrenze

`compareBudgets` warnt, wenn ein Messwert **mehr als 5 %** unter seiner
Referenz liegt. Exakt 5 % warnen nicht. Die bestehende Warnungsausgabe schreibt
dies in Job-Summary und CI-Annotationen, auch bei Prüfungen ohne Merge-Basis.
Reduktionen bleiben nicht-blockierend; eigene PR-Deltas und absolute Grenzen
werden unverändert geprüft. Die Warnung verändert keine Policy automatisch.

5 % liegen deutlich über den beobachteten 0 % Wiederholungsrauschen und auch
über der bestehenden 2-%-Wachstumstoleranz. So verlangt nicht jede kleine
Minifier-/Chunk-Verschiebung eine Policy-Änderung, während die 5–9-%-Altlast aus
dem Issue sichtbar wird. Zwei gleiche Läufe begrenzen nicht jedes künftige
Rauschen; deshalb löst die Schwelle eine Warnung mit erneuter Messpflicht aus,
keine automatische Freigabe oder blockierende Rauschentscheidung.

Bei einer Warnung: aktuellen Development-SHA festhalten, zweimal kalt unter
vergleichbarer Linux-Umgebung messen, Ergebnisse prüfen und nur die betroffene
Referenz samt `legacyCeiling` nach unten nachziehen. Bei abweichenden Läufen
zuerst Ursache und Plattform-/Konfigurationsunterschiede klären. Keine fremden
Überschreitungen, höheren Referenzen oder breitere Delta-Toleranz mitgenehmigen.

## Verifikation

- Neue Warnungsfälle: vor Implementierung 3 fehlgeschlagene Tests, danach grün.
- Policy-/Messungs-/Base-Skripttests: 30 bestanden.
- `cd frontend && pnpm run check`: bestanden, ohne Warnungen.
- Finale CI und Quorum werden im PR mit dem geprüften Head dokumentiert.
