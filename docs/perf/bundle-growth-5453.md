# Bundle-Zuwachs seit dem 28.10.2026 (#5453)

## Entscheidung

Keine pauschale Budgeterhöhung: #5408 hat den historischen Zuwachs auf den
16 App-Routen bereits mehr als ausgeglichen. Nur `/help/[[...topic]]` benötigt
eine neue Referenz. Die Routensumme enthält sowohl gemeinsame Fehlerdarstellung als auch
zusätzliche Hilfeartikel und Symbole; ihr alter Deckel wird noch um rund 1 kB
überschritten. Ohne die zusätzlichen Hilfemodule und Hilfe-Icons läge sie
rechnerisch unter dem alten Deckel. Delta-Toleranz und die
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

Die Tabelle enthält alle 22 archivierten Feature-Zwei-Baum-Vergleiche aus
`featureComparisons` der Evidenzdatei. #5433 und #5456 liegen nach #5408 und
zeigen den späteren Stand, nicht zusätzliche Ursachen des historischen Zuwachses.

| PR / gemessener Feature-Head | Module und Zweck | Delta Start / Hilfe / Eltern-Login | Linux-Run |
|---|---|---:|---|
| #5227, `d4742c5ed3` | Fehlercodes/-texte für Leitungsnachrichten | +283 / +283 / +370 B | [40874459376](https://github.com/moto-nrw/project-phoenix/actions/runs/40874459376) |
| #5249, `05a6eaef97` | Nachrichtenstatus, Mitlesende, Hilfe | 0 / +526 / +1027 B | [40874487582](https://github.com/moto-nrw/project-phoenix/actions/runs/40874487582) |
| #5276, `38203a072f` | Nachweis-Upload in Ferienanmeldungen und zugehörige Hilfe | 0 / +179 / 0 B | [40883123180](https://github.com/moto-nrw/project-phoenix/actions/runs/40883123180) |
| #5288, `1611e5f9dd` | Passwort-Zurücksetzen im Träger-Portal, Fehlertexte und Hilfe | +51 / +80 / +91 B | [40866854951](https://github.com/moto-nrw/project-phoenix/actions/runs/40866854951) |
| #5292, `ab891e83a7` | Belegung nach Uhrzeit und zugehörige Hilfe | 0 / +197 / 0 B | [40880423418](https://github.com/moto-nrw/project-phoenix/actions/runs/40880423418) |
| #5298, `1946a9d92c` | Betreuungsplan-Auswirkung mit UI-Kit-Bausteinen | 0 / +11 / 0 B | [40885705026](https://github.com/moto-nrw/project-phoenix/actions/runs/40885705026) |
| #5315, `c9d10d4d15` | Soll-/Ist-Betreuungszeiten und zugehörige Hilfe | 0 / +643 / +30 B | [41013625443](https://github.com/moto-nrw/project-phoenix/actions/runs/41013625443) |
| #5339, `7a22749894` | Unfallanzeige: Fehlerkatalog, Hilfeartikel, FirstAidKit-Icon | +321 / +1673 / +344 B | [40971185508](https://github.com/moto-nrw/project-phoenix/actions/runs/40971185508) |
| #5259, `796eee6b24` | gemeinsame Kontoprüfung, `session-account-binding` | +781 / +234 / +1046 B | [41081035949](https://github.com/moto-nrw/project-phoenix/actions/runs/41081035949) |
| #5376, `e8da421c1f` | Lohnbüro-Versand und zugehörige Hilfe | +125 / +585 / +107 B | [41092916445](https://github.com/moto-nrw/project-phoenix/actions/runs/41092916445) |
| #5377, `9b7d8bb876` | Wochenprotokolle: Hilfe, Notebook-Icon in der damaligen vollständigen Konzept-Registry, Fehlertexte | +243 / +1600 / +324 B | [41193004602](https://github.com/moto-nrw/project-phoenix/actions/runs/41193004602) |
| #5132, `621cf148ca` | Fehlzeitenliste und zugehörige Hilfe | +120 / +685 / +143 B | [41198465692](https://github.com/moto-nrw/project-phoenix/actions/runs/41198465692) |
| #5109, `3f18a6ca23` | Elternsprechtag: Fehlercodes/-texte und Hilfe | +843 / +1788 / +1045 B | [41206397194](https://github.com/moto-nrw/project-phoenix/actions/runs/41206397194) |
| #5258, `2898931d59` | gemeinsame SWR-Key-Factories und geschlossene Cache-Zugriffe | +612 / +20 / +1496 B | [41206620338](https://github.com/moto-nrw/project-phoenix/actions/runs/41206620338) |
| #5413, `c5f1145ffd` | Optimistisches Senden von Nachrichten und SWR-Generationsgrenzen | +160 / 0 / +184 B | [41217148254](https://github.com/moto-nrw/project-phoenix/actions/runs/41217148254) |
| #5414, `bf18105151` | Listendaten als Detail-Vorschau und paralleles Nachladen | +115 / 0 / +115 B | [41218795958](https://github.com/moto-nrw/project-phoenix/actions/runs/41218795958) |
| #5421, `7e2f468f9f` | Nachrichtenstatus bei ausgeschaltetem Elternnachrichten-Modul | +216 / +270 / +216 B | [41235802160](https://github.com/moto-nrw/project-phoenix/actions/runs/41235802160) |
| #5425, `ea39911c95` | Optimistische Alltagsaktionen und zugehörige Hilfe | 0 / +57 / -101 B | [41245601351](https://github.com/moto-nrw/project-phoenix/actions/runs/41245601351) |
| #5403, `db6099854c` | sitzungsgebundene Antwortsperre, `session-end-barrier`, Portal-Provider | +345 / +176 / +1357 B | [41251067700](https://github.com/moto-nrw/project-phoenix/actions/runs/41251067700) |
| #5427, `800e119579` | Vorabladen von Nachrichtendetails und Vorschau-Wechsel | 0 / 0 / 0 B | [41242468797](https://github.com/moto-nrw/project-phoenix/actions/runs/41242468797) |
| #5433, `156f45c6d9` | Gemeinsamer Ressourcen-Cache im Eltern-Portal, nach #5408 | +55 / 0 / +1990 B | [41277751229](https://github.com/moto-nrw/project-phoenix/actions/runs/41277751229) |
| #5456, `1eec6b529f` | SSE-/Polling-Abgleich und Zeitfenstergrenzen, nach #5408 | +1320 / 0 / +629 B | [41276170050](https://github.com/moto-nrw/project-phoenix/actions/runs/41276170050) |

