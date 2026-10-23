# Operator-Zugriffe und Audit-Ausfälle (Slice 3842-b)

## Im Code umgesetzt

Die folgenden Operator-Ansichten schreiben vor dem Lesen einen Eintrag mit
`action=view` nach `platform.operator_audit_log`. Alle Pfade beginnen mit
`/operator`:

| Ansichtskategorie | GET-Pfad | Bezug |
| --- | --- | --- |
| `accounts` | `/accounts` | globale Kontoliste |
| `school_accounts` | `/schools/{id}/accounts` | Schul-ID |
| `organization_accounts` | `/organizations/{id}/accounts` | Träger-ID |
| `school_persons` | `/schools/{id}/persons` | Schul-ID |
| `organization_persons` | `/organizations/{id}/persons` | Träger-ID |
| `school_account_caregiver` | `/schools/{id}/accounts/{accountId}/caregiver-capability/` | Schul- und Konto-ID |
| `school_account_mfa` | `/schools/{id}/accounts/{accountId}/mfa/` | Schul- und Konto-ID |
| `account_mfa` | `/accounts/{accountId}/mfa/global-override` | Konto-ID |
| `account_tenants` | `/accounts/{accountId}/tenants` und `/accounts/{accountId}/tenants/` | Konto-ID |
| `announcement_views` | `/announcements/{id}/views` | Ankündigungs-ID als `resource_id`, Ressourcentyp `announcement` |
| `staff_person_merge_candidates` | `/staff-person-merges/{schoolId}` | Schul-ID |

Die Ankündigungsansicht enthält schulübergreifende Kontonamen und Lesestatus.
Sie nutzt dieselbe verpflichtende Lesesperre wie die Kontolisten.
Die Vorschau zum Zusammenführen doppelter Mitarbeitendendatensätze protokolliert
ebenfalls einen Lesezugriff über ihren vorhandenen Workflow-Audit-Port. Dieser
Eintrag teilt die bestehende administrative Anfrage-Transaktion; scheitert das
Lesen danach, rollt auch der Eintrag zurück. Die HTTP-Schicht reicht die geparste
Anfrage-IP durch den Vorschau- und Audit-Port bis in `request_ip` weiter.

Ein Eintrag bezeichnet einen **Zugriffsversuch**, keinen bestätigten Empfang.
Er enthält Operator-ID, Zeitpunkt, Anfrage-IP, eine feste Ansichtskategorie und
gegebenenfalls Schul-, Träger-, Konto- oder Ankündigungs-ID. Keine Ergebniszeilen, Namen,
E-Mail-Adressen, Suchbegriffe, Anfragekörper oder Zugangsschlüssel. Es entsteht
ein Eintrag pro Anfrage, auch bei leeren Listen; spätere Lesefehler können einen
bereits protokollierten Versuch hinterlassen. Die bestehenden Operator-Sitzungs-
und Berechtigungsprüfungen bleiben davor bestehen.

`GET /operator/invitations/` bleibt bewusst **ohne Lese-Audit**. Es zeigt
Einladungen interner moto-Teammitglieder samt E-Mail-Adressen, keine Schul- oder
Elternkonten und keine schulübergreifende Nutzeraktivität. Dieser Slice schützt
die oben abgegrenzten fachlichen Personen-/Kontozugriffe und erweitert nicht
pauschal alle internen Teamverwaltungsansichten. Der Ausschluss ist eine
Umfangsentscheidung, keine Aussage, dass Einladungsadressen unkritisch seien.

Folgende Änderungen geben Audit-Fehler an ihre bestehende administrative
Transaktion zurück und rollen mit ihr zurück:

- Geräte-Schlüsselrotation, Geräte-Löschung und Schultransfer eines Geräts.
- Soft-Delete von Schule, Träger und Person. Die Träger-Löschung war bereits
  transaktional protokolliert; sie verwendet nun denselben Fehlercode.
