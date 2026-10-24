# Bewertung: Zugriff auf Gesundheits- und Personaldaten

Stand: 23.10.2026. Slice 3841-d zu [#3841][issue], Teil von #3846.
Bewertung, keine Freigabe oder Bestätigung des Betriebs.

## Ergebnis und Nachweisgrenze

**Bewertung:** Kategorie-Rechte und verpflichtende Zugriffsprotokolle für
sensible Dokumentdownloads und Finanzdaten sind eine brauchbare Grundlage.
Sie erfassen aber nicht alle Gesundheits- und Personaldaten: Insbesondere
`health_info`, Krankheitsstatus und Personalabwesenheiten sind über breitere
Rechte lesbar. Eine pauschale Ausnahme „gewöhnliches Personal braucht keinen
weiteren Schutz“ wäre deshalb nicht begründet. Auch die geplante
MFA-Untergrenze aus PR #4835 schließt diese Lücke nicht vollständig.
Die Entscheidung über eine eigene Stufe für `health_info` bleibt offen.
Belege: [K1], [K2], [K4], [K5], [K6], [M1].

**Maßstab:** Schutzbedarf und mögliche Folgen bestimmen die Maßnahmen, nicht
eine pauschale Technologiepflicht. Die gesonderte Prüfung der Rechtsgrundlagen
für Gesundheitsdaten bleibt in #3844; Zugangsschutz ersetzt sie nicht.
Grundlage: [rechtliche Einordnung][legal], „Maßstab“ und „Rechtsgrundlagen“,
sowie [Sächsische Datenschutzaufsicht][aufsicht], „Anforderungen“ und
„Sicherheit der Verarbeitung“.

| Status | Reichweite |
| --- | --- |
| Im Code nachvollzogen | Statische Prüfung von `origin/development` bei Commit `c66191b29879360e1eb6085ce46d716080a3b3be`. Genannte Tests sind vorhandene Verträge, hier nicht ausgeführt. |
| Vorgeschlagen, nicht Basisstand | PR [#4835][mfa-pr], beim Abruf offen. Rechteklassifikation bei Head `e796c6321998ea59a2d098d8fc5e5a22a9be553c` geprüft [M1]. Kein Merge- oder Auslieferungsnachweis. |
| Produktiv nachzuweisen | Sämtliche Maßnahmen: Version, tatsächliche Rechtevergabe, RLS, MFA-Einstellungen, Sitzungsentzug und Protokolle wurden nicht am produktiven System geprüft. |
| Offen | Schutzentscheidung für breit lesbare Gesundheits-/Personaldaten, erforderliche Folgemaßnahmen, kontobezogener Anmeldeversuchsschutz (3841-c) und menschliche Nachweise. Die Bewertung allein erfüllt AK4 „angemessen absichern“ nicht. |

Die [Eltern-Anmeldung](eltern-anmeldung-bewertung.md) bewertet AK6 getrennt.
Der historische Befund „Eltern haben Passkeys“ ist kein Istnachweis.

## Wer sieht was? Im Code nachvollzogen

Entscheidend sind Berechtigungen und Mandant, nicht der Rollenname.
`admin:*` und `*:*` erfüllen die geprüften Rechteprüfungen als Wildcards [K7].
Die Tabelle beschreibt die geprüften Pfade, nicht alle Produktdatenflüsse oder
die tatsächliche Rechtevergabe einer Schule.

| Rolle/Recht im jeweiligen Mandanten | Datenarten und Grenze | Beleg |
| --- | --- | --- |
| `students:read` ohne `users:read`, etwa Systemrolle Gast | Kindersuche/Detailansicht ohne `health_info`, Krankheitsstatus und sensible Freitextnotizen. Kein Exportzugang über dieses Recht allein. | [K1], [K2], [T1] |
| `users:read` oder Admin-Wildcard | `health_info`, `extra_info`, Begleitnotiz sowie Krank-/Entschuldigtstatus in Kind-Listen/Details. **Keine eigene Gesundheitsstufe.** Die Befüllung hängt von `canReadSensitiveStudentData` ab, nicht von `has_full_access`. Adressen und Planung haben eine zusätzliche Personal-/Adminprüfung. Personalzugriff ist grundsätzlich schulweit, nicht auf die eigene Gruppe beschränkt. | [K1], [K2], [K3] |
| `users:read` am Exportpfad | Gesundheitsliste mit Gesundheitsfreitext, ohne zusätzliches `student_documents:health`. Fehlender Audit-Dienst oder Schreibfehler verweigert die Dateifreigabe. | [K1], [K8], [T2] |
| `student_documents:health` plus Kindzugriffsprüfung | Attest, Impfnachweis, Medikamentenplan. Kategorieprüfung für Liste, Lesen, Upload und Löschen. Sorgerechtsdokumente: `student_documents:legal`; andere Dokumente: `users:update`. Sensible Downloads verlangen ein erfolgreiches Zugriffsprotokoll. | [K9], [K10], [T3] |
| `enrollment_uploads:read` plus `config:manage` | Dateien, die Eltern in der Anmeldung hochladen, etwa Einkommensnachweise (#4783). Die Anmeldungsdetails mit `config:manage` zeigen nur Dateinamen; Öffnen verlangt das eigene Recht. Jeder Abruf schreibt vorher einen Eintrag in `audit.data_access_log` (`enrollment_upload_view`); ohne Eintrag keine Datei. Ablage privat und schulgebunden, gelöscht nach `enrollment.upload_retention_days` ab der Entscheidung. | #4783 |
| `staff:stammdaten` oder `time_tracking:manage` beim Lesen; `staff:stammdaten` für Abschnittsänderungen | Personalstammdaten mit Geburtstag, privaten Kontakten, Vertrag und Qualifikationen anderer Beschäftigter. Personalnummer und Arbeitszeitverwaltung haben eigene Routen mit `time_tracking:manage`. Bank-/Steuerdaten bleiben getrennt. | [K4] |
| `staff:financial` | Bankverbindung, Steuer-ID, Sozialversicherungsnummer und Lohnabrechnungen. Finanzansicht zunächst maskiert; Offenlegung separat. Beide Finanzlesewege protokolliert und bei Auditfehler verweigert. Tatsächlicher Bezeichner: `staff:financial`, nicht `staff_financial`. | [K4], [K5], [K6], [T4] |
| `staff_documents:health` bzw. `staff:documents` | AU-Dokumente erfordern das Gesundheitsrecht, allgemeine Personalunterlagen das Dokumentrecht, Lohnabrechnungen `staff:financial`. Tab-Zugang gibt nicht alle Kategorien frei; sensible Downloads sind protokollpflichtig. | [K4], [K6], [K11], [T4] |
| `vacation:approve` oder `time_tracking:manage` | Abwesenheiten anderer Beschäftigter derselben Schule. Das Modell enthält den Typ `sick`; das AU-Dokumentrecht schützt nicht sämtliche Krankheitsinformationen. | [K4], [K12], [K20] |

**Bewertung:** Gesundheitliche Freitexte können ebenso schutzbedürftig sein wie
ein Attest. „Dokument versus Freitext“ begründet keine pauschal niedrigere
Schutzstufe. Schulweiter Zugriff erhöht die möglichen Folgen einer
Kontoübernahme. Grundlage: [K2], [K3], [K9], [legal].

## Vorhandene Schutzmaßnahmen und Grenzen

1. **Mandantentrennung:** `TenantTxMiddleware` nutzt bei validiertem Mandanten
   dessen Transaktion; `WithinTenant` setzt `phoenix_tenant` und
   `app.current_tenant_id`. Die RLS-Provisionierung erzwingt den Tenantvergleich
   für Lesen und Schreiben. Das beschränkt nicht auf eine Betreuungsgruppe und
   schützt nicht vor einem übernommenen berechtigten Konto. Plattformzugriffe
   ohne Tenant nutzen einen gesonderten Adminpfad. [K13]
2. **Rechte und Protokolle:** Kategorieprüfung und Audit vor sensiblen Downloads,
   protokollierte Finanzansicht/-offenlegung und Gesundheitslisten-Audit sind
   in diesen Pfaden umgesetzt. Daraus folgt **kein Nachweis einer vollständigen
   Protokollierung jedes `health_info`-Lesens**. Die geprüfte Kinddetail-Ausgabe
   befüllt das Feld ohne entsprechenden eigenen Auditaufruf. Risikobewertung:
   Ausdrucke und heruntergeladene Kopien bleiben außerhalb späteren
   Sitzungsentzugs. [K2], [K5], [K6], [K8], [K9], [K14]
3. **MFA im Basisstand:** Schul-MFA ist konfigurierbar, Registry-Default
   `off`. Policy und Overrides können MFA verlangen oder über `force_off`
   ausschalten. Der Passwortlogin kann Enrollment/Challenge verlangen;
   ein gültiges gemerktes Gerät kann eine erneute Challenge ersetzen.
   Eine unabänderliche MFA-Untergrenze für sensible Schulrechte ist damit
   nicht belegt. Operatorlogin nutzt bei konfiguriertem Gate
   Enrollment/Challenge; ohne konfiguriertes Gate stellt der Code direkt
   eine Sitzung aus. Reale Verdrahtung und Betrieb bleiben nachzuweisen.
   [K15], [K16], [K17], [K18], [K19]
4. **MFA-Vorschlag #4835:** Die Klassifikation umfasst u. a. Admin-/Verwaltungsrechte,
   `staff:financial`, `staff_documents:health`, `student_documents:health`
   und Rechte mit Aktion `health`. `users:read`, `staff:stammdaten`,
   `time_tracking:manage` und `vacation:approve` allein fallen nicht darunter.
   Deshalb benötigen insbesondere Gesundheitsfreitexte und
   Krankheitsabwesenheiten eine eigene Schutzentscheidung.
   Sitzungserneuerung, Bestand und Recovery sind mit dem endgültigen
   MFA-Slice zu prüfen, nicht durch diese Doku als bestanden erklärt.
   [M1], [K2], [K4], [K12]

## Restrisiken und Empfehlung

Qualitative Bewertung möglicher Folgen, keine Messung von Angriffshäufigkeiten
oder tatsächlichen Vorfällen:

| Szenario | Einschätzung und empfohlene Behandlung |
| --- | --- |
| Wiederverwendetes/abgegriffenes Personalpasswort mit breiten Leserechten | Hohe mögliche Folgen durch schulweiten Gesundheitszugriff. Keine generelle MFA-Ausnahme für Betreuungspersonal. Rechte auf nötige Konten begrenzen und für diese einen wirksamen zweiten Nachweis vorsehen. `required_all` ist eine zu prüfende Übergangsoption, keine hier umgesetzte Garantie. Override-, Passkey-, Bestands- und Recoverypfade dürfen den gewählten Schutz nicht umgehen. [K2], [K3], [K15], [K16], [M1] |
| Übernommene reine Personalverwaltungs-/Abwesenheitsrolle | Hohe mögliche Folgen bei privaten Personalangaben und Krankheitstagen. MFA-Schutzbedarf auch für `staff:stammdaten`, `time_tracking:manage` und `vacation:approve` gesondert bewerten, nicht aus fehlendem `*:health` auf Unbedenklichkeit schließen. [K4], [K12], [M1] |
| Unnötiges Lesen oder unkontrollierte Kopien durch berechtigte Konten | MFA ersetzt keine Zweckbindung. Schule muss Aufgabenrechte, Vertretung, Entzug und Umgang mit Export-/Papierkopien regeln. Vorhandene Audits nutzen; Leselog-Lücke separat entscheiden. [K2], [K6], [K8], [K9], [legal] |
| Falsch kategorisierte Datei oder Gesundheitsangabe in allgemeiner Notiz | Kategorie-Rechte schützen nur richtig zugeordnete Inhalte. Datenminimierung und Erfassungs-/Korrekturregeln nötig; kein pauschales Feldverschlüsselungsprojekt ableiten. [K2], [K9], [K11], [legal] |
| Verteilte Anmeldeversuche und kompromittierter Wiederherstellungskanal | Fehlenden kontobezogenen Login-Schutz in 3841-c behandeln. Ein weiterer Mailcode beseitigt eine Mailkontoübernahme nicht. Elternpfade und Belege: [Eltern-Bewertung](eltern-anmeldung-bewertung.md). MFA und Anmeldebremsen ergänzen sich. |

## Offene Entscheidung: eigene Stufe für `health_info`

**Keine Produktentscheidung in diesem Slice.** Optionen aus den belegten
Zugriffspfaden [K1], [K2], [K3], [K8], [M1]:

| Option | Folgen und notwendige Absicherung |
| --- | --- |
| `users:read` beibehalten | Einfacher Betreuungszugriff, aber weiter schulweite Gesundheitsfreitexte für einen breiten Kreis. Erfordert dokumentierten Bedarf aller Rollen, wirksamen Kontoschutz einschließlich MFA nach Schutzbedarf und entschiedenes Protokollierungskonzept. #4835 allein reicht nicht. |
| `student_documents:health` auch auf das Feld anwenden | Nutzt vorhandenes Recht und die geplante MFA-Klassifikation. Koppelt jedoch nötige Notfallhinweise mit Zugriff auf vollständige Gesundheitsdokumente. Bestandsrechte und Betreuungsfähigkeit vor Umstellung prüfen. |
| Eigenes Recht für Gesundheitsfreitext | Genauere Minimalrechte und eigene MFA-Einordnung möglich. Zusätzlicher Pflegeaufwand für Rollen, UI, Elternschreibwege, alle Listen/Details/Exporte und Bestand. Notfallzugang bei Bedarf eng begrenzt und protokolliert gestalten, nicht als ungeschützte Ausnahme. |

**Empfehlung für den Entscheidungs-Slice:** Die dritte Option gegen den
Betreuungsbedarf prüfen. Vor Einschränkungen legen Schule und Produkt fest,
wer im Alltag und bei Vertretung welche Notfallinformation braucht.
Fehlende Angaben dürfen nicht als „keine gesundheitliche Einschränkung“
erscheinen; die Gesundheitsliste unterscheidet bereits fehlende Einträge
und ihren Filterumfang [K8]. Abnahme muss Listen/Details, Exporte,
Elternpflege, Bestandsrollen, MFA/Recovery und Fehlerfälle einschließen.

## Offene Abnahme und Mensch-Schritte

1. **Schule/Verantwortlicher:** Aufgabenrechte und Rechtsgrundlagen je Datenart
   festlegen; `health_info`-Optionen und Restrisiken entscheiden.
   Keine personenbezogenen Berechtigungslisten in Doku oder PR aufnehmen.
2. **Umsetzungsverantwortliche:** MFA-Untergrenze und 3841-c abschließen;
   breit lesbare sensible Daten berücksichtigen. Bestandsrollen, Sessions,
   Recovery, Overrides und Entzug separat abnehmen.
3. **Betriebsverantwortliche, noch offen:** Nach Auslieferung Version,
   Einstellungen und Rechteverteilung prüfen. Mit synthetischen Prüfdaten
   unberechtigte Rollen, fremde Mandanten, Export-/Auditfehler,
   MFA-Bestandssitzungen und Wiederherstellung nachvollziehen.
   Nur Datum, Version, Prüffall und Ergebnis ohne Dateninhalte festhalten.
   Für diese Bewertung wurde kein Serverzugriff ausgeführt und kein neues
   produktives Prüfverfahren als getestet nachgewiesen.
4. **Vertragsverantwortliche:** TOM-Zusage mit Ergebnissen abgleichen.
   Erst nach erfolgreichem Nachweis als produktiv bestätigt kennzeichnen.
   Teilfehler und nicht geprüfte Pfade bleiben offen.

**Vorhandene Testverträge, hier nicht ausgeführt:** [T1] prüft Redaktion
sensibler Kindfelder; [T2] Exportrecht und Audit beim Gesundheitslistenexport;
[T3] Kategoriezugriff, sensible Downloads und Auditfehler bei Kinddokumenten;
[T4] Finanz-/Dokumentzugriff mit Verweigerung bei Auditfehler; [T5] die
RLS-Katalogabdeckung einer Testdatenbank. Kein neuer Laufzeitnachweis und
keine vollständige AK7-Abnahme.

## Vorschlag AVV/TOM

Nur Textvorschlag für [AVV Anhang 1 Ziff. 2, 3 und 6][avv]; keine Änderung im
Legal-Repo. Dessen Zusage „zugeordnete Standorte“ ist nicht als Beschränkung
auf Betreuungsgruppen zu lesen [K3]. Folgenden Zusatz erst nach Entscheidung,
Umsetzung und betrieblichem Nachweis als bestätigten Iststand übernehmen:

> Gesundheitsdokumente von Kindern und Beschäftigten sowie Finanzdaten
> benötigen gesonderte Aufgabenrechte. Gesundheitsfreitexte und
> Krankheitsabwesenheiten werden in die Schutzbedarfsbewertung einbezogen;
> eine reine Personalrolle begründet keine Ausnahme von erforderlichem
> Kontoschutz. Erforderliche zweite Anmeldenachweise müssen auch bei
> Bestandskonten, Sitzungserneuerung und Wiederherstellung wirksam bleiben.
> Innerhalb einer Schule besteht derzeit bei entsprechendem Leserecht
> schulweiter Zugriff auf Gesundheitsfreitexte, keine Beschränkung auf die
> eigene Betreuungsgruppe. Sensible Dokumentdownloads, Finanzansichten und
> Gesundheitslistenexporte werden in den beschriebenen Pfaden protokolliert
> und bei fehlgeschlagener verpflichtender Protokollierung verweigert.
> Eine vollständige Protokollierung jedes Gesundheitsfreitext-Lesezugriffs
> wird damit nicht zugesagt. Zugriffsstufe und Restrisiken werden mit der
> verantwortlichen Schule dokumentiert.

Bis dahin: **Bewertung vorhanden; Schutzentscheidung, Umsetzung offener
Maßnahmen und produktive Wirksamkeit nicht bestätigt.**

## Fundstellen

Code-Links sind auf den Basiscommit fixiert; Funktionsnamen benennen den
Belegumfang. Spätere Änderungen erfordern Neubewertung. Legal-Quellen wurden
nur gelesen: [Einordnung][legal], [interne Checkliste][checkliste], [AVV][avv].

[issue]: https://github.com/moto-nrw/project-phoenix/issues/3841
[legal]: https://github.com/moto-nrw/legal/blob/7a21f6742782bcf6141470a47333ae5be6c3182f/research/14-dsgvo-mindestumfang.md
[checkliste]: https://github.com/moto-nrw/legal/blob/7a21f6742782bcf6141470a47333ae5be6c3182f/vertragspaket/intern/checkliste.md
[avv]: https://github.com/moto-nrw/legal/blob/7a21f6742782bcf6141470a47333ae5be6c3182f/vertragspaket/03-avv.md
[aufsicht]: https://www.datenschutz.sachsen.de/technischer-und-organisatorischer-datenschutz.html
[mfa-pr]: https://github.com/moto-nrw/project-phoenix/pull/4835
[K1]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/presenceprojection/http/student_resource.go#L54-L75
[K2]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/presenceprojection/http/response_helpers.go#L325-L422
[K3]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/securityruntime/internal/application/authorize/student_access.go#L17-L38
[K4]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/workforce/http/timetracking/staff_admin.go#L135-L281
[K5]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/workforce/internal/application/staffadmin.go#L218-L351
[K6]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/workforce/internal/application/staffadmin_documents.go#L35-L178
[K7]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/securityruntime/internal/application/authorize/permission.go#L5-L51
[K8]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/presenceprojection/http/health_list_export.go#L18-L122
[K9]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/careplan/internal/application/care_documents.go#L69-L247
[K10]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/careplan/http/document_permissions.go#L5-L11
[K11]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/workforce/compose/staffadmin.go#L49-L59
[K12]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/workforce/absence.go#L7-L100
[K13]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/docs/agents/contracts.md#tenant-boundary
[K14]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/presenceprojection/http/student_handlers.go#L61-L107
[K15]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/application/account_mfa_flow.go#L71-L179
[K16]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/domain/mfa.go#L139-L186
[K17]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/settings/internal/defaults/security.go#L35-L59
[K18]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/application/account_login.go#L50-L95
[K19]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/application/operator_authentication.go#L75-L129
[K20]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/workforce/http/timetracking/staff_admin_sessions.go#L540-L582
[T1]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/presenceprojection/http/student_read_permissions_internal_test.go
[T2]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/presenceprojection/http/health_list_export_routes_test.go
[T3]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/careplan/contracttest/lifecycle_student_document_service_test.go
[T4]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/workforce/internal/application/staffadmin_test.go
[T5]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/database/migrations/rls_coverage_test.go
[M1]: https://github.com/moto-nrw/project-phoenix/blob/e796c6321998ea59a2d098d8fc5e5a22a9be553c/backend/modules/identityaccess/internal/domain/mfa_privilege.go#L5-L41
