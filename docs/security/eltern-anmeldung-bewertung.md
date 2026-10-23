# Elternportal: Anmeldung, Datenzugriff und Wiederherstellung

Stand: 23.10.2026. Slice 3841-d, AK6 von #3841.
Codebasis: `c66191b29879360e1eb6085ce46d716080a3b3be`.
Die Codebelege unten sind auf diesen Commit festgelegt. Statische Prüfung,
keine ausgeführten Verhaltenstests, keine Prüfung produktiver Systeme.

## Ergebnis und Reichweite

**Bewertung:** Einen zusätzlichen Eltern-MFA-Kanal nicht pauschal als
Abnahmekriterium verlangen. Das entspricht der internen Einordnung vom
23.10.2026, die risikogerechten Zugangsschutz fordert, den zusätzlichen
Eltern-MFA-Kanal aber nicht als allgemeine Voraussetzung einstuft. Das ist
keine rechtliche Freigabe des konkreten Betriebs. Weitergehende Kundenpflichten
und der vollständige Legal-/AVV-/TOM-Abgleich bleiben beim Hauptdokument. [L1]

**Bewertung:** Der Verzicht ist nicht mit „Eltern sehen nur harmlose Daten“ zu
begründen. Der geprüfte Elternpfad verwendet Passwort-Login ohne belegten
Passkey-/MFA-Schritt; das Portal liefert unter anderem Gesundheitsangaben,
Betreuungszeiten, Nachrichten und Dateien. Je Kind bestehen außerdem
sicherheitsrelevante Schreib- und Einladungsrechte. [C1] [C2] [C3] [C4] [C5]
[C6] [C7] [C8]

**Offene Schlussfolgerung:** Eine abschließende Angemessenheitsbewertung ohne
zusätzliche Eltern-MFA ist erst möglich, wenn insbesondere der Schutz gegen
verteilte Loginversuche, der Mail-/Recovery-Weg, die tatsächlichen Rechte und
die Sitzungsrestlaufzeiten belegt sind. Das vorhandene IP-Limit ist keine
kontobezogene Login-Sperre; 3841-c bleibt dafür ein offener Folge-Slice, nicht
eine hier bereits umgesetzte Maßnahme. [C1] [C9] [C10] [C11] [C12]

## 1. Im Code: Daten und Berechtigungsstufen

Die schulübergreifende Kindauflösung verlangt aktive Schulzuordnungen und
filtert die Beziehungen auf `parent_portal.access`; danach prüft
`ResolvePermittedChild` das für die jeweilige Aktion gespeicherte Recht.
Schulmitgliedschaft, Kontaktstatus oder Abholberechtigung allein reichen damit
nicht. Kindbezogene Datenzugriffe öffnen anschließend die Transaktion für die
aufgelöste Schule. Das ist ein Codebefund, kein vollständiger Nachweis aller
RLS-Policies oder ihrer produktiven Wirksamkeit. [C13] [C14] [C3]

Die Presets `primary_guardian`, `legal_guardian` und `co_guardian` erhalten das
volle Eltern-Rechteset. `pickup_only`, `emergency_contact`, `social_worker`
und `custom` erhalten durch die Preset-Funktion keine solchen Rechte.
Maßgeblich bleiben die tatsächlich gespeicherten Rechte je Kind, nicht allein
der Rollenname. Ein Konto kann deshalb für verschiedene Kinder unterschiedliche
Befugnisse haben. [C7] [C13] [C14]