- Schulbezogener und globaler MFA-Override, auch das Entfernen eines Overrides,
  sowie das Zurücksetzen der MFA-Anmeldung durch einen Operator. Das neue
  schulbezogene Operator-Protokoll speichert keine Freitextbegründung. Die
  HTTP-Schicht bindet die geparste Anfrage-IP als request-lokale Audit-Metadaten;
  MFA-Kommandos reichen sie über ihren bestehenden Audit-Port weiter. Bei
  Aufrufen ohne HTTP-Anfrage bleibt eine unbekannte Quelladresse unbekannt.

Bei diesen Audit-Ausfällen und bei nicht protokollierbaren Lesezugriffen antwortet
die API mit `503`, Code `operator.audit_unavailable`. Der Operator erhält eine
Fehlermeldung; es gibt keinen Erfolg und keine Datenfreigabe. Provisioning und MFA melden den Audit-Ausfall im Betriebslog mit fester
Aktionskategorie, ohne Anfrageinhalt. Beim Merge-Workflow macht die bestehende
Anfrageprotokollierung die abgelehnte Antwort sichtbar.
Andere bisherige best-effort-Auditpfade werden nicht pauschal umgestellt.
Eine künftige endgültige Mandantenlöschung muss ihren Audit-Schritt ebenfalls
in ihre Löschtransaktion aufnehmen; dieser Slice implementiert sie nicht.

Das Schema erlaubt die neue Aktion ohne Migration. Eigentümer des Protokolls,
Datenbankrechte und Löschlauf bleiben unverändert. Die Aufbewahrung endet
weiterhin am Ende des Folgejahres, siehe
[Fristenliste](../reference/loeschfristen.md). Es wird keine zusätzliche Frist
und keine allgemeine gesetzliche 30-Tage-Frist eingeführt.

## Lokaler Nachweis

Die Tests verwenden ausschließlich synthetische Fixtures und isolierte
Testdatenbanken. Ein Trigger verweigert das Einfügen von Audit-Zeilen. Geprüft
werden Rollback von Schlüssel, Transfer, Löschungen und MFA-Zustand sowie die
Sperre der gemounteten Personen-/Kontenlisten und Ankündigungs-Lesedetails.
Der Ankündigungstest prüft einen Eintrag mit ID und Anfrage-IP sowie bei
Audit-Ausfall `503 / operator.audit_unavailable` ohne Kontodaten.
Port-Fehlerinjektion prüft auch
Audit-Ausfälle, die PostgreSQL nicht selbst in einen Fehlerzustand versetzen.
HTTP-Regressionstests prüfen die gespeicherte Quelladresse für Merge-Vorschau,
Operator-MFA und Provisioning-Leselisten, einschließlich direkter IPv4-Adressen
und der vom vertrauenswürdigen Proxy gewählten IPv6-Adresse.

```sh
scripts/run-go-toolchain.sh go -C backend test -tags=testhooks ./api \
  -run 'TestOperatorAudit|TestOperatorMFAAuditsPersistRequestIP|TestStaffPersonMergePreview|TestOperatorPersonAndAccountRoutes|TestFullProductionRouterGolden' -count=1
scripts/run-go-toolchain.sh go -C backend test -tags=testhooks \
  ./modules/communication/... \
  ./modules/organizationtenancy/internal/application \
  ./modules/organizationtenancy/inbound/operator
```

## Produktiv durch Menschen nachzuweisen

Dieser Code-Nachweis bestätigt keinen produktiven Stand. Nach dem Rollout muss
der Betrieb mit einem freigegebenen internen Testkonto prüfen, dass die neuen
`view`-Einträge ankommen und Audit-Ausfälle als Betriebsfehler erkannt werden.
Fehlerinjektion nur in einer isolierten Testumgebung durchführen. Keine echten
Schlüssel, Namen, Adressen oder Antwortinhalte als Nachweis übernehmen.

Zu bestätigen bleiben die produktiven Protokollrechte, Überwachung der
Audit-Ausfälle und der bereits vorhandene Löschlauf. Das Ergebnis wird vom
Orchestrator im Ergebnisbericht 3842-f zusammengeführt. Dieser Slice bearbeitet
dessen Datei nicht.
