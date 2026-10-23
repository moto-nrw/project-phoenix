# Operator-Zugriffe und Audit-Ausfälle (Slice 3842-b)

## Im Code umgesetzt

Die vollständige Inventur unten führt **98 Operator-Routen** auf: **34** mit
verpflichtendem Lese-/Freigabe-Audit, **14** begründete Ausnahmen ausschließlich
für eigene moto-Teamdaten und **50** ohne Personenobjekt in der regulären
Antwort. Sie umfasst GET, Schreibantworten und Slash-Aliasse. Maßgeblich sind
die einzelnen Tabellenzeilen, nicht eine pauschale Aussage über alle
privilegierten Zugriffe oder beliebige Freitexte.

Geschützte Antworten schreiben vor dem Handler `action=view` nach
`platform.operator_audit_log`. Dazu zählen Konten, Personen, Schulzugänge,
Betreuungs- und MFA-Details, Schul-Kontaktdaten, Schul-Einladungs-/Kontoantworten,
Ankündigungsinhalt/-Lesedetails, die Mitarbeitenden-Merge-Vorschau, das Settings-Schema,
die Wertefreigabe und die RFID-Triage. Bei Schreibantworten liegt dieser
Auditschritt **vor der Änderung**. Er ersetzt deren bestehendes Änderungs-Audit
nicht und macht nicht pauschal alle Änderungen transaktional fail-closed.

Die Buchungsfolgen-Vorschau
`GET /operator/schools/{id}/settings/booking-authority-impact` enthält je Kind
ID, Namen und Klasse. Sie erhält die eigene Kategorie
`school_booking_authority_impact`, Ressourcentyp `school`, die Schul-ID als
`resource_id` und in `changes.school_id` sowie die geparste Request-IP. Der
Auditschritt liegt vor Care Plans unveränderter schulbezogener Lesetransaktion.

`settings/values/{key}/reveal` hatte im gelesenen Settings-Pfad kein Lese-Audit.
Die neue Kategorie `school_setting_reveal` protokolliert jetzt den Versuch mit
Schul-ID, **ohne Schlüssel oder Wert**. Der bestehende Ausschluss von
`admin_only`-Einstellungen bleibt bestehen. Die Freigabefunktion liefert nicht
nur Password-Felder, sondern grundsätzlich den aufgelösten Wert eines erlaubten
Settings; auch deshalb ist die Route geschützt. Das Schema kann konfigurierte
Freitexte enthalten und nutzt `school_settings_schema`.

Die beiden `unregistered-tag-scans`-Antworten enthalten `tag_uid`, Zeitpunkte,
Schul-/Gerätebezug, `resolved_by_operator_id` und den freien `resolution_note`.
Es gibt keinen Join auf Kind-/Elternnamen. Die UID und Notiz können dennoch
Personenbezug haben, deshalb schützt `unregistered_tag_scans` GET **und** POST.
Ein neuer Audit-Eintrag enthält weder UID noch Notiz oder ungeprüfte Filter.

Ein Eintrag bezeichnet einen **Zugriffsversuch**, keinen bestätigten Empfang
und keinen Änderungserfolg. Er enthält Operator-ID, Zeitpunkt, geparste
Request-IP, feste Ansichtskategorie und gegebenenfalls IDs. Es gibt höchstens
einen Lese-Audit-Eintrag pro Anfrage, nicht pro Ergebniszeile, auch bei leeren
Listen. Keine Ergebniszeilen, Namen, E-Mail-Adressen, Suchbegriffe,
Anfragekörper, Settings-Werte oder Zugangsschlüssel im neuen Leseprotokoll.
Spätere Validierungs-/Lesefehler können einen protokollierten Versuch
hinterlassen. Beim Merge-Workflow teilt der Eintrag dagegen die bestehende
administrative Anfrage-Transaktion und rollt bei deren Fehler zurück.
Die Operator-Sitzungs- und Berechtigungsprüfungen bleiben davor bestehen.

Folgende Änderungen geben Audit-Fehler an ihre bestehende Transaktion zurück
und rollen mit ihr zurück:

- Geräte-Schlüsselrotation, Geräte-Löschung und Schultransfer eines Geräts.
- Soft-Delete von Schule, Träger und Person. Die Träger-Löschung war bereits
  transaktional protokolliert; sie verwendet denselben Fehlercode.
- Schulbezogener/globaler MFA-Override einschließlich Entfernen und Operator-
  Reset der MFA-Anmeldung. Das schulbezogene Operator-Protokoll übernimmt
  keine Freitextbegründung. Die geparste Anfrage-IP wird request-lokal über
  den bestehenden Audit-Port weitergereicht.

Bei diesen Audit-Ausfällen und bei nicht protokollierbaren geschützten Antworten
liefert die API **503 / `operator.audit_unavailable` ohne Datenfreigabe**.
Provisioning und MFA melden den Fehler mit fester Aktionskategorie, ohne
Anfrageinhalt. Andere bisherige best-effort-Änderungs-Audits werden nicht
pauschal umgestellt. CLI-Zugriffe und endgültige Mandantenlöschung sind nicht
Gegenstand dieser Operator-Routeninventur; deren gesonderte Nachweise werden
hier nicht behauptet.