| Daten/Handlung | Im Code sichtbar beziehungsweise möglich | Grenze und Bewertung |
|---|---|---|
| Identität und Betreuung | Stammdaten enthalten Name, Geburtstag, Klasse, Status, Betreuungszeitraum und dauerhafte Gehzeit-/Abholkonfiguration. Die Tagesansicht liefert Anwesenheitsstatus und reduzierte Ankunfts-/Abgangs-/Abholzeiten, keine Raum- oder Bewegungshistorie in diesem Payload. [C3] [C4] | **Bewertung:** Auch diese reduzierte Sicht verrät Tagesabläufe eines Kindes. Keine Aussage über die Erforderlichkeit einzelner Daten im konkreten Schulbetrieb. |
| Gesundheit | `GetChildMasterData` verlangt nur `ResolveOwnedChild`, also Portalzugriff; `loadMasterData` und `toMasterDataResponse` reichen `health_info` weiter. Für das Lesen gibt es in diesem Pfad keine zusätzliche Gesundheits-Berechtigungsstufe. Änderungen benötigen `parent_portal.master_data.edit`, laufende Betreuung und den Schulschalter; sie werden transaktional mit einer Änderungszeile erfasst. [C3] [C14] [C15] [C42] | **Bewertung:** Schon ein lesender Portalzugang kann Gesundheitsangaben offenlegen. Eine reine Schreibsperre oder ein deaktivierter Bearbeitungsschalter beseitigt dieses Leserisiko nicht. |
| Kontakt- und Abholinformationen | Eigene E-Mail, Anschrift und primäre Telefonnummer stehen in den Stammdaten. Die Kontaktliste anderer Bezugspersonen zeigt Namen, Beziehung, Abhol-/Notfallstatus und Abholnotizen; E-Mail/Telefon/Adresse werden dort für andere Kontoinhaber, Sozialarbeit und familienübergreifend geteilte Kontakte ausgeblendet. Nicht geschützte Kontakte können diese Daten liefern. Die separate Ansicht „verbundene Konten“ liefert dagegen auch E-Mail-Adressen: keine pauschale Kontaktredaktion über alle Endpunkte behaupten. [C3] [C16] [C17] [C18] | Kontaktänderung und Abholverwaltung haben getrennte Rechte (`guardian.edit`, `pickup.manage`) und zusätzliche Zielschutzregeln. **Bewertung:** Kontoübernahme kann neben Vertraulichkeit auch Abholabläufe und Erreichbarkeit betreffen. [C16] [C17] |
| Kommunikation und Dateien | Die Unterhaltung wird für das Paar Kind/Elternkonto geladen; Schreiben braucht zusätzlich `parent_portal.notes.write`. Bereits vorhandene Historie bleibt bei abgeschalteter Nachrichtenfunktion lesbar. Ankündigungsanhänge verlangen aktuelle Zielgruppenberechtigung, lebende Ankündigung und freigeschaltete Funktion; Dateizugriffe laufen in der ermittelten Schule mit erneuter Zugriffsprüfung. Für Anhänge im eigenen Erklärungsnachweis besteht ein gesonderter Pfad, auch nach Ablauf/Rücknahme der Ankündigung. [C5] [C6] [C19] | **Bewertung:** Freitext und Anhänge können weitere sensible Angaben enthalten; der konkrete Inhalt wurde nicht erhoben. Nicht als Zugriff auf sämtliche Schüler-/Personalakten oder auf alle Chats anderer Eltern missverstehen. |
| Weitere Befugnisse | Routen existieren für Krankmeldungen, Änderungsanfragen zu Betreuung/Stammdaten, Essensteilnahme, Einwilligungen/Erklärungen und Anmeldungen. Das volle Preset trennt die entsprechenden Aktionsrechte. Einladungen/Entzug weiterer Konten verlangen `related_accounts.manage` und Schulregeln. [C7] [C8] [C20] | **Bewertung:** Eine Elternsitzung ist nicht generell nur lesend. Eine Übernahme kann falsche Erklärungen, Nachrichten oder Zugangserweiterungen ermöglichen, soweit die konkreten Rechte und Schulregeln das zulassen. |

Die Bewertung der Personalzugriffe und der dortigen Gesundheitsrechte gehört
in die Schwesterdatei
[Zugriff auf Gesundheits- und Personaldaten](zugriff-gesundheits-und-personaldaten.md).

