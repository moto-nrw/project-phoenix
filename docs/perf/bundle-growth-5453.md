# Bundle-Zuwachs seit dem 28.10.2026 (#5453)

## Entscheidung

Keine pauschale Budgeterhöhung: #5408 hat den historischen Zuwachs auf den
16 App-Routen bereits mehr als ausgeglichen. Nur `/help/[[...topic]]` benötigt
eine neue Referenz. Die Hilfe enthält zusätzliche Artikel und Symbole; ihr
alter Deckel wird noch um rund 1 kB überschritten. Delta-Toleranz und die
Behandlung geerbter Überschreitungen als Warnung bleiben unverändert.

Eine experimentelle Zusammenlegung der Fehlercode-Module spart entgegen der
Hypothese kein Bundle: +322 B auf dem historischen Baum, +338 B mit einer
konkreten Laufzeitableitung auf dem heutigen Baum, jeweils auf allen 17
Routen. Beide Varianten wurden verworfen. Keine Laufzeitquellen oder Tests
werden dafür geändert.

## Zuordnung zu Commits und Modulen

Der Zuwachs ist kumulativ. Folgende kalte Linux-Zwei-Baum-Vergleiche ordnen
einzelne Beiträge dem jeweiligen Feature zu. Die drei Zahlen sind Tenant-Start,
Hilfe und Eltern-Login. Sie sind **keine addierbare Zeitreihe**: Features wurden
auf unterschiedlichen Merge-Basen gemessen und Chunk-Grenzen ändern sich.

| Änderung / Commit | Module und Zweck | Delta Start / Hilfe / Eltern-Login | Linux-Run |
|---|---|---:|---|
| #5182, `81a68c7792`, `86dd587225` | `swr/scope`, `swr/generation`, `swr/hooks`: Cache je Schule/Konto und Schutz vor alten Antworten | +4164 / −26 / +3949 B | [40754352528](https://github.com/moto-nrw/project-phoenix/actions/runs/40754352528) |
| #5227, `7d3e77effe` | Fehlercodes/-texte für Leitungsnachrichten | +283 / +283 / +370 B | [40874459376](https://github.com/moto-nrw/project-phoenix/actions/runs/40874459376) |
| #5249, `33dd08b12d`, `07c3d4b219` | Nachrichtenstatus, Mitlesende, Hilfe | 0 / +526 / +1027 B | [40874487582](https://github.com/moto-nrw/project-phoenix/actions/runs/40874487582) |
| #5339, `9e34667661` | Unfallanzeige: Fehlerkatalog, Hilfeartikel, FirstAidKit-Icon | +321 / +1673 / +344 B | [40971185508](https://github.com/moto-nrw/project-phoenix/actions/runs/40971185508) |
| #5259, `1a5af59c2e` | gemeinsame Kontoprüfung, `session-account-binding` | +781 / +234 / +1046 B | [41081035949](https://github.com/moto-nrw/project-phoenix/actions/runs/41081035949) |
| #5377, `349dad55fc` | Wochenprotokolle: Hilfe, Notebook-Icon in der damaligen vollständigen Konzept-Registry, Fehlertexte | +243 / +1600 / +324 B | [41193004602](https://github.com/moto-nrw/project-phoenix/actions/runs/41193004602) |
| #5109, `e60fec635f` | Elternsprechtag: Fehlercodes/-texte und Hilfe | +843 / +1788 / +1045 B | [41206397194](https://github.com/moto-nrw/project-phoenix/actions/runs/41206397194) |
| #5258, `3318cbcb7b`, `67c3942ac1` | gemeinsame SWR-Key-Factories und geschlossene Cache-Zugriffe | +612 / +20 / +1496 B | [41206620338](https://github.com/moto-nrw/project-phoenix/actions/runs/41206620338) |
| #5403, `74e9584142` | sitzungsgebundene Antwortsperre, `session-end-barrier`, Portal-Provider | +345 / +176 / +1357 B | [41251067700](https://github.com/moto-nrw/project-phoenix/actions/runs/41251067700) |
| #5445, `ec26ae4be1` | eigenes Delta des ursprünglichen Vergleichs, nicht die Altlast | +18 / 0 / +18 B | [41256428322](https://github.com/moto-nrw/project-phoenix/actions/runs/41256428322) |