Keine Migration: `action` ist bereits `VARCHAR(100)` ohne Werte-Constraint.
Protokoll-Owner, Datenbankrechte und Löschlauf bleiben unverändert. Aufbewahrung
weiterhin bis Ende des Folgejahres, siehe [Fristenliste](../reference/loeschfristen.md).
Keine zusätzliche Frist und keine allgemeine gesetzliche 30-Tage-Frist.

## Lokaler Nachweis

Die Tests nutzen synthetische Fixtures und isolierte Testdatenbanken. Ein
Trigger verweigert Audit-INSERTs. Die Buchungsfolgen-Regression war vor dem Fix
rot: kein Eintrag und 200 mit Kinderdaten trotz Audit-Ausfall. Nach dem Fix
prüft sie genau einen Eintrag mit Schulbezug und Request-IP sowie 503 mit
registriertem Fehlercode ohne Kinderdaten oder interne Diagnostik.

Die zusätzliche Routenregression prüft erfolgreiche, datenminimierte Einträge
für Schulansichten, Schema, Wertefreigabe und RFID-Triage sowie die Sperre aller
17 weiteren in Runde 3 geschützten Routen bei Audit-Ausfall. Ein Scan-Spy
belegt, dass dabei weder Leser noch Schreiber aufgerufen werden. Bestehende
Tests decken Rollback von Schlüsseln, Transfer, Löschungen und MFA sowie
Personen-/Kontenlisten, Ankündigungs-Lesedetails und Merge-Vorschau ab. Die
erweiterte Ankündigungsregression schützt zusätzlich Liste, Detail und
Schreibantworten mit freiem Inhalt; auch diese vier Fälle waren vor dem Fix rot.

Das Middleware-Golden erhält ausschließlich das zusätzliche RFID-Audit hinter
der bisherigen Operator-Authentifizierung. Route-Tabelle, Sitzungsketten,
Tenant-Transaktionen und Analytics-Zuordnung bleiben unverändert.

```sh
scripts/run-go-toolchain.sh go -C backend test -tags=testhooks ./api \
  -run 'TestOperatorAudit|TestOperatorPersonAndAccountRoutes|TestFullProductionRouterGolden' -count=1
scripts/run-go-toolchain.sh go -C backend test -tags=testhooks -p 2 -parallel 8 \
  ./modules/careplan/... ./modules/settings/... ./modules/devicefleet/... \
  ./modules/organizationtenancy/internal/application \
  ./modules/organizationtenancy/inbound/operator \
  ./modules/identityaccess/inbound/operator
scripts/run-go-toolchain.sh scripts/backend-architecture.sh check
(cd backend && ../scripts/run-go-toolchain.sh golangci-lint run --allow-serial-runners --timeout 10m)
scripts/run-go-toolchain.sh scripts/test-changed.sh origin/development
```

## Produktiv durch Menschen nachzuweisen

Dieser Code-Nachweis bestätigt keinen produktiven Stand. Nach dem Rollout:

1. Mit freigegebenem internem Testkonto für die geschützten Tabellenzeilen
   `view`-Einträge anhand fester Kategorie, IDs und Request-IP prüfen. Keine
   Namen, Antwortinhalte oder Secrets als Nachweis übernehmen.
2. Fehlerüberwachung für `503 / operator.audit_unavailable` und die
   entsprechenden Betriebslogs bestätigen. Fehlerinjektion nur isoliert;
   dort auch Schreibantworten auf ausbleibende Mutation prüfen.
3. Produktive Protokoll-Leserechte und bestehenden Löschlauf bestätigen.
4. Der Orchestrator führt die Ergebnisse in 3842-f zusammen. Diese Datei wurde
   hier nicht bearbeitet; CI-Abwarten, quorum und Merge sind nicht Teil des
   Implementierer-Auftrags dieser Runde.

## Offen und Grenzen

Produktiver Rollout, Rechteprüfung, Überwachung und Löschlauf bleiben offen.
Die 14 Team-Ausnahmen haben durch diesen Slice kein neues Lese-Audit; die
Tabelle nennt ihre Inhalte und Begründung einzeln. Freitextfelder werden nicht
inhaltlich auf beliebige zweckwidrige Personendaten geprüft. Es wird keine
vollständige Protokollierung aller internen oder CLI-Zugriffe behauptet.

Die bestehenden MFA-Detailhandler behandeln Lesefehler beim schulbezogenen/
globalen Override weiterhin als `none`. Das kann im Operator-Dialog einen
falschen Zustand zeigen; der schulbezogene Handler begründet dies ausdrücklich
mit der Verfügbarkeit des Dialogs. Der separate Anzeigevertrag wird hier nicht
geändert. Das Login-Gate bleibt davon getrennt; die neue Audit-Sperre umgeht
keine bestehende Zugriffskontrolle.

## Vollständige Operator-Routeninventur, Runde 3