## 2. Im Code: Anmeldung, Einladungen und Recovery

### Regulärer Login und fehlender zweiter Faktor

`LoginRequest.Bind` normalisiert die E-Mail. `validateLoginCredentials`
prüft Passwort und aktives Konto; `issueParentTokens` verlangt eine Guardian-Rolle
an mindestens einer aktiven Schulzuordnung und erstellt direkt ein Tokenpaar
mit `scope=parent`. Der Frontend-Provider `parent-credentials` folgt diesem
Passwortpfad. In diesen geprüften regulären Elternpfaden ist kein Passkey-,
WebAuthn-, MFA-Challenge- oder MFA-Recovery-Code-Schritt belegt. Eine MFA im
Personalportal ist deshalb kein Nachweis einer MFA für diesen Elternpfad. [C1]
[C2] [C21] [C38]

Passwortprüfung/-hashing verwendet die gemeinsamen Security-Runtime-Helfer
für Argon2id. Die Reset- und Einladungsannahme prüfen zusätzlich die
Passwortstärke. Das schützt nicht gegen die Verwendung eines bereits bekannten,
korrekten Passworts. Letzteres ist eine Risikobewertung, kein hier ausgeführter
Angriffsversuch. [C22] [C11] [C23]

### Passwort-Reset per E-Mail

1. `InitiatePasswordReset` normalisiert die Adresse und prüft für den
   Eltern-Reset die Guardian-Eignung. Unbekannte/nicht geeignete Adressen
   bekommen keinen Link; der HTTP-Erfolg bleibt neutral. Die Mail wird über
   den Eltern-Reset-Scope adressiert. [C11] [C24] [T2]
2. Bei aktiviertem Reset-Limit sind drei versandfähige Anfragen je normalisierter
   Adresse im einstündigen Datenbankfenster möglich; weitere werden mit `429`
   und `Retry-After` abgewiesen. Unbekannte/nicht geeignete Adressen werden
   nicht mitgezählt. Das ist ein Reset-Limit, kein Zähler falscher
   Loginpasswörter. Die Aktivierung ist konfigurationsabhängig. [C11] [C25]
   [C26] [C27] [C39]
3. Die Erstellung widerruft vorherige offene Reset-Links des Kontos und erzeugt
   einen UUID-v4-Link mit konfigurierter Gültigkeit. Die Einlösung verlangt
   einen unbenutzten, nicht abgelaufenen Link und ein ausreichend starkes
   neues Passwort; Passwortänderung, Verbrauch des Links und Widerruf der
   gespeicherten Kontositzungen liegen in derselben Transaktion. Die
   Bestätigung ist kontoweit mit dem Personal-Reset geteilt, nicht ein
   unabhängiges Elternpasswort. [C11] [C28] [C24] [C41]

**Bewertung:** Ein übernommenes Mailkonto kann damit den Passwortwechsel
ermöglichen. Ein zusätzlicher Code an dasselbe Mailkonto wäre für genau dieses
Szenario kein unabhängiger Schutz. Die neutrale Erfolgsantwort ist außerdem kein
vollständiger Enumerationsnachweis: Nur geeignete Konten erreichen das
adressbezogene Limit und damit dessen beobachtbares `429`. Timing und
Beobachtbarkeit über wechselnde IPs bleiben ungeprüft. [C11] [C26]

### Einladungen und Berechtigungserweiterung

Schulseitig erzeugte Einladungen verwenden UUID-v4-Token mit Ablaufzeit und
Mailversand. Annahme, Kontoverknüpfung, Schulzugang und Tokenverbrauch erfolgen
transaktional. Abgelaufene, verwendete, noch nicht genehmigte oder abgelehnte
Einladungen sind nicht einlösbar. Ein vorhandenes Konto derselben Adresse wird
wiederverwendet, ohne dessen Passwort zu überschreiben. [C23] [C29]