Zusätzliche Messnachweise außerhalb dieser 22 Feature-Paare:

- #5182 (`81a68c7792`, `86dd587225`): Der direkte Vergleich der
  aufeinanderfolgenden Merges `7d898cb22d` → `a3a1d0de26` misst
  +4164 / −26 / +3949 B. Beide archivierten Quellbäume stimmen exakt mit
  diesen Merges überein ([40754352528](https://github.com/moto-nrw/project-phoenix/actions/runs/40754352528)).
- #5445 (`ec26ae4be1`): +18 / 0 / +18 B eigenes PR-Delta, nicht die Altlast
  ([41256428322](https://github.com/moto-nrw/project-phoenix/actions/runs/41256428322)).

Für die 22 Feature-Paare enthält die Evidenzdatei beide Quellidentitäten,
Revisionen und die Deltas aller 17 Routen. Die Tabelle nennt die gemessenen
Feature-Heads; ihre Zahlen sind keine isolierten Kosten eines einzelnen Commits.

### Grenze der commitweisen Zuordnung

Gemessen sind **21 von 79 Frontend-Merges** im historischen Bereich
`a9da0bcbc0` → `cb4d7d12e2`: 20 Feature-Paare vor #5408 und der direkte
Vergleich für #5182. Gezählt werden First-Parent-Merges mit Änderungen unter
`frontend/src/`; reine Änderungen an Frontend-Werkzeugen, Budgets oder anderer
Dokumentation zählen nicht zu diesen 79. Das ist keine vollständige Zuordnung
des historischen Zuwachses zu allen Commits.

Grenze auf den App-Routen: Auf der Tenant-Startseite und den Eltern-Routen
bleiben **4,8 bis 8,5 kB (grob 25 bis 45 %)** als „Übrige
Shell-/Seiten-/Chunk-Änderungen“ ohne Zahlen je Commit. Für
`/[tenant]/display` weist die Bilanz zusätzlich einen Rest von 4125 B aus.
Die folgenden Modulgruppen schließen die Größenbilanz, ersetzen aber keinen
commitweisen Nachweis dieser verbleibenden Anteile.

### Modulbilanz der historischen Endpunkte

Kalter Vergleich `8d02dc39a4` → `cb4d7d12e2`, Linux ARM64. Gemeinsamer
Fehlercode-Registeranteil: +1009 B; Fehlertextkatalog: +3137 B auf jeder der
17 Routen. `220a468ae0` (#5243) führte die Registeraufteilung ein; neue
Fehlercodes und übersetzte Meldungen aus den oben genannten Features wachsen
zusätzlich mit. `ToastContext.tsx` lädt `error-presentation` per dynamischem
`import()` erst bei Bedarf; von dort werden Register und Katalog eingebunden.
`bundle-routes.mjs` zählt alle Client-Chunks einer Route, einschließlich der
dynamisch erreichbaren Chunks. Diese gemeinsame Last in der Routensumme ist
also kein Beleg für einen entsprechenden initialen JavaScript-Transfer.

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

Die Referenz wächst gegenüber 715670 B um **15343 B**. Rund **4,1 kB** davon
entfallen auf das gemeinsame Fehlercode-Register und den Fehlertextkatalog
(+1009 B und +3137 B im historischen Modulvergleich). Dieser Anteil wird aus
`ToastContext.tsx` dynamisch geladen und mitgezählt, weil der Bundle-Parser
alle Client-Chunks erfasst, nicht nur die beim ersten Seitenaufruf geladenen.
Die Budgetbegründung darf daher nicht den gesamten Zuwachs als Hilfeinhalt
oder initiale Last darstellen.

Den Ausschlag für den benötigten höheren Deckel geben die zusätzlichen
**Hilfemodule (+9824 B)** und **Hilfe-Icons (+1555 B)**. Ohne diese Beiträge
läge die Route rechnerisch bei `731013 - 9824 - 1555 = 719634` B, also unter
dem alten Deckel von 729983 B. Das ist eine Größen-Gegenrechnung der gemessenen
Anteile, kein zusätzlicher Build mit entfernten Funktionen.

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