Stand: nach Integration von `origin/development` (`cb62e5a712`) und PR-Head
`1d573145723279b3974b38579074b49a5fed9d21`. Grundlage ist jede einzelne
Operator-Route in `backend/api/testdata/route_table.golden`, einschließlich
Schreibmethoden und Slash-Aliassen. Handler **und** tatsächlich serialisierte
Antwort-DTOs wurden gelesen. Die Tabelle beschreibt den Code, keinen
produktiven Nachweis und keine Prüfung beliebiger missbräuchlicher Freitexte.
„Nein“ bedeutet keine Personendaten von Schulnutzern/Kindern/Eltern in der
regulären Antwort, nicht „der gesamte Vorgang verarbeitet keine Daten“.
Aggregierte Schulzahlen sind keine individuellen Finanz-/Gesundheitsdaten.
Geräte-, Raum-, Rollen-, Schul- und Trägernamen sind Sachbezeichnungen;
Personen gehören nicht in diese Bezeichnungen. Schul-Kontaktfelder werden
vorsorglich geschützt, weil sie auch eine natürliche Person benennen können.
RFID-UID und Bearbeitungsnotiz werden dagegen ausdrücklich als möglicher
Personenbezug behandelt. Nur eigene Teamdaten sind begründet ausgenommen.

Die Spalte „protokolliert“ meint das **Lese-/Freigabe-Audit** dieser Antwort.
Ein vorhandenes best-effort-Änderungs-Audit ersetzt es nicht. Bei geschützten
Schreibantworten kommt `view` vor dem Handler und damit vor der Änderung;
ein Audit-Ausfall führt deshalb nicht erst nach einem erfolgreichen Write
zu einem Fehler. Ein `view` ist auch dort nur ein Versuch, kein Änderungserfolg.

### Fundstellen-Schlüssel

Alle Pfade beginnen mit `backend/`. Funktionsnamen und DTOs stehen pro Zeile.

- **P:** [Provisioning-Handler](../../backend/modules/organizationtenancy/inbound/operator/provisioning.go), [Dashboard-Handler](../../backend/modules/organizationtenancy/inbound/operator/provisioning_summaries.go); DTOs [Provisioning](../../backend/modules/organizationtenancy/provisioning.go), [School](../../backend/modules/organizationtenancy/school.go), [Organization](../../backend/modules/organizationtenancy/organizationtenancy.go).
- **I:** Identity-Operator [Schulzugänge](../../backend/modules/identityaccess/inbound/operator/account_access.go), [MFA-Administration](../../backend/modules/identityaccess/inbound/operator/mfa_admin.go), [Auth](../../backend/modules/identityaccess/inbound/operator/auth.go), [Profiländerung](../../backend/modules/identityaccess/inbound/operator/profile.go).
- **O:** Identity-Operator [Einladungen](../../backend/modules/identityaccess/inbound/operator/invitations.go), [Profil](../../backend/modules/identityaccess/inbound/operator/operator_profile.go), [Teamstatus](../../backend/modules/identityaccess/inbound/operator/operator_status.go).
- **M/K:** Identity-Operator [MFA](../../backend/modules/identityaccess/inbound/operator/mfa.go), [Passkeys](../../backend/modules/identityaccess/inbound/operator/passkeys.go); DTOs aus deren Ports, [TrustedDeviceDTO](../../backend/api/common/trusted_device_dto.go) und öffentlichen Identity-Verträgen.
- **B:** [Billing-Handler und DTOs](../../backend/modules/organizationtenancy/inbound/operator/billing.go).
- **A:** [Ankündigungs-Handler und Wire-DTOs](../../backend/modules/communication/http/operatorannouncements/announcements.go), [AnnouncementStats](../../backend/modules/communication/communication.go).
- **C:** [Care-Plan-Handler](../../backend/modules/careplan/http/booking_authority_impact.go), [BookingAuthorityImpactChild](../../backend/modules/careplan/care_lifecycle.go).
- **S:** [Settings-Handler](../../backend/modules/settings/inbound/operator/settings.go), [Schema-/Reveal-Leser](../../backend/modules/settings/internal/application/composition_setting_transaction_present.go), [Schema-Builder](../../backend/modules/settings/internal/application/schema_builder.go), Definitionen unter `modules/settings/internal/defaults/`.
- **T:** [RFID-Triage-Handler und scanResponse](../../backend/modules/devicefleet/inbound/operator/unregistered_tag_scans.go).
- **D/W:** [CaregiverCapabilityState](../../backend/modules/peopledirectory/caregiver_values.go), [Merge-Handler](../../backend/workflows/staffpersonmerge/http/routes.go) und [Candidate/PersonSnapshot](../../backend/workflows/staffpersonmerge/merge.go).
- **Wiring/Audit:** [Operator-Router](../../backend/api/operator_routes.go), [AuditReadAccess](../../backend/modules/organizationtenancy/inbound/operator/provisioning_read_audit.go), [RecordOperatorRead](../../backend/modules/organizationtenancy/internal/application/provisioning_read_audit.go).

**98 Routen: 34 mit verpflichtendem Lese-Audit, 14 begründete Team-Ausnahmen, 50 ohne Personenobjekt in der Antwort.**