Weitere Elternkonten können nur mit `related_accounts.manage` eingeladen werden;
`guardians.parent_invite_mode` kann dies sperren oder eine Personalfreigabe
verlangen. Der Entzug ist separat schulkonfiguriert, und der primäre Guardian ist
vor Eltern-Entzug geschützt. **Bewertung:** Die Richtigkeit von Mailadresse,
Sorge-/Vertretungsbefugnis und Freigabe ist Teil des Schutzes, nicht allein die
Gültigkeit des Einladungslinks. Konkrete produktive Prüfverfahren sind offen.
[C20] [C23]

### Sitzungen, Scope und Refresh-Recovery

Der Eltern-Access-Token ist nicht an eine einzelne Schule gebunden.
`ParentMiddleware` akzeptiert nur `scope=parent`; die Kinder-/Aktionsprüfung
ermittelt anschließend die erlaubte Schule und Beziehung. Das Elterncookie ist
host-only, `HttpOnly`, `SameSite=Strict` und bei `NODE_ENV=production` als
`Secure` konfiguriert. Diese Maßnahmen begrenzen andere Portalsitzungen und
Browserangriffe; sie ersetzen weder das Passwort noch die Beziehungskontrolle.
[C1] [C14] [C21] [C30]

Refresh rotiert die gespeicherte Sitzung, prüft das aktive Konto und erhält
den Eltern-Scope. Die Wiederaufnahme nach verlorener Refresh-Antwort verlangt
innerhalb der konfigurierten Schonfrist einen passenden Recovery-Proof und eine
konsistente Nachfolgerkette; erkannter Replay widerruft die Familie.
Diese technische Refresh-Recovery ist kein zweiter Anmeldefaktor und keine
unabhängige Wiederherstellung eines verlorenen Mailkontos. Logout widerruft die
vorgelegte Refresh-Familie, nicht automatisch alle anderen Gerätefamilien.
[C12] [C31]

**Grenze:** Der Reset widerruft gespeicherte Sitzungen. Daraus folgt nicht die
sofortige Sperrung bereits ausgegebener Access-JWTs: Die geprüfte reguläre
Eltern-Middlewarekette validiert Token/Claims/Scope und bildet den Principal,
ohne dort die Refresh-Familie in der Datenbank nachzuschlagen. Das gesonderte
Frontend-Hand-off `ValidateSessionTokens` prüft dagegen bei mitgegebenem
Refresh-Token dessen gespeicherte Sitzung. Effektive Restzugriffe nach Reset,
Logout oder Kontodeaktivierung sind deshalb ausdrücklich zu prüfen; keine
sofortige globale Abmeldung zusagen. JWT-, Refresh- und Reset-Laufzeiten werden
aus Konfiguration übernommen, hier wurden keine produktiven Werte festgestellt.
[C11] [C32] [C33] [C34] [C35] [C40] [C41]

## 3. Im Code: vorhandene Bremsen und offene Login-Sperre

`mountPortalRoutes` bindet den gemeinsamen Auth-Limiter an das Elternportal;
`RouterWithAuthRateLimiter` umfasst Login, Reset-Anforderung und
Reset-Bestätigung. Bei `RATE_LIMIT_ENABLED=true` gilt standardmäßig eine
Token-Bucket-Rate von 5 Anfragen/Minute mit Burst 10 je IP; die Rate ist
konfigurierbar. Ohne Aktivierung werden die Routen ohne diesen Limiter montiert.
Die Zähler liegen im Prozessspeicher, nicht in einem gemeinsamen
replikaübergreifenden Speicher. [C9] [C10] [C36] [C37]

**Codebefund:** Im geprüften Loginpfad folgen auf falsche Zugangsdaten Audit
und Ablehnung, aber keine kontobezogene Fehlversuchszählung, Verzögerung oder
Sperre. Der konkret verdrahtete Auth-Limiter erhält keinen Konto-Schlüssel;
seine Standardidentität ist die Client-IP. [C1] [C9] [C10] [C38]