Bei #5182 stimmen die archivierten Quellbäume exakt mit den aufeinanderfolgenden
Merges `7d898cb22d` und `a3a1d0de26` überein. Für die weiteren Zeilen enthält
die Evidenzdatei beide Quellidentitäten, Revisionen und die Deltas aller 17 Routen.
Die genannten Implementierungscommits erklären den Codepfad; die Delta-Zahlen
gehören zum gesamten jeweiligen Feature-Head, nicht isoliert zu einem Commit.

### Modulbilanz der historischen Endpunkte

Kalter Vergleich `8d02dc39a4` → `cb4d7d12e2`, Linux ARM64. Gemeinsamer
Fehlercode-Registeranteil: +1009 B; Fehlertextkatalog: +3137 B auf jeder der
17 Routen. `220a468ae0` (#5243) führte die Registeraufteilung ein; neue
Fehlercodes und übersetzte Meldungen aus den oben genannten Features wachsen
zusätzlich mit. Der Importpfad ist `error-presentation` → Register/Katalog;
globale Fehlerdarstellung erklärt die gemeinsame Last, nicht ein neues Layout-Paket.

| Route | Register | Fehlertexte | SWR-Helfer | Hilfemodule | Phosphor | Übrige Shell-/Seiten-/Chunk-Änderungen | Gesamt |
|---|---:|---:|---:|---:|---:|---:|---:|
| `/[tenant]` | +1009 | +3137 | +4711 | +0 | +710 | +4811 | +14378 |
| `/[tenant]/display` | +1009 | +3137 | +4711 | +0 | +718 | +4125 | +13700 |
| `/help/[[...topic]]` | +1009 | +3137 | +0 | +9824 | +1555 | -618 | +14907 |
| `/parents` | +1009 | +3137 | +4918 | +0 | +745 | +7838 | +17647 |
| `/parents/anmeldung` | +1009 | +3137 | +4918 | +0 | +745 | +7272 | +17081 |
| `/parents/calendar` | +1009 | +3137 | +4918 | +0 | +745 | +7333 | +17142 |
| `/parents/demo` | +1009 | +3137 | +4918 | +0 | +745 | +7301 | +17110 |
| `/parents/documents` | +1009 | +3137 | +4918 | +0 | +745 | +7308 | +17117 |
| `/parents/invite` | +1009 | +3137 | +4918 | +0 | +745 | +7555 | +17364 |
| `/parents/login` | +1009 | +3137 | +4918 | +0 | +745 | +7700 | +17509 |
| `/parents/meal-plan` | +1009 | +3137 | +5866 | +0 | +745 | +8487 | +19244 |
| `/parents/messages` | +1009 | +3137 | +5865 | +0 | +745 | +8106 | +18862 |
| `/parents/messages/[studentId]` | +1009 | +3137 | +5865 | +0 | +745 | +8071 | +18827 |
| `/parents/messages/group/[groupThreadId]` | +1009 | +3137 | +4918 | +0 | +745 | +7472 | +17281 |
| `/parents/news` | +1009 | +3137 | +4918 | +0 | +745 | +7791 | +17600 |
| `/parents/news/[id]/nachweis` | +1009 | +3137 | +4918 | +0 | +745 | +7457 | +17266 |
| `/parents/settings` | +1009 | +3137 | +4918 | +0 | +745 | +7309 | +17118 |

Die übrigen Beiträge umfassen insbesondere Sitzungsgrenzen, SSE-Kontobindung
(`37361cd377`, `37ebab3f6a`), gemeinsame Form-Kit-/Navigationsbausteine
(#5233, `f06bd573b3`) sowie seitenspezifische Änderungen. Die Bilanz summiert
auch negative Beiträge; sie unterschlägt keine Einsparungen. Minifier- und
Chunk-ID-Änderungen dürfen nicht als isolierte Featurekosten interpretiert werden.

## Nach #5408 und Budgetentscheidung je Route

`76b331c2ce` (#5408) ersetzt den gemeinsamen Laufzeitimport von
`MOTO_CONCEPTS` durch direkt importierte Konzeptdefinitionen. Das beseitigt
den vermeidbaren gemeinsamen Icon-Anteil. Der kalte Linux-Run
[41254911890](https://github.com/moto-nrw/project-phoenix/actions/runs/41254911890)
misst 643841 B auf der Tenant-Startseite und 763236 B auf dem Eltern-Login.
Der heutige Baum enthält zusätzlich #5433 und #5456. Folgende Werte stammen
aus dem kalten ARM64-Build von `20ef1e283b`; sie sind Diagnosewerte, keine
Linux-x64-Budgetreferenzen. Die zugehörige finale PR-CI prüft die absoluten Werte.

| Route | Historischer Zuwachs laut Issue (Linux-CI) | Heute (Linux ARM64) | Alter Deckel | Entscheidung |
|---|---:|---:|---:|---|
| `/[tenant]` | +14379 | 645323 | 719167 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |
| `/[tenant]/display` | +13701 | 617056 | 690212 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |
| `/help/[[...topic]]` | +14908 | 731023 | 729983 | Nur diese Referenz bewusst neu setzen; zusätzliche Hilfeartikel und Symbole. |
| `/parents` | +17648 | 788490 | 837176 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |
| `/parents/anmeldung` | +17082 | 755362 | 807443 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |
| `/parents/calendar` | +17143 | 768496 | 819906 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |
| `/parents/demo` | +17111 | 755110 | 808403 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |
| `/parents/documents` | +17118 | 763431 | 811714 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |
| `/parents/invite` | +17365 | 766052 | 819305 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |
| `/parents/login` | +17510 | 765995 | 819099 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |
| `/parents/meal-plan` | +19245 | 768739 | 819938 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |
| `/parents/messages` | +18863 | 850702 | 900841 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |
| `/parents/messages/[studentId]` | +18828 | 843310 | 894499 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |
| `/parents/messages/group/[groupThreadId]` | +17282 | 760693 | 813923 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |
| `/parents/news` | +17601 | 777982 | 829923 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |
| `/parents/news/[id]/nachweis` | +17267 | 757027 | 810199 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |
| `/parents/settings` | +17119 | 779307 | 825731 | Unverändert; historischer Zuwachs nach #5408 ausgeglichen. |

Für die Hilfe ist 731013 B bereits durch den kalten Linux-x64-Run
[41276170050](https://github.com/moto-nrw/project-phoenix/actions/runs/41276170050)
auf `1eec6b529f` belegt; #5433 trägt dort 0 B bei
([41277751229](https://github.com/moto-nrw/project-phoenix/actions/runs/41277751229)).
Die neue Obergrenze ist `Math.round(731013 * 1.02) = 745633` B.
`/help/nfc/erste-schritte` im gleichen Budgetdokument bleibt unverändert.
Die benötigten Inhalte betreffen u. a. Unfallanzeigen, Wochenprotokolle,
Elternsprechtage, Leitungsnachrichten und Ferienanmeldungen. Sie werden für
Artikelauswahl und Hilfesuche genutzt; entfernt werden weder Artikel noch Suche.

## Messung, Reproduktion und Grenzen

- Zahlen sind komprimierte Client-JS-Bytes laut Turbopack, einschließlich
  gemeinsamer Chunks pro Route. Routen niemals zu einer Gesamtgröße summieren.
- Lokale Builds: Node 24.22.2, pnpm 10.34.5, Next 16.4.1, `linux/aarch64`,
  Image-Digest und alle Messwerte in der Evidenzdatei. Alle fünf Analysen liefen
  nacheinander über `~/bin/agent-heavy -- docker run …`, ohne App-Server.
- Je Revision: `git archive <revision> frontend`, eigene eingefrorene
  Lockfile-Installation, leeres `.next`, `pnpm exec next experimental-analyze --output`.
  Umgebung wie `.github/workflows/lint.yml:frontend-build`, ausschließlich
  Localhost-URLs und zur Laufzeit erzeugte Wegwerf-Geheimnisse.
- Die unveränderten Quellen der Referenz wurden vollständig kalt neu gebaut.
  Gegenüber dem historischen x64-Artefakt ergeben sich auf den 17 Routen
  gleichmäßig +10 B, beim Altstand +9 B. Das historische Delta weicht deshalb
  um 1 B ab. Absolute Budgets stützen sich ausschließlich auf die x64-CI, nicht
  auf diese ARM64-Werte. Kein macOS-Build wurde zur Budgetfreigabe verwendet.
- Die zwei Gegenproben ändern ausschließlich die Registerdarstellung. Die
  vollständigen Domänentabellen und ihre Werte bleiben erhalten. Ihre gemessene
  Verschlechterung verhindert, dass eine nur vermutete Optimierung übernommen wird.
- Der Budget-Reset `160d55ffc9` ist keine Ursachenbehebung. Die verbleibenden
  16 Budgets werden hier weder wegen dieses Resets noch wegen fremder Features erhöht.

[Maschinenlesbare Evidenz](bundle-growth-5453-evidence.json): historische
Quellidentitäten, alle Feature-Paare, die vollständige Modulgruppenbilanz je
Route sowie die fünf kalten lokalen Vergleichsstände.