| Route | Personendaten ja/nein | Lesezugriff protokolliert / Ausnahme | Handler → Antwort-DTO / Begründung |
| --- | --- | --- | --- |
| `DELETE /operator/accounts/{accountId}/tenants/{tenantId}` | Ja | Ja: `view=account_tenants` vor Handler | I.UpdateAccountTenantRole/RevokeAccountTenantAccess → AccountTenantAccessEntry[]: gesamte verbleibende Zugangsliste |
| `DELETE /operator/announcements/{id}` | Nein | Kein Lese-Audit erforderlich | A.DeleteAnnouncement → data=null; bestehendes Änderungs-Audit |
| `DELETE /operator/auth/mfa/trusted-devices/{deviceId}` | Nein | Kein Lese-Audit erforderlich | M.RevokeTrustedDevice → 204, nur eigenes Team-Gerät |
| `DELETE /operator/auth/passkeys/{passkeyId}` | Nein | Kein Lese-Audit erforderlich | K.PasskeyRevoke → 204, nur eigener Operator-Passkey |
| `DELETE /operator/devices/{id}` | Nein | Kein Lese-Audit erforderlich | P.DeleteDevice → data=null; verpflichtendes Änderungs-Audit |
| `DELETE /operator/invitations/{id}` | Nein | Kein Lese-Audit erforderlich | O.RevokeInvitation → data=null/204, keine Personenfelder; Team-Einladungsverwaltung |
| `DELETE /operator/organizations/{id}` | Nein | Kein Lese-Audit erforderlich | P.SoftDeleteOrganization → 204 bzw. data=null; kein Personenobjekt zurückgegeben |
| `DELETE /operator/persons/{id}` | Nein | Kein Lese-Audit erforderlich | P.SoftDeletePerson → 204; verpflichtendes Änderungs-Audit |
| `DELETE /operator/schools/{id}` | Nein | Kein Lese-Audit erforderlich | P.SoftDeleteSchool → 204 bzw. data=null; kein Personenobjekt zurückgegeben |
| `DELETE /operator/schools/{id}/accounts/{accountId}/caregiver-capability/` | Ja | Ja: `view=school_account_caregiver` vor Handler | P.Get/Enable/DisableSchoolAccountCaregiverCapability → D.CaregiverCapabilityState: Namen, E-Mail, Bindungen |
| `DELETE /operator/schools/{id}/accounts/{accountId}/mfa/` | Nein | Kein Lese-Audit erforderlich | I.ResetSchoolAccountMFA/SetSchoolAccountMFAOverride → 204; verpflichtendes Änderungs-Audit, keine Personendaten in der Antwort |
| `DELETE /operator/schools/{id}/settings/values/{key}` | Nein | Kein Lese-Audit erforderlich | S.ResetSchoolSettingValue → 204 oder schoolSettingMutationResponse: nur school_slug, keine Werte; config.setting_audit protokolliert Änderungen |
| `GET /operator/accounts` | Ja | Ja: `view=accounts` vor Handler | P.ListAllAccounts → OrganizationAccount: Namen, E-Mail, Rollen, Schule |
| `GET /operator/accounts/{accountId}/mfa/global-override` | Ja | Ja: `view=account_mfa` vor Handler | I.GetAccountMFAGlobalOverride → MFAGlobalOverrideStateResponse: MFA-Status des Kontos |
| `GET /operator/accounts/{accountId}/tenants` | Ja | Ja: `view=account_tenants` vor Handler | I.List/GrantAccountTenantAccess → AccountTenantAccessEntry[]: kontobezogene Schulzugänge, Rollen, Status und Zeitpunkte |
| `GET /operator/accounts/{accountId}/tenants/` | Ja | Ja: `view=account_tenants` vor Handler | I.List/GrantAccountTenantAccess → AccountTenantAccessEntry[]: kontobezogene Schulzugänge, Rollen, Status und Zeitpunkte |
| `GET /operator/accounts/{accountId}/tenants/{tenantId}/roles` | Nein | Kein Lese-Audit erforderlich | I.ListAssignableSchoolRoles → RoleOption[]: verfügbare Schulrollen, keine Rollenbelegung des Kontos; Account-Lookup nur Guard |
| `GET /operator/announcements/` | Ja | Ja: `view=announcements` vor Handler | A.ListAnnouncements → AnnouncementResponse: freier Titel/Inhalt kann Personen benennen; Zielrollen/-schulen/-träger und interne created_by-ID, keine Empfängerliste |
| `GET /operator/announcements/{id}` | Ja | Ja: `view=announcement` vor Handler | A.GetAnnouncement → AnnouncementResponse: freier Titel/Inhalt kann Personen benennen; Zielrollen/-schulen/-träger und interne created_by-ID, keine Empfängerliste |
| `GET /operator/announcements/{id}/stats` | Nein | Kein Lese-Audit erforderlich | A.GetStats → AnnouncementStats: nur announcement_id und drei Summen |
| `GET /operator/announcements/{id}/views` | Ja | Ja: `view=announcement_views` vor Handler | A.GetViewDetails → AnnouncementViewDetailResponse[]: user_id, user_name, seen_at, dismissed; AccountEmail des internen DTO wird nicht ausgegeben |
| `GET /operator/auth/mfa/trusted-devices` | Ja, nur moto-Team | Begründet ausgenommen: nur eigene Operatoren, keine Schul-/Elternkonten | M.ListTrustedDevices → common.TrustedDeviceDTO[]: eigene IP, User-Agent, Zeitpunkte; nur Team-Selbstverwaltung |
| `GET /operator/auth/passkeys` | Ja, nur moto-Team | Begründet ausgenommen: nur eigene Operatoren, keine Schul-/Elternkonten | K.PasskeyList → PasskeyCredentialSummary[]: Name und Zeitpunkte eigener Operator-Passkeys, keine Schul-/Elternkonten |
| `GET /operator/auth/passkeys/` | Ja, nur moto-Team | Begründet ausgenommen: nur eigene Operatoren, keine Schul-/Elternkonten | K.PasskeyList → PasskeyCredentialSummary[]: Name und Zeitpunkte eigener Operator-Passkeys, keine Schul-/Elternkonten |
| `GET /operator/billing/key-date-counts` | Nein | Kein Lese-Audit erforderlich | B.ListKeyDateCounts → billingKeyDateCountResponse[]: Stichtag, Schul-/Trägernamen, Kinder-/Terminalzahlen, keine Namen, Bankdaten oder individuellen Zahlungen |
| `GET /operator/billing/key-date-counts/export` | Nein | Kein Lese-Audit erforderlich | B.ExportKeyDateCounts → CSV mit denselben Schul-Summen: Stichtag, Schul-/Trägernamen, Kinder-/Terminalzahlen, keine Namen, Bankdaten oder individuellen Zahlungen |
| `GET /operator/billing/key-day` | Nein | Kein Lese-Audit erforderlich | B.GetKeyDay → billingKeyDayResponse: Stichtag, Schul-/Trägernamen, Kinder-/Terminalzahlen, keine Namen, Bankdaten oder individuellen Zahlungen |
| `GET /operator/devices/` | Nein | Kein Lese-Audit erforderlich | P.ListAllDevices → OperatorDevice[]: Geräte-/Schulstamm, keine Schulnutzer; bestehendes Änderungs-Audit bei Writes, Rotation/Transfer verpflichtend |
| `GET /operator/devices/{id}/transfer-status` | Nein | Kein Lese-Audit erforderlich | P.GetDeviceTransferStatus → DeviceTransferStatus: Sitzung/Angebot/Raum, keine Teilnehmenden: Geräte-/Schulstamm, keine Schulnutzer; bestehendes Änderungs-Audit bei Writes, Rotation/Transfer verpflichtend |
| `GET /operator/invitations/` | Ja, nur moto-Team | Begründet ausgenommen: nur eigene Operatoren, keine Schul-/Elternkonten | O.ListInvitations → InvitationsListResponse: Einladungs-E-Mails/-Namen plus bestehende Operatoren, creator_name und last_login; keine Schul-Einladungen |
| `GET /operator/organizations/` | Nein | Kein Lese-Audit erforderlich | P.ListOrganizations → Organization[]: Trägerstammdaten/Branding, ggf. Summen, keine natürlichen Personen oder Kontaktfelder |
| `GET /operator/organizations/summaries` | Nein | Kein Lese-Audit erforderlich | P.ListOrganizationSummaries → OrganizationSummary[]: Trägerstammdaten/Branding, ggf. Summen, keine natürlichen Personen oder Kontaktfelder |
| `GET /operator/organizations/{id}/accounts` | Ja | Ja: `view=organization_accounts` vor Handler | P.ListSchoolAccounts/ListOrganizationAccounts → SchoolAccount/OrganizationAccount: Namen, E-Mail |
| `GET /operator/organizations/{id}/devices` | Nein | Kein Lese-Audit erforderlich | P.ListOrganizationDevices → OperatorDevice[]: Geräte-/Schulstamm, keine Schulnutzer; bestehendes Änderungs-Audit bei Writes, Rotation/Transfer verpflichtend |
| `GET /operator/organizations/{id}/persons` | Ja | Ja: `view=organization_persons` vor Handler | P.ListSchoolPersons/ListOrganizationPersons → OperatorPerson: Namen, Konto-E-Mail, RFID-Zuordnung |
| `GET /operator/organizations/{id}/schools` | Ja | Ja: `view=organization_schools` vor Handler | P.ListOrganizationSchoolSummaries → SchoolSummary[]: dieselben Kontaktfelder |
| `GET /operator/profile/` | Ja, nur moto-Team | Begründet ausgenommen: nur eigene Operatoren, keine Schul-/Elternkonten | O.GetProfile → OperatorResponse: eigene Team-E-Mail und eigener Anzeigename |
| `GET /operator/roles` | Nein | Kein Lese-Audit erforderlich | P.ListSystemRoles → SystemRole[]: Rollenstamm, keine Belegung |
| `GET /operator/schools/` | Ja | Ja: `view=schools` vor Handler | P.ListSchools → School[]: Adresse, Telefon, E-Mail neben Schulstammdaten |
| `GET /operator/schools/summaries` | Ja | Ja: `view=school_summaries` vor Handler | P.ListSchoolSummaries → SchoolSummary[]: dieselben Kontaktfelder und aggregierte Zahlen |
| `GET /operator/schools/{id}/accounts` | Ja | Ja: `view=school_accounts` vor Handler | P.ListSchoolAccounts/ListOrganizationAccounts → SchoolAccount/OrganizationAccount: Namen, E-Mail |
| `GET /operator/schools/{id}/accounts/{accountId}/caregiver-capability/` | Ja | Ja: `view=school_account_caregiver` vor Handler | P.Get/Enable/DisableSchoolAccountCaregiverCapability → D.CaregiverCapabilityState: Namen, E-Mail, Bindungen |
| `GET /operator/schools/{id}/accounts/{accountId}/mfa/` | Ja | Ja: `view=school_account_mfa` vor Handler | I.GetSchoolAccountMFAState → MFAAdminStateResponse: MFA-Status des Kontos |
| `GET /operator/schools/{id}/devices` | Nein | Kein Lese-Audit erforderlich | P.ListSchoolDevices → OperatorDevice[]: Geräte-/Schulstamm, keine Schulnutzer; bestehendes Änderungs-Audit bei Writes, Rotation/Transfer verpflichtend |
| `GET /operator/schools/{id}/persons` | Ja | Ja: `view=school_persons` vor Handler | P.ListSchoolPersons/ListOrganizationPersons → OperatorPerson: Namen, Konto-E-Mail, RFID-Zuordnung |
| `GET /operator/schools/{id}/pwa-usage` | Nein | Kein Lese-Audit erforderlich | P.GetSchoolPWAUsage → SchoolPWAUsage: aggregierte Staff-/Parent-Zahlen, keine Konten |
| `GET /operator/schools/{id}/settings/booking-authority-impact` | Ja | Ja: `view=school_booking_authority_impact` vor Handler | C.ServeHTTP → BookingAuthorityImpactChild[]: student_id, first_name, last_name, school_class, erster Tag ohne Buchung |
| `GET /operator/schools/{id}/settings/schema` | Ja | Ja: `view=school_settings_schema` vor Handler | S.GetSchoolSettingsSchema → SettingsSchema: aufgelöste Werte/Freitexte können Personen-/Kontaktdaten enthalten; Password-Werte maskiert, admin_only ausgeblendet |
| `GET /operator/schools/{id}/settings/values/{key}/reveal` | Ja | Ja: `view=school_setting_reveal` vor Handler | S.RevealSchoolSettingValue → {value}: unmaskierter Wert; nicht auf Password-Typ begrenzt; admin_only bleibt verboten; bisher kein Lese-Audit, jetzt vorgeschaltet |
| `GET /operator/staff-person-merges/{schoolId}` | Ja | Ja: `view=staff_person_merge_candidates` vor Handler | W.preview → Candidate[]/PersonSnapshot: Namen, Geburtstag, RFID-ID, Konto-/Beschäftigungsbezug; Audit im Workflow |
| `GET /operator/stats` | Nein | Kein Lese-Audit erforderlich | P.GetProvisioningStats → ProvisioningStats: ausschließlich vier Summen |
| `GET /operator/unregistered-tag-scans/` | Ja | Ja: `view=unregistered_tag_scans` vor Handler | T.ListUnregisteredTagScans → scanResponse[] bzw. scanResponse: tag_uid, Scan-/Bearbeitungszeit, resolution_note (Freitext), Geräte-/Schulbezug, resolved_by_operator_id; keine Namen-Join-Abfrage, aber Personenbezug möglich |
| `POST /operator/accounts/{accountId}/tenants` | Ja | Ja: `view=account_tenants` vor Handler | I.List/GrantAccountTenantAccess → AccountTenantAccessEntry[]: kontobezogene Schulzugänge, Rollen, Status und Zeitpunkte |
| `POST /operator/accounts/{accountId}/tenants/` | Ja | Ja: `view=account_tenants` vor Handler | I.List/GrantAccountTenantAccess → AccountTenantAccessEntry[]: kontobezogene Schulzugänge, Rollen, Status und Zeitpunkte |
| `POST /operator/announcements/` | Ja | Ja: `view=announcements` vor Handler | A.CreateAnnouncement → AnnouncementResponse: freier Titel/Inhalt kann Personen benennen; Zielrollen/-schulen/-träger und interne created_by-ID, keine Empfängerliste |
| `POST /operator/announcements/{id}/publish` | Nein | Kein Lese-Audit erforderlich | A.PublishAnnouncement → data=null; bestehendes Änderungs-Audit |
| `POST /operator/auth/email-confirm` | Nein | Kein Lese-Audit erforderlich | O.ConfirmEmailChange → data=null; Rückgabe des Operators wird ausdrücklich verworfen |
| `POST /operator/auth/invitations/accept` | Ja, nur moto-Team | Begründet ausgenommen: nur eigene Operatoren, keine Schul-/Elternkonten | O.AcceptInvitation → OperatorResponse: E-Mail/Anzeigename des neu angelegten Teammitglieds |
| `POST /operator/auth/invitations/validate` | Ja, nur moto-Team | Begründet ausgenommen: nur eigene Operatoren, keine Schul-/Elternkonten | O.ValidateInvitation → InvitationValidationResponse: E-Mail/Anzeigename nur der internen Operator-Einladung |
| `POST /operator/auth/login` | Ja, nur moto-Team | Begründet ausgenommen: nur eigene Operatoren, keine Schul-/Elternkonten | I.Login → LoginResponse/OperatorResponse: eigenes Teamprofil bzw. maskierte eigene E-Mail und Tokens |
| `POST /operator/auth/mfa/enroll/confirm` | Nein | Kein Lese-Audit erforderlich | M.EnrollConfirm → MFATokenResponse: nur Tokens/Challenge der eigenen Team-Anmeldung, keine Schul-/Elternkonten |
| `POST /operator/auth/mfa/enroll/start` | Nein | Kein Lese-Audit erforderlich | M.EnrollStart → 204, Code nur an Operator-E-Mail |
| `POST /operator/auth/mfa/resend` | Nein | Kein Lese-Audit erforderlich | M.Resend → MFAResendResponse: nur Tokens/Challenge der eigenen Team-Anmeldung, keine Schul-/Elternkonten |
| `POST /operator/auth/mfa/verify` | Nein | Kein Lese-Audit erforderlich | M.Verify → MFATokenResponse: nur Tokens/Challenge der eigenen Team-Anmeldung, keine Schul-/Elternkonten |
| `POST /operator/auth/passkeys/enrollment/challenge` | Ja, nur moto-Team | Begründet ausgenommen: nur eigene Operatoren, keine Schul-/Elternkonten | K.PasskeyEnrollmentChallenge → PasskeyEnrollmentChallenge: ChallengeToken und maskierte eigene Operator-E-Mail, nur Team-Anmeldung |
| `POST /operator/auth/passkeys/login/options` | Nein | Kein Lese-Audit erforderlich | K.PasskeyLoginOptions → WebAuthn CredentialAssertion: Challenge, keine Schul-/Elternkonten |
| `POST /operator/auth/passkeys/login/verify` | Nein | Kein Lese-Audit erforderlich | K.PasskeyLoginVerify → LoginResponse ohne Operator-Profil: nur Tokens/Challenge der eigenen Team-Anmeldung, keine Schul-/Elternkonten |
| `POST /operator/auth/passkeys/register/options` | Ja, nur moto-Team | Begründet ausgenommen: nur eigene Operatoren, keine Schul-/Elternkonten | K.PasskeyRegisterOptions → WebAuthn CredentialCreation: eigene Operator-UserEntity, Challenge, keine Schul-/Elternkonten |
| `POST /operator/auth/passkeys/register/verify` | Ja, nur moto-Team | Begründet ausgenommen: nur eigene Operatoren, keine Schul-/Elternkonten | K.PasskeyRegisterVerify → PasskeyCredentialSummary: eigener Passkey-Name/Zeitpunkte, nur Team-Selbstverwaltung |
| `POST /operator/auth/refresh` | Nein | Kein Lese-Audit erforderlich | I.RefreshToken → RefreshTokenResponse: nur Tokens/Challenge der eigenen Team-Anmeldung, keine Schul-/Elternkonten |
| `POST /operator/billing/key-date-counts/seed` | Nein | Kein Lese-Audit erforderlich | B.SeedKeyDateCounts → {written}, nur lokaler Demo-Seeder: Stichtag, Schul-/Trägernamen, Kinder-/Terminalzahlen, keine Namen, Bankdaten oder individuellen Zahlungen |
| `POST /operator/devices/` | Nein | Kein Lese-Audit erforderlich | P.CreateDevice → OperatorDevice mit einmaligem api_key: Geräte-/Schulstamm, keine Schulnutzer; bestehendes Änderungs-Audit bei Writes, Rotation/Transfer verpflichtend |
| `POST /operator/devices/{id}/set-api-key` | Nein | Kein Lese-Audit erforderlich | P.SetDeviceAPIKey → OperatorDevice mit einmaligem api_key: Geräte-/Schulstamm, keine Schulnutzer; bestehendes Änderungs-Audit bei Writes, Rotation/Transfer verpflichtend |
| `POST /operator/devices/{id}/transfer` | Nein | Kein Lese-Audit erforderlich | P.TransferDevice → OperatorDevice ohne api_key: Geräte-/Schulstamm, keine Schulnutzer; bestehendes Änderungs-Audit bei Writes, Rotation/Transfer verpflichtend |
| `POST /operator/invitations/` | Nein | Kein Lese-Audit erforderlich | O.CreateInvitation → data=null/204, keine Personenfelder; Team-Einladungsverwaltung |
| `POST /operator/invitations/{id}/resend` | Nein | Kein Lese-Audit erforderlich | O.ResendInvitation → data=null/204, keine Personenfelder; Team-Einladungsverwaltung |
| `POST /operator/operators/{id}/deactivate` | Ja, nur moto-Team | Begründet ausgenommen: nur eigene Operatoren, keine Schul-/Elternkonten | O.DeactivateOperator/changeOperatorStatus → OperatorListItem: E-Mail, Anzeigename, Aktiv-/Löschstatus, nur moto-Team |
| `POST /operator/operators/{id}/reactivate` | Ja, nur moto-Team | Begründet ausgenommen: nur eigene Operatoren, keine Schul-/Elternkonten | O.ReactivateOperator/changeOperatorStatus → OperatorListItem: E-Mail, Anzeigename, Aktiv-/Löschstatus, nur moto-Team |
| `POST /operator/organizations/` | Nein | Kein Lese-Audit erforderlich | P.CreateOrganization → Organization: Trägerstammdaten/Branding, ggf. Summen, keine natürlichen Personen oder Kontaktfelder |
| `POST /operator/organizations/{id}/restore` | Nein | Kein Lese-Audit erforderlich | P.RestoreOrganization → 204 bzw. data=null; kein Personenobjekt zurückgegeben |
| `POST /operator/profile/email-change` | Nein | Kein Lese-Audit erforderlich | O.InitiateEmailChange → data=null; eigene Team-Adresse nicht in Antwort |
| `POST /operator/profile/password` | Nein | Kein Lese-Audit erforderlich | I.ChangePassword → data=null, nur eigene Team-Anmeldedaten |
| `POST /operator/schools/` | Ja | Ja: `view=schools` vor Handler | P.CreateSchool → School: Kontaktfelder; Ziel-ID existiert beim Zugriffsversuch noch nicht |
| `POST /operator/schools/{id}/accounts/{accountId}/caregiver-capability/` | Ja | Ja: `view=school_account_caregiver` vor Handler | P.Get/Enable/DisableSchoolAccountCaregiverCapability → D.CaregiverCapabilityState: Namen, E-Mail, Bindungen |
| `POST /operator/schools/{id}/create-account` | Ja | Ja: `view=school_account_creation` vor Handler | P.CreateSchoolAccount → CreatedAccount: E-Mail, Username, Avatar, Kontostatus |
| `POST /operator/schools/{id}/invite-admin` | Ja | Ja: `view=school_admin_invitation` vor Handler | P.InviteSchoolAdmin → operatorInvitationResponse: Namen, E-Mail, Einladungs-/Versanddaten, Creator-E-Mail |
| `POST /operator/schools/{id}/restore` | Nein | Kein Lese-Audit erforderlich | P.RestoreSchool → 204 bzw. data=null; kein Personenobjekt zurückgegeben |
| `POST /operator/staff-person-merges/{schoolId}` | Nein | Kein Lese-Audit erforderlich | W.execute → data=null, nur Ergebnistext; bestehendes Änderungs-Audit |
| `POST /operator/unregistered-tag-scans/{id}/resolve` | Ja | Ja: `view=unregistered_tag_scans` vor Handler | T.ResolveUnregisteredTagScan → scanResponse[] bzw. scanResponse: tag_uid, Scan-/Bearbeitungszeit, resolution_note (Freitext), Geräte-/Schulbezug, resolved_by_operator_id; keine Namen-Join-Abfrage, aber Personenbezug möglich |
| `PUT /operator/accounts/{accountId}/mfa/global-override` | Nein | Kein Lese-Audit erforderlich | I.SetAccountMFAGlobalOverride → 204; verpflichtendes Änderungs-Audit |
| `PUT /operator/accounts/{accountId}/tenants/{tenantId}` | Ja | Ja: `view=account_tenants` vor Handler | I.UpdateAccountTenantRole/RevokeAccountTenantAccess → AccountTenantAccessEntry[]: gesamte verbleibende Zugangsliste |
| `PUT /operator/announcements/{id}` | Ja | Ja: `view=announcement` vor Handler | A.UpdateAnnouncement → AnnouncementResponse: freier Titel/Inhalt kann Personen benennen; Zielrollen/-schulen/-träger und interne created_by-ID, keine Empfängerliste |
| `PUT /operator/billing/key-day` | Nein | Kein Lese-Audit erforderlich | B.UpdateKeyDay → billingKeyDayResponse: Stichtag, Schul-/Trägernamen, Kinder-/Terminalzahlen, keine Namen, Bankdaten oder individuellen Zahlungen |
| `PUT /operator/organizations/{id}` | Nein | Kein Lese-Audit erforderlich | P.UpdateOrganization → Organization: Trägerstammdaten/Branding, ggf. Summen, keine natürlichen Personen oder Kontaktfelder |
| `PUT /operator/profile/` | Ja, nur moto-Team | Begründet ausgenommen: nur eigene Operatoren, keine Schul-/Elternkonten | I.UpdateProfile → OperatorResponse: eigene Team-E-Mail und eigener Anzeigename |
| `PUT /operator/schools/{id}` | Ja | Ja: `view=school` vor Handler | P.UpdateSchool → School: Kontaktfelder, auch bestehende Daten |
| `PUT /operator/schools/{id}/accounts/{accountId}/mfa/override` | Nein | Kein Lese-Audit erforderlich | I.ResetSchoolAccountMFA/SetSchoolAccountMFAOverride → 204; verpflichtendes Änderungs-Audit, keine Personendaten in der Antwort |
| `PUT /operator/schools/{id}/settings/values/{key}` | Nein | Kein Lese-Audit erforderlich | S.SetSchoolSettingValue → 204 oder schoolSettingMutationResponse: nur school_slug, keine Werte; config.setting_audit protokolliert Änderungen |