**Bewertung/offen, Slice 3841-c:** Wechselnde IPs und mehrere Backendprozesse
können die Wirkung eines einzelnen IP-Budgets begrenzen. Passwort-Reuse mit
einem korrekten Passwort braucht zudem nicht viele Versuche. Kontobezogenen
Schutz gegen verteilte Fehlversuche zusätzlich umsetzen und nachweisen, ohne
eine leicht missbrauchbare dauerhafte Fremdsperre zu schaffen. Die vorhandenen
Reset-Zähler schließen diese Loginlücke nicht. [C1] [C9] [C10] [C11] [C38]

## 4. Vorhandene Tests, hier nicht ausgeführt

Die folgenden Tests sind als Codebelege vorhanden. Ihre Existenz ist kein
grünes Testergebnis und kein Betriebsnachweis.

| Bereich | Konkreter Test und Aussagegrenze |
|---|---|
| Eltern-HTTP/Auth | `TestParentLogin_Success_Returns200WithTokens`, `TestParentLogin_NotAGuardian_Returns403WithCode`, `TestParentPasswordReset_RateLimitedSetsRetryAfter`, `TestParentPasswordResetConfirm_InvalidToken_Returns410`, `TestRouterWithAuthRateLimiter_ThrottlesOnlyPublicAuthRoutes`. Handler-/Stubtests, kein Beleg für produktive Konfiguration oder wirksame kontobezogene Sperre. [T1] |
| Reset | `TestInitiatePasswordResetIssuesARedeemableLink` prüft Verbrauch, Wiederverwendung, Ablauf und schwaches Passwort; `TestPasswordResetScopesServeTheirPortalOnly`, `TestPasswordResetRateLimitBlocksAfterThreeAttempts`, `TestPasswordResetRateLimitCountsOnlyActionableRequests`. Der 3/h-Test nutzt den gemeinsamen Staff-Reset-Scope; Eltern-Status/Header werden separat im HTTP-Test geprüft. `TestPasswordResetMailLinksToTheScopesPortal` enthält den Eltern-Mailpfad. Kein End-to-End-Beleg für sofortige Sperrung alter Eltern-Access-JWTs. [T2] [T3] |
| Daten/Rechte | `TestGetChildMasterData_ReturnsChainData`, `TestMasterDataUsesSelectedChildGuardianProfile`, `TestUpdateMasterDataField_HealthInfo_AppliesAndAudits`, `TestSubmitSickNote_MissingGuardianPermission`, `TestListChildGuardians_RedactsSocialWorkerContact`. Sie schützen konkrete Daten-/Beziehungspfade, nicht jede denkbare Kombination gespeicherter Rechte. [T4] [T5] [T6] |
| Einladungen/Dateien | `TestGuardianInvitationService_PublicTokenRejectsUnapprovedStatuses`, `TestGuardianInvitationService_Accept_ReusesExistingAccountWithoutPasswordChange`, `TestGuardianInvitationBelongsToItsSchoolOnly`; `TestGuardianAnnouncementTenant_OnlyForTheAudience`, `TestGuardianAnnouncementTenant_DraftAndDisabledFeatureHideTheFile`, `TestGuardianAnnouncementTenant_FeatureOffHidesTheFile`. Keine Aussage über tatsächlich versandte Mails oder produktive Dateispeicherfreigaben. [T7] [T8] |
| Scope/Sitzungen | `TestParentMiddleware_RejectsTenantScope`, `TestParentMiddleware_RejectsPlatformScope`, `TestLogoutLeavesOtherPortalSessionsIntact`, `TestRevokeAllTokensDeletesSessionsAcrossTenants`, `TestRefreshToken_RejectionsRevokeFamilyWithoutIssuingTokens`. Keine erfolgreiche Ausführung oder vollständige Prüfung sämtlicher Eltern-Recovery-Szenarien in diesem Research. [T9] [T10] [T11] |

## 5. Produktiv nachzuweisen und offen

Hier liegt **kein produktiver Nachweis** vor. Folgende Punkte sind Bedingungen
der weiteren Bewertung, keine Behauptung vorhandener Mängel im Betrieb:

1. **Loginabwehr:** Aktivierung und effektive Parameter beider Limits,
   vertrauenswürdige Client-IP-Kette über Frontend/Proxy, Anzahl der Prozesse,
   Sicherheitsprotokollierung und Reaktion auf verteilte Versuche nachweisen;
   3841-c bleibt offen. Begründung: konfigurierbares/prozesslokales IP-Limit und
   fehlender Kontozähler im geprüften Loginpfad. [C1] [C9] [C10] [C27] [C38]
2. **Zugriffsbestand:** Aktive Schulzuordnungen und konkrete Rechte je Kind,
   Einladungsmodus/-freigaben sowie erforderliche Sichtbarkeit von `health_info`,
   Kontaktlisten, Nachrichten und Anhängen prüfen. Auch ein Konto mit bloßem
   Leserecht braucht eine Schutzbewertung. Begründung: die oben belegten
   Lesepfade und Rechte, nicht ein pauschal harmloses Rollenlabel. [C3] [C7]
   [C13] [C16] [C18] [C19] [C20]
3. **Recovery/Betrieb:** Mailzustellung, Linklaufzeiten und Umgang mit falscher
   Adresse, verlorenem/übernommenem Mailkonto sowie unberechtigten Einladungen
   organisatorisch und technisch nachweisen. Eltern-Reset darf bei Konten mit
   weiteren Portalrollen nicht als isolierter Eltern-Passwortwechsel verstanden
   werden. Begründung: gemeinsame Kontozugangsdaten und tokenbasierter Mailweg.
   [C11] [C23] [C24] [C35]
4. **Sitzungsrestzugriff:** Reale JWT-/Refresh-Laufzeiten, Cookies/TLS und das
   Verhalten nach Reset, Logout, Rollen-/Beziehungsentzug und Deaktivierung
   prüfen. Tests aus Abschnitt 4 ausführen beziehungsweise passende Nachweise
   ergänzen; ihre bloße Existenz reicht nicht. [C12] [C14] [C21] [C32] [C33]
   [C34] [C35]
5. **Entscheidung zu Eltern-MFA:** **Empfehlung:** Nach diesen Nachweisen das
   Restrisiko ausdrücklich akzeptieren oder zusätzliche Maßnahmen wählen.
   Wenn Gesundheits-/Familieninformationen und sicherheitsrelevante Änderungen
   mit Passwort und Mail-Recovery nicht angemessen geschützt werden können,
   ist ein unabhängiger zusätzlicher Faktor oder eine passend engere
   Daten-/Aktionsfreigabe erneut zu bewerten. Ein Code an dieselbe kompromittierte
   Mailadresse allein löst das Mailübernahmeszenario nicht. Keine pauschale
   gesetzliche MFA-Pflicht und keine pauschale Ausnahme für Eltern ableiten.
   [L1] [C3] [C11] [C20]

## Quellen am geprüften Commit

Die Links nennen Datei und genaue Zeilen; die maßgeblichen Funktionen stehen
jeweils im Linktitel. Alle Produktquellen außer L1 beziehen sich auf die oben
genannte Codebasis.

[L1]: https://github.com/moto-nrw/legal/blob/7a21f6742782bcf6141470a47333ae5be6c3182f/research/14-dsgvo-mindestumfang.md "Interne Einordnung, Stand 23.10.2026: Maßstab, Entscheidungen für Dokumente und Issues, Quellen und Reichweite; nur lesend herangezogen"
[C1]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/application/parent_login.go#L11-L98 "loginParentWithAudit, issueParentTokens, buildParentMetadata, findGuardianTenantForAccount"
[C2]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/http/auth_handlers.go#L15-L124 "LoginRequest.Bind, Resource.login"
[C3]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/http/master_data_handlers.go#L16-L134 "MasterDataResponse, toMasterDataResponse, getMasterData"
[C4]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/internal/care/parent_today_status.go#L9-L105 "TodayStatus, deriveTodayStatus"
[C5]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/internal/messaging/parent_messaging_service.go#L179-L319 "UnreadMessageCount, GetChildConversation, PostChildMessage"
[C6]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/filestorage/internal/application/attachments.go#L298-L355 "ListGuardianAttachments, OpenGuardianAttachment"
[C7]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/securityruntime/internal/application/authorize/guardian_permission.go#L12-L186 "GuardianPermission-Konstanten, fullParentPortalPermissions, StudentGuardianPermissionSet, StudentGuardianHasPermission"
[C8]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/http/routes.go#L61-L204 "mountEnrollmentRoutes, mountChildStatusRoutes, mountCommunicationRoutes, mountChildCareRoutes, mountChildMasterDataRoutes"
[C9]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/api/serve_middleware.go#L240-L299 "buildAuthRateLimiters, authRateLimitersFromEnv, authMiddleware"
[C10]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/middleware/ratelimit.go#L16-L134 "RateLimiter, NewRateLimiter, defaultRateLimitKey, requestKeyForBucket"
[C11]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/application/password_reset.go#L77-L250 "InitiatePasswordReset, issueResetInAdmin, portalEligible, checkRateLimitInTransaction, ResetPassword, redeemResetInAdmin"
[C12]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/application/session_refresh.go#L115-L233 "refreshClaimsGuard, refreshRotation.run, refreshRotation.lockAccount"
[C13]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/careplan/internal/adapters/parentpostgres/child_repository.go#L60-L165 "ListByAccount, FindForAccount, portalLinks, resolveChildren"
[C14]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/internal/care/child.go#L28-L93 "ResolveOwnedChild, ResolvePermittedChild, childHasPermission"
[C15]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/internal/care/parent_master_data_service.go#L103-L218 "GetChildMasterData, UpdateMasterDataField, requireTrackAEditEnabled, applyTrackAInTx"
[C16]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/internal/care/guardian_contacts.go#L199-L225 "ListChildGuardians: PortalAccess, GuardianEdit, PickupManage und Schulschalter"
[C17]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/internal/care/guardian_family.go#L80-L178 "contactProtected, projectChildGuardian, applyGuardianContactDetails"
[C18]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/internal/care/parent_related_accounts_service.go#L71-L129 "ListRelatedAccounts, relatedAccountForLink"
[C19]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/internal/messaging/parent_announcement_attachments.go#L9-L98 "GuardianAnnouncementTenant, GuardianDeclarationProofAttachmentTenant"
[C20]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/internal/care/parent_related_accounts_service.go#L194-L285 "InviteRelatedAccount, inviteRequiresStaffApproval, RemoveRelatedAccount"
[C21]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/frontend/src/server/auth/parent-config.ts#L33-L194 "parentAuthConfig: authorize, Provider, Cookies und Session"
[C22]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/securityruntime/credentials.go#L5-L26 "HashPassword, VerifyPassword, PasswordTooWeak"
[C23]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/application/guardian_invitation.go#L30-L258 "CreateGuardianInvitation, AcceptGuardianInvitation, provisionGuardianAccount, grantGuardianAccess"
[C24]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/http/auth_handlers.go#L127-L188 "initiatePasswordReset, resetPassword: gemeinsamer Kontoreset"
[C25]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/domain/password_reset.go#L19-L47 "PasswordResetRateLimitThreshold, PasswordResetWindow.Exceeded"
[C26]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/http/password_reset_runtime.go#L53-L66 "renderPasswordResetRateLimit"
[C27]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/roots/owners/root_identity.go#L115-L126 "PasswordReset-Wiring: Expiry und RateLimitEnabled aus Konfiguration"
[C28]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/adapters/postgres/password_reset.go#L120-L144 "FindRedeemablePasswordResetToken, RedeemPasswordResetToken"
[C29]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/domain/guardian_invitation.go#L25-L40 "GuardianInvitation.Redeemable"
[C30]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/sessiontoken/parent_middleware.go#L16-L38 "ParentMiddleware"
[C31]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/application/session_refresh.go#L338-L499 "resolveRefreshHandoff, createAndPersistSuccessor, LogoutWithAudit"
[C32]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/api/session_route_assembly.go#L29-L30 "ParentSessionChain"
[C33]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/sessiontoken/authenticator.go#L77-L117 "Authenticator"
[C34]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/application/session_validation.go#L14-L89 "ValidateSessionTokens, parseSessionRefresh, validateSessionInTx"
[C35]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/api/root_config.go#L70-L77 "rootDomainConfig: Reset- und JWT-Laufzeiten"
[C36]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/api/serve_mounts_portals.go#L41-L46 "mountPortalRoutes"
[C37]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/http/api.go#L114-L142 "RouterWithAuthRateLimiter"
[C38]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/application/account_login.go#L223-L249 "validateLoginCredentials, verifyPassword: Audit/Ablehnung ohne Kontozähler"
[C39]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/adapters/postgres/password_reset.go#L15-L84 "passwordResetWindowLength, PasswordResetWindow, CountPasswordResetRequest: Datenbankfenster je E-Mail"
[C40]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/api/security_principal_middleware.go#L12-L29 "SecurityPrincipalMiddleware, principalSessionFacts: Claims-Projektion"
[C41]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/application/session_revocation.go#L87-L124 "DeleteAccountSessionsWithAudit, ScheduleAccountWideRevoke: sofortiger Widerruf im administrativen Reset-Kontext"
[C42]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/internal/care/parent_master_data_service.go#L495-L542 "loadMasterData: HealthInfo, eigenes Guardianprofil und Telefonnummer"
[T1]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/http/auth_handlers_test.go#L146-L521 "Eltern-HTTP-Auth-, Reset- und Limiter-Tests"
[T2]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/behavior/password_reset_flow_test.go#L64-L225 "Reset-Link-, Scope- und Rate-Limit-Tests"
[T3]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/behavior/password_reset_delivery_test.go#L69-L109 "TestPasswordResetMailLinksToTheScopesPortal"
[T4]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/parent_master_data_service_test.go#L191-L296 "Stammdaten-Kette, ausgewähltes Guardianprofil und Gesundheitsänderung"
[T5]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/parent_write_service_test.go#L574-L605 "TestSubmitSickNote_MissingGuardianPermission"
[T6]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/parent_guardian_service_test.go#L866-L894 "TestListChildGuardians_RedactsSocialWorkerContact"
[T7]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/behavior/guardian_invitation_service_test.go#L262-L370 "Schulbindung, nicht genehmigte Token, vorhandenes Konto ohne Passwortänderung"
[T8]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/workflows/parentportal/internal/care/parent_announcement_attachments_test.go#L28-L115 "GuardianAnnouncementTenant: Zielgruppe, Entwurf, deaktivierte Funktion"
[T9]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/api/parent_scope_middleware_test.go#L72-L145 "TestParentMiddleware_RejectsTenantScope, TestParentMiddleware_RejectsPlatformScope"
[T10]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/behavior/session_scope_test.go#L274-L700 "Portalgetrennter Logout, kontoweiter Sitzungswiderruf über Schulen"
[T11]: https://github.com/moto-nrw/project-phoenix/blob/c66191b29879360e1eb6085ce46d716080a3b3be/backend/modules/identityaccess/internal/application/session_flow_internal_test.go#L90-L147 "TestRefreshToken_RejectionsRevokeFamilyWithoutIssuingTokens"
