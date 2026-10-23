# Externe Einwilligung: Widerruf und Löschweisung umsetzen

Slice 3844-f, Refs #3844, Teil von #3846. Code-Abgleich am 24.10.2026 gegen
`origin/development`, Commit `cb62e5a7127baad83898848c66db6fd518810021`.

**Status: dokumentierter Ablauf auf Basis des Codes, keine Betriebsfreigabe.**
Die Erprobung durch Schule und Betrieb, die produktiven Nachweise und die
rechtliche Prüfung bleiben offen. Dieses Runbook ändert keine Funktionen,
Rechte oder Fristen. Es enthält keine schreibenden SQL-Befehle und ist kein
geprüftes Skript für selektive Löschungen. Die Abnahme steht in Abschnitt 7.

## 1. Auftrag und Zuständigkeit klären

1. Die Schule als Verantwortliche prüft die anfragende Person und ihre
   Berechtigung für das betroffene Kind über ihren bestehenden Kontaktweg.
   Sie unterscheidet Erteilung, Ablehnung, Widerruf, Löschung und Einschränkung.
   Anfragen direkt an moto gehen unverzüglich an die zuständige Schule.
2. Die Schule bestimmt je Funktion Zweck, Datenumfang und Rechtsgrundlage.
   Bei Gesundheitsdaten sind Art. 6 **und** eine Ausnahme nach Art. 9 Abs. 2
   DSGVO zu klären. Ausdrückliche Einwilligung ist eine Möglichkeit, keine
   allgemeine Pflicht für jedes Gesundheitsfeld. Für Hardt sind Zuständigkeit,
   Träger und einschlägige Regeln in Baden-Württemberg gesondert zu prüfen.
   Eine NRW-Annahme oder ein Betreuungsvertrag allein genügt dafür nicht.
3. Stützt sich die Funktion auf Einwilligung, führt die Schule den Nachweis
   extern: konkreter Zweck, gültige Textfassung, Erteilung oder Ablehnung,
   Zeitpunkt, erklärungsberechtigte Person und gegebenenfalls Widerruf. Sie
   bestimmt einen ebenso einfachen Widerrufsweg. Zusätzlich bestehende
   Portaländerungen berücksichtigen: Für Fotos gibt es Widerruf und
   Neuerteilung nach Widerruf im Elternportal. Das ist kein allgemeiner
   Portal-Widerrufsweg für andere Funktionen (Abschnitt 1.1 und 2.1).
4. Die Schule weist ihre berechtigte Leitung bzw. Datenpflege an. Reicht der
   vorhandene Weg nicht aus, erhält das moto-Team eine begrenzte Weisung in
   Textform über den vereinbarten geschützten Supportweg. Vor jeder Aktion
   richtige Schule und richtiges Kind prüfen. Keine mandantenübergreifende
   Datenbankarbeit und keine personenbezogenen Anhänge in GitHub.
5. Erfassen sofort beenden, soweit die Grundlage fehlt oder widerrufen ist.
   Bestehende Daten und Dokumente separat prüfen. Eine andere Grundlage nicht
   nachträglich wählen, nur um den Widerruf zu umgehen. Aufbewahrungspflichten,
   Umfang, Zweck, Zugriff und Endtermin entscheidet die Schule gesondert.

**Rollen:** Einstellungen brauchen je Schlüssel `config:manage` (Protokoll,
Feedback) bzw. `config:update` (Fotos). Kinder bearbeiten benötigt
`users:update` und die serverseitige Berechtigung als Mitarbeiter bzw. Admin;
Kinder endgültig löschen `users:delete` und diese Berechtigung. Nicht allein
vom Rollennamen ausgehen. Fehlende Rechte durch die Leitung klären, nicht
vorübergehend pauschale Adminrechte vergeben. Dokumentrechte stehen unten.

### 1.1 Eingangswege und Abgleich

Externe Erklärungen sind nicht die einzige Quelle. Schule und moto-Team
berücksichtigen folgende im Code vorhandene Wege; API-Pfade beziehen sich
auf das Backend unter `/api`, nicht auf eine Freigabe für direkte Requests.

| Funktion | Erklärungen, Änderungen und weitere Erfassungswege |
|---|---|
| Kinderfotos | Externer Nachweis; Personal-UI bzw. `PUT /students/{id}` mit `photo_consent_given`; `POST /students/{id}/photo` kann mit `consent_acknowledged=true` zugleich erteilen. Elternportal: `GET /parent/me/children/{studentId}/consents`, `PUT`/`DELETE /parent/me/children/{studentId}/consents/photo`. Hinzu kommen Anmeldeübernahme/-abgleich und Kinderimport (Abschnitt 2.5). |
| Raumhistorie/Aufbewahrung | Externe Erklärung; Personal-UI und `GET`/`PUT /students/{id}/privacy-consent`; Kinderimport kann die Aufbewahrungseinwilligung initial anlegen, schreibt eine bestehende dabei nicht um. Das ist `users.privacy_consents`, nicht der Schlüssel `data_processing` des Einwilligungsprotokolls. Raumbesuche entstehen über Personal-/Web-Anmeldung und Raumwechsel, Geräte-Anmeldung/-Raumwechsel und Stundenplanabläufe. Kein entsprechender Elternportal-Einwilligungs- oder Widerrufsweg in den geprüften Routen. |
| Stimmungsabfrage/Feedback | Externe Erklärung; Geräteeingabe `POST /iot/feedback`; Personal-API `POST /feedback/` und `/feedback/batch`, Löschen `DELETE /feedback/{id}`. Keine kindbezogene Einwilligungs-API und kein entsprechender Elternportal-Weg in den geprüften Routen. |
| Gesundheitsangaben | Externe Erklärung, soweit diese Grundlage gilt; Personal-UI und Kinderanlage/-änderung (`POST /students/`, `PUT /students/{id}`, Feld `health_info`); direkte Elternbearbeitung `PATCH /parent/me/children/{studentId}/master-data/student/health_info`; Anmeldeübernahme und Kinderimport. Krankmeldungen (`POST /parent/me/children/{studentId}/sick-note`), Personal-Statusänderungen und Gesundheitsdokumente sind gesonderte Bestände, kein Widerruf dieses Feldes. Keine eigene Gesundheits-Einwilligungs-API. |
| AGB, Datenschutzkenntnisnahme, E-Mail-Kontakt | Anmeldeformular: öffentliche oder angemeldete Einreichung, danach Übernahme/Abgleich; Kinderimport mit Einwilligungsdaten. Eltern-/Personalansicht zeigt die gespeicherten Zustände. `GET /parent/me/children/{studentId}/consents` ist nur lesend; keine `PUT`/`DELETE`-Wege für diese drei Schlüssel. Der reguläre Kinder-Änderungsrequest bietet dafür ebenfalls keine Felder (Abschnitt 2.5). |

Eine Erklärung über Elternbriefe/Einverständnisse (`POST
/parent/me/news/{announcementId}/respond`) hat einen eigenen Antwortverlauf.
Sie ändert die oben genannten Kinderfelder oder Schalter nicht automatisch.
Auch externe Rückmeldungen per vereinbartem Kontaktweg müssen in den Auftrag
einfließen. Für Fotos gilt vor Setzen des Häkchens, vor einem Upload und vor
erneutem Einschalten zwingend der quellenübergreifende Abgleich aus Abschnitt 2.1.

## 2. Erteilung, Ablehnung und Widerruf je Funktion

### 2.1 Kinderfotos

Schulweit: **Einstellungen → Betrieb → Kinderfotos**
(`operations.student_photos_enabled`). Kindbezogen:
**Datenverwaltung → Kinderdaten → Kind → Bearbeiten → Foto**.

**Foto-Portalweg:** Eltern öffnen das Kind im Elternportal und dort
**Angaben → Einwilligungen und Bestätigungen → Foto-Einwilligung**. Bei laufender
Betreuung und passender Beziehungsberechtigung `consent_manage`
(`GuardianPermissionConsentManage`, `parent_portal.consent.manage`) können sie
**Widerrufen → Jetzt widerrufen** wählen. Nach einem protokollierten Widerruf
ist **Erneut geben → Einwilligung geben** möglich, auch nach weiteren
Widerrufen. Es ist kein Weg zur allerersten Erteilung ohne vorherigen Widerruf.
Beide Änderungen funktionieren **unabhängig vom Kinderfotos-Schalter** und
schreiben wirksame Übergänge mit Quelle `parent_portal` ins Einwilligungsprotokoll.
Der Widerruf entfernt auch den Fotoverweis und beauftragt die Dateilöschung;
eine Neuerteilung stellt kein Foto wieder her. Rechte sind kindbezogen, nicht
allein durch ein Elternkonto gegeben. Nach Betreuungsende verweigert der
Schreibweg die Änderung; dann den externen Kontaktweg zur Schule nutzen.

**Verbindlicher Quellenabgleich:** Vor **jedem** Setzen von „Einwilligung der
Eltern liegt vor“, vor **jedem** Foto-Upload (auch mit
`consent_acknowledged=true`) und vor erneutem Einschalten prüft die zuständige
Person den aktuellen Zustand **und** den Einwilligungsverlauf in moto,
einschließlich `parent_portal`, gegen die externen Erklärungen. Der vollständige
Verlauf samt Quelle ist nicht die bloße Statusanzeige im Kinderprofil;
geschützter Prüfweg über das moto-Team: Abschnitt 5.1. Fehlt der Verlauf oder
ist die Zuordnung unklar, kein Häkchen setzen und kein Foto hochladen.

Ein **jüngerer ausdrücklicher Widerruf** aus Portal oder externem Kontakt hat
Vorrang vor einer älteren Erteilung, einem Importdatum oder einer späteren
technischen Übernahme eines alten Formulars. Nicht allein den jüngsten
Datenbankzeitstempel werten: Er kann nur das Nachtragen dokumentieren. Bei
Widerspruch oder unklarer Reihenfolge gilt: **kein Häkchen, kein Upload und
Rückfrage bei den Eltern**. Einen bestehenden widersprechenden Haken nach
Weisung entfernen; einen Portal-Widerruf niemals mit einem alten Papiernachweis
überschreiben. Eine spätere Portal-Neuerteilung ist eine neue ausdrückliche
Erklärung, belegt aber nicht automatisch den extern geführten Zweck-/Fassungs-
nachweis. Schule gleicht diesen ab bzw. klärt ihn mit den Eltern, bevor Fotos
erneut genutzt werden. Nicht automatisch aus „Hinterlegt“ auf Freigabe schließen.

1. **Erteilung:** Quellenabgleich durchführen und den zweckbezogenen Nachweis
   bestätigen. Nur dann bei eingeschalteten Kinderfotos das Häkchen setzen,
   gegebenenfalls ein Foto auswählen und **Speichern**. Der gespeicherte
   Zeitpunkt belegt die Eintragung, nicht den ursprünglichen Zeitpunkt oder
   die Textfassung einer externen Erklärung. Portal-Neuerteilungen nicht
   ungeprüft als extern nachgewiesen übernehmen.
2. **Ablehnung/fehlender Nachweis:** Häkchen nicht setzen, kein Foto hochladen.
   Einen bestehenden widersprechenden Zustand wie einen Widerruf bereinigen.
   Ablehnung extern festhalten: Ein fehlender Zeitstempel allein unterscheidet
   nicht zwischen „abgelehnt“ und „noch nicht angefragt“.
3. **Widerruf bei eingeschalteten Kinderfotos:** Häkchen entfernen und
   **Speichern**. Neu öffnen: Häkchen aus, Foto weg. Der Code entfernt
   Fotoverweis und Einwilligungszeitstempel;
   ein neuer Upload ohne Einwilligungsbestätigung wird abgewiesen; der Abruf
   findet nach dem Widerruf keinen Fotoverweis mehr. Dateilöschung wird
   nach dem Commit versucht und über einen dauerhaften Aufräumauftrag
   nachgeholt. Der Widerruf bleibt im Einwilligungsprotokoll nachvollziehbar.
4. **Nur Foto löschen:** „Foto entfernen“ und **Speichern** entfernt das Foto,
   ist aber kein Ersatz für das Entfernen der Einwilligung. Bereits exportierte
   oder heruntergeladene Kopien durch die Schule gesondert bearbeiten lassen.

Kinderfotos schulweit auszuschalten entfernt **alle** Fotos, nicht nur das
betroffene. Die Einwilligungsnachweise bleiben dabei bestehen. Das ist ein
Ersatzweg bei nicht beherrschbarer Nutzung, kein kindbezogener Widerruf.
Das moto-Team bestätigt den Abschluss offener Datei-Aufräumaufträge separat.

**Widerruf bei bereits ausgeschalteten Kinderfotos:** Nur der Fotobereich der
**Personal-UI** samt Häkchen ist dann nicht sichtbar. Der oben genannte
Elternportal-Widerruf bleibt möglich und wirksam; auch Neuerteilung bleibt
möglich. Das Ausschalten selbst entfernt alte Einwilligungen nicht.

1. Schule führt externe Widerrufe im bestehenden Auftrag nach und berücksichtigt
   bereits erfolgte Portaländerungen. Ist der wirksame Widerruf in moto schon
   protokolliert, nicht nochmals als fehlend behandeln. Andernfalls extern
   **Widerruf in moto noch nicht nachgetragen** vermerken. Eltern mit passender
   Berechtigung können selbst im Portal widerrufen, ohne Fotos einzuschalten;
   diesen Weg nicht für unberechtigte Personen oder andere Funktionen zusagen.
2. **Vor jedem erneuten Einschalten** aktuellen Zustand und vollständigen
   Verlauf aller betroffenen Kinder einschließlich Portaländerungen und
   Anmelde-/Importübernahmen nach obiger Regel abgleichen. Während des
   Abgleichs verbindliche Pause aller Foto-Uploads und aller Änderungen an
   Fotoeinwilligungen organisieren, einschließlich Portal-Neuerteilungen,
   Anmeldeübernahmen und Importen; laufende Vorgänge erst abschließen lassen.
   Neue Widerrufe trotzdem sofort entgegennehmen und berücksichtigen. Ist
   diese Koordination nicht zuverlässig, bleiben Fotos aus und der Auftrag offen.
3. Nur noch fehlende Widerrufe durch berechtigte Eltern im Portal nachtragen
   lassen oder während der vereinbarten Pause Fotos einschalten und in der
   Personal-UI sofort das Häkchen der betroffenen Kinder entfernen, speichern
   und erneut prüfen. **Kein Häkchen aufgrund älterer externer Nachweise setzen.**
   Auch bereits im Portal widerrufene Kinder müssen widerrufen bleiben.
4. Nach dem letzten Nachtrag den Verlauf und aktuellen Zustand nochmals prüfen.
   Erst nach vollständigem Abgleich Uploads wieder zulassen. Bei neuer Erklärung,
   Fehler oder Unterbrechung die Prüfung wiederholen; bei nicht beherrschbarer
   Nutzung Fotos wieder ausschalten (löscht erneut alle Fotos). Danach gilt der
   Quellenabgleich weiterhin vor jedem Häkchen und Upload, nicht nur einmalig.

Dieser Ersatzweg vermeidet ein zusätzliches kurzes Einschalten allein für den
Widerruf. Ausgeschaltet sind Uploads bereits gesperrt; kritisch ist das spätere
Einschalten mit alten Einwilligungszeitstempeln. **Offen:** Nur die Personal-UI
verbirgt den Foto-Widerruf bei ausgeschalteter Funktion, nicht das Elternportal.
`ApplyPhotoConsent` hat kein Feature-Gate; `PUT /students/{id}` kann den
Fotozustand auch dann ändern. Vorschlag: Der Personal-UI das Entfernen der
Fotoeinwilligung auch bei ausgeschalteten Fotos anbieten, ohne Uploads
freizugeben. Ein eigenes neues Einwilligungssystem ist dafür nicht nötig.
Keine der vorhandenen Stellen prüft den externen Nachweis automatisch; auch
Portal-Neuerteilung, Uploadbestätigung, Import und Anmeldeübernahme können einen
Zustand neu setzen. Der vorgeschriebene Quellenabgleich und die Koordination
gleichzeitiger Änderungen sind organisatorisch zu prüfen, kein technischer
Schutz vor jedem Konflikt. Bis dahin ersetzt das Ausschalten diesen Abgleich nicht.
Der beschriebene Ersatzweg ist durch Schule und Betrieb noch zu erproben.

### 2.2 Raumhistorie und Anwesenheitsprotokoll

Schulweit: **Einstellungen → Datenschutz → Anwesenheitsprotokoll**
(`gdpr.attendance_log_enabled`). Kindbezogen gibt es unter **Bearbeiten →
Datenschutz** „Einwilligung zur Datenverarbeitung erteilt“ und eine
Aufbewahrungsfrist. Diese Werte sind **kein Erfassungsverbot**.

Der zugehörige Personal-API-Weg ist `GET`/`PUT /students/{id}/privacy-consent`
(`users:read` bzw. `users:update` plus Kinderzugriff). Auch der Kinderimport
kann diesen Aufbewahrungsdatensatz initial anlegen. Er ist von
`data_processing` aus Anmeldung/Einwilligungsprotokoll zu unterscheiden.
In den geprüften Elternportal-Routen gibt es hierfür keinen Erteilungs-/
Widerrufsweg. Geräte-, Web- und Stundenplan-Eingaben bleiben als Erfassungswege
für Besuche zu berücksichtigen, auch wenn dieser Datensatz nicht akzeptiert ist.

1. **Erteilung:** Extern zweckbezogen nachweisen. Die kindbezogenen Werte nur
   gemäß geprüfter Weisung pflegen. Eine Einwilligung eines Kindes erlaubt
   nicht, die Historie aller anderen Kinder einzuschalten. Der Verantwortliche
   muss den tatsächlich erfassten Personenkreis prüfen.
2. **Ablehnung/fehlender Nachweis:** Nicht als erteilt eintragen. Benötigt die
   längerfristige Raumhistorie dieses Kindes eine Einwilligung, darf sie nicht
   weiterlaufen. Ein fehlendes Häkchen verhindert Raumbesuche nicht: Ohne
   akzeptierte Einwilligung greift eine schulweite Standard-Aufbewahrungsfrist.
3. **Widerruf:** Kindbezogen das Häkchen entfernen und speichern, aber damit
   den Auftrag **nicht** als erledigt melden. Es gibt keinen belegten
   kindbezogenen Schalter zum Stoppen der Raumerfassung. Als vorhandenen
   Ersatzweg die Historie schulweit ausschalten. Löschfolgen vorher mit der
   Schule abstimmen und mit **LÖSCHEN** bestätigen. Fehlender Erfolg oder
   fehlende Befugnis bleibt ein offener Auftrag mit sofortiger Eskalation.
4. **Grenze des Ersatzwegs:** Ausschalten löscht abgeschlossene Raumbesuche
   vor dem heutigen Berliner Tag sofort; der Nachtlauf entfernt später die
   weiteren fälligen Abschlüsse. Heutige Abschlüsse und offene Besuche bleiben
   zunächst erhalten. Aktueller Aufenthaltsort und Tagesanwesenheit bleiben
   getrennte Verarbeitungen mit eigenen Zwecken und Fristen. Das Ausschalten
   ist daher kein vollständiger Erfassungsstopp für dieses Kind.

`gdpr.room_detail_visible_days` begrenzt die Sichtbarkeit, nicht die Erfassung
oder vollständige Speicherung. Ein kleinerer Wert erfüllt keinen Widerruf.
Sind auch die verbleibenden Tages-/Ortsdaten nicht rechtmäßig erforderlich,
den betreffenden raumbasierten Ablauf bis zu einer wirksamen Lösung nicht
nutzen. **Offen:** kindbezogener Erfassungsstopp und gegebenenfalls sofortige
selektive Löschung ohne Verlust rechtmäßig benötigter Tagesdaten. Vor einer
internen Lösung Tenantgrenze, betroffene Lese-/Schreibwege und Fehlerfälle
lokal testen; dieses Runbook stellt dafür keinen freigegebenen Löschweg bereit.
Nicht das ganze Kind löschen, nur weil es die optionale Historie ablehnt.

Details und Mengenprüfung: [Raumhistorie, Slice 3844-b](raumhistorie-3844-b.md).

### 2.3 Stimmungsabfrage / Feedback

Schulweit: **Einstellungen → Datenschutz → Feedback der Kinder beim Gehen**
(`feedback.enabled`). Berechtigung: `config:manage`.

Erfassung erfolgt über Geräte-API `POST /iot/feedback` (Geräteauthentisierung)
und Personal-API `POST /feedback/` bzw. `/feedback/batch` (`feedback:create`).
Beide nutzen die Feedback-Funktion mit schulweiter Sperre. In den geprüften
Elternportal-Routen gibt es weder eine Stimmungsabfrage noch einen eigenen
Feedback-Einwilligungs-/Widerrufsweg. Eine Rückmeldung zu einem Elternbrief ist
ein anderer Vorgang und schaltet diese Erfassung nicht um.

1. **Erteilung:** Extern für den konkreten Zweck nachweisen. Erst nach Prüfung
   des erfassten Personenkreises einschalten. Einwilligung eines Kindes ist
   keine Freigabe für die ganze Schule.
2. **Ablehnung/fehlender Nachweis:** Für dieses Kind keine Stimmung erfassen,
   weder am Terminal noch durch nachträgliche Mitarbeitereingabe. Es gibt
   keinen kindbezogenen Ausschluss-Schalter. Eine bloße Bitte ans Kind oder
   eine ungetestete Absprache im Team ist kein verlässlicher Schutz.
3. **Widerruf:** Erfassung für dieses Kind beenden. Solange ein organisatorischer
   Ausschluss nicht in allen genutzten Terminal- und Mitarbeiterschritten
   zuverlässig geprüft ist, **Feedback schulweit ausschalten**. Der Code
   sperrt dann neue Einzel-/Sammeleinträge und Feedback-Lesewege. Die
   Betreuung und Abmeldung müssen ohne Stimmungsantwort möglich bleiben.
4. **Bestand:** Ausschalten löscht vorhandene Rückmeldungen nicht. Diese haben
   eine eigene Frist (`feedback.data_retention_days`, siehe Fristenliste).
   Besteht eine sofortige Löschweisung, nicht nur auf Fristablauf warten.
   Das moto-Team muss einen getrennten, tenant-sicheren Löschauftrag prüfen.

Die Backend-API kennt Löschen einzelner Feedback-Einträge mit `feedback:delete`
(auch bei ausgeschalteter Funktion). Das ist kein nachgewiesener schulischer
Bildschirm und kein geprüftes Sammelverfahren für ein Kind. **Offen:**
vollständige Bestandsauswahl, selektive Löschung, Prüfung und Nachweis. Kein
ungeprüftes SQL und kein erneutes Einschalten nur zum Suchen alter Einträge.

### 2.4 Gesundheitsangaben, soweit die konkrete Nutzung betroffen ist

Kindbezogen: **Datenverwaltung → Kinderdaten → Kind → Bearbeiten →
Gesundheitsinformationen** (`health_info`), Berechtigung `users:update`.
Es gibt keinen eigenen Gesundheits-Einwilligungsschalter. Das Häkchen unter
„Datenschutz“ steuert diese Angaben nicht. Slice 3844-h bleibt unberührt.

Weitere Eingänge für `health_info`: Kinderanlage/-änderung in der Personal-API,
Übernahme passender Anmeldefelder und Kinderimport. Im Elternportal unter
**Kind → Angaben → Gesundheit** lässt sich das Feld direkt ändern, über
`PATCH /parent/me/children/{studentId}/master-data/student/health_info`.
Das braucht laufende Betreuung, die kindbezogene Berechtigung
`parent_portal.master_data.edit` und die schulweite Freigabe
`operations.parent_master_data_edit_enabled`. Das ist eine Datenänderung,
keine Gesundheits-Einwilligung und kein automatischer Widerruf anderer
Bestände. Krankmeldungen, Kranktage und Dokumente haben eigene Schreibwege.

1. **Erteilung:** Nur die erforderlichen Angaben für den geprüften Zweck
   eintragen und speichern. Falls Einwilligung die Grundlage ist, den externen
   Nachweis vorher prüfen. Sonst die bestätigte Art.-6-/Art.-9-Grundlage
   dokumentieren, nicht zusätzlich pauschal eine Einwilligung verlangen.
2. **Ablehnung/fehlende Einwilligung bei Einwilligungsgrundlage:** Feld leer
   lassen. Unzulässig vorhandene Angaben nach Weisung entfernen. Keine
   Ausweichkopie in Notizen, Elternnachrichten oder Dokumenten anlegen.
3. **Widerruf bei Einwilligungsgrundlage:** Feld vollständig leeren,
   **Speichern**, erneut öffnen und leeren Zustand prüfen. Damit verschwinden
   weder Atteste noch Medikamentenpläne, Kranktage, Notizen, Anmeldedaten oder
   bereits erfolgte Exporte. Diese Bestände gesondert nach Abschnitt 3 prüfen.
4. **Erneute Eingabe verhindern:** Schule muss Betreuungsteam, Eltern-Eingaben,
   Anmeldungen und Übernahmen in den Ablauf einbeziehen. Eltern können
   Gesundheitshinweise unmittelbar bearbeiten; Leeren sperrt das Feld nicht.
   Ohne nachgewiesen verlässlichen Ablauf bleibt der Erfassungsstopp offen.
   Betroffene Eingabe-/Übernahmewege bis zur Lösung nicht nutzen; bei Bedarf
   technische Steuerung separat umsetzen. Keine produktive Wirksamkeit behaupten.

Neue Gesundheitsänderungen speichern im Änderungsprotokoll nur Zustandsmarken,
nicht den freien Gesundheitsinhalt. Für Altbestand existieren Maskierungs-
migrationen; deren produktive Ausführung ist nachzuweisen. Andere Felder und
Dokument-Dateinamen können weiterhin Personenbezug enthalten.

### 2.5 AGB, Datenschutzkenntnisnahme und E-Mail-Kontakt

Die drei Schlüssel `agb`, `data_processing` und `email_contact` im
Einwilligungsprotokoll sind keine Schalter für Gesundheit, Raumhistorie oder
Feedback. Insbesondere ist `data_processing` nicht `users.privacy_consents`.
Der im Code verwendete Begriff „consent“ entscheidet nicht, ob rechtlich eine
Einwilligung, Kenntnisnahme oder Vertragsbestätigung vorliegt.

1. **Anmeldung:** Öffentliche Einreichung oder `POST
   /parent/enrollments/{tenantSlug}/submit` im Elternportal enthält die
   konfigurierten Erklärungen. Erst die Übernahme bzw. der Abgleich schreibt
   die Zeitpunkte am Kind; Auditquelle `enrollment`. Erneute Abgleiche können
   vorliegende Zustände ersetzen. Bei ersetzender Übernahme werden explizit
   nicht erteilte konfigurierte Erklärungen geleert; fehlende Formularschlüssel
   bedeuten nicht automatisch Widerruf. Das betrifft auch `photo`.
2. **Import:** Kinderimport kann diese Zeitpunkte sowie Fotoeinwilligung bei
   Anlage und Aktualisierung übernehmen, Quelle `import`. Vor Übernahme mit
   neueren Erklärungen und Widerrufen abgleichen; eine alte Importdatei oder
   wiederholte Anmeldeübernahme ist keine neue Erklärung der Eltern.
   `migration_snapshot` kennzeichnet historischen Bestand, keinen laufenden
   Eingangsweg für eine aktuelle Erklärung.
3. **Anzeige:** Eltern sehen die drei Zustände unter **Einwilligungen und
   Bestätigungen**, Personal im Kinderprofil. Die Eltern-API bietet dafür nur
   `GET /parent/me/children/{studentId}/consents`; Änderungsaktionen existieren
   dort nur für Fotos. Der reguläre `PUT /students/{id}` hat für diese drei
   Zeitpunkte keine Änderungsfelder. Keine pauschale Portal-Widerrufsfunktion
   aus der Überschrift oder dem Auditmodell ableiten.
4. **Ablehnung/Widerrufs- oder Korrekturanfrage:** Über den vereinbarten
   Kontaktweg an die Schule geben, Zweck und rechtliche Wirkung einzeln klären.
   Die Oberfläche zeigt keinen individuellen Änderungsweg für diese drei
   Zustände. Eine nötige Korrektur bzw. Umsetzung als gesonderte Weisung an
   moto bearbeiten; dieses Runbook liefert dafür keinen geprüften Schreibweg.
   Keine neue Anmeldung oder Importdatei nur zur Umgehung eines Widerrufs
   verwenden. Auch eine E-Mail-Kontakt-Zeitmarke belegt nicht, dass sämtliche
   Nachrichtenwege automatisch gesperrt oder freigegeben werden.

## 3. Lösch- und Einschränkungsweisung ausführen

### Einzelne Angaben und Dokumente

1. Weisung der Schule eingrenzen: betroffene Zwecke, Felder, Dokumentkategorien,
   Zeitraum und Kopien. Aufbewahrung oder Einschränkung nicht mit einem
   pauschalen „alles löschen“ übergehen. Ein Widerruf verlangt nicht automatisch
   die Löschung des gesamten betreuten Kindes.
2. Angaben wie oben bearbeiten. Für Dateien **Datenverwaltung → Kinderdaten →
   Kind → Dokumente** öffnen. Betroffene Datei über **Löschen** auswählen und
   im Dialog „Dokument löschen“ zuerst **Ja, löschen**, dann
   **Endgültig löschen** wählen.
   Kategorien Attest/Impfnachweis/
   Medikamentenplan brauchen `student_documents:health`, Sorgerecht
   `student_documents:legal`, sonst `users:update`. Eine Rolle mit nur einem
   Kategorienrecht sieht nicht zwingend alle Dateien. Vollständigkeit durch
   entsprechend berechtigte Bearbeiter prüfen lassen.
3. Liste neu laden und Änderung prüfen. Der Eintrag im **Änderungsprotokoll**
   bleibt zunächst erhalten, einschließlich Kategorie und Anzeigename der Datei,
   aber ohne deren Inhalt. Dateientfernung kann fehlschlagen und wird durch
   Aufräumläufe nachgeholt. Den Datenbank-/UI-Erfolg getrennt vom tatsächlichen
   Entfernen der Datei bestätigen lassen.
4. Schule prüft Ausdrucke, Downloads, eigene Ablage und Empfänger bereits
   versandter Daten. moto-Team prüft auf begrenzte Weisung weitere betroffene
   Bestände (etwa angenommene Anmeldung, Nachrichtenanhänge, Dateiablage,
   Diagnosekopien). Nicht alle Bestände sind bloß aus dem Kinderprofil löschbar.
   Nicht erledigte Teile mit Zuständigkeit und Termin offen dokumentieren.

**Einschränkung ist nicht Löschen oder Ausblenden.** Es gibt hier keinen
belegten kindbezogenen Sperrmodus über alle Datenarten. Muss etwas ausschließlich
aufbewahrt, aber nicht mehr regulär genutzt werden, benennt die Schule
zulässigen Zweck, berechtigte Personen und Endtermin. Betrieb prüft einen
getrennten geschützten Aufbewahrungsweg und wirksame Sperre vor weiterer Nutzung.
Ein unverändert sichtbares Feld oder eine Rollenänderung ohne Prüfung aller
Lese-/Exportwege genügt nicht. Solange der Schutz nicht gewährleistet ist,
betroffene Verarbeitung aussetzen und Einschränkung als **offen** behandeln.

### Ganzes Kind endgültig löschen, nur bei entsprechender Weisung

1. Berechtigte Leitung öffnet **Datenverwaltung → Kinderdaten → Kind → Löschen**.
   Vorschau der gelöschten/gelösten Daten lesen. Bei vorgeschriebener
   Aufbewahrung vor Bestätigung stoppen und Einschränkungsweg klären.
2. Grund **Datenschutz-/Löschanfrage** wählen, Bestätigung anhaken, Namen im
   Dialog eingeben und **Kind endgültig löschen** wählen. Namen nur dort
   eingeben, nicht in den Durchführungsnachweis kopieren. Ändert sich die
   Vorschau oder besteht ein Konflikt beim Heimweg eines Begleitkindes,
   den Konflikt bearbeiten und eine neue Vorschau prüfen, keine Schutzprüfung
   umgehen. Für Abgänger ist der Löschweg unter dem Jahrgangswechsel vorgesehen.
3. Nach Erfolg prüfen, dass das Kind nicht mehr abrufbar ist. Der Workflow
   löscht/verändert verknüpfte Datensätze in einer Schultransaktion, darunter
   Feedback und Einwilligungen; Dokument-Aufräumaufträge werden darin gesichert.
   Angenommene Anmeldungen des Kindes werden einbezogen. Person und
   Jahrgangswechsel-Historie werden anonymisiert, nicht pauschal jede Zeile
   entfernt. Maßgeblich bleibt die konkrete Vorschau samt Fehlerergebnis.
4. Elternkontakte/-konten, andere Kinder und gemeinsame Termine bleiben
   absichtlich erhalten. Elternkontakte ohne weiteres Kind folgen einer
   eigenen Frist. Änderungsdaten und Audit-Verweise sind gesondert zu prüfen;
   der datensparsame Löschungsnachweis bleibt. **Betreuung beenden** ist kein
   Ersatz: Das setzt einen letzten Betreuungstag, löscht aber nicht sofort.
5. moto-Team bestätigt offene Foto-/Dokument-Datei-Aufräumaufträge und
   Nebensysteme getrennt. Teilfehler nicht als vollständigen Abschluss melden.
   Nachweis erfolgreicher DB-Löschung allein genügt nicht für „alle Kopien weg“.

Bestehende Fristen, auch für Änderungs- und Löschprotokolle:
[Löschfristen](../reference/loeschfristen.md). Dieser Slice verlängert oder
verkürzt keine Frist und setzt keine gesetzliche Aufbewahrung neu fest.

## 4. Sicherungen und Wiederherstellung

Sicherungskopien werden nicht durch die UI-Löschung eines Kindes einzeln
bereinigt. Betrieb muss den tatsächlichen Zyklus samt Replikaten und
Zwischenkopien belegen. Der Vertragsentwurf sieht höchstens 90 Tage vor;
das ist weder eine allgemeine DSGVO-Frist noch ein produktiver Nachweis.
Keine neue Sicherung oder Kopie darf die ursprüngliche Frist neu beginnen.

Die Weisung bleibt bis zum bestätigten Auslaufen der betroffenen Kopien
nachverfolgbar. Sicherungen sind nicht für reguläre Nutzung freigegeben.
Nach Wiederherstellung bleiben Anwendungen gesperrt, bis Löschungen und
Einschränkungen erneut umgesetzt und geprüft sind. Der datensparsame
Durchführungsnachweis unten ersetzt keine zugriffsbeschränkte, ausführbare
Wiederholungsliste: Zuordnung zu bereits vorhandenen geschützten Aufträgen muss
auch nach Verlust der aktuellen DB möglich sein. **Offen:** Vollständigkeit
und Erprobung dieser Wiederholung, siehe [Backup-Runbook](daily-backup.md).
Ohne diesen Nachweis keine Zusage, ein Restore könne erledigte Weisungen
nicht rückgängig machen.

## 5. Was als Nachweis taugt

| Vorhandener Mechanismus | Aussage und Grenze |
|---|---|
| `audit.student_consent_changes` | Ereignisse für `agb`, `data_processing`, `email_contact`, `photo`; Aktion, Quelle, Zeitpunkt und Akteur. Foto-Portaländerungen tragen `parent_portal`; weitere Quellen sind `tenant_portal`, `enrollment`, `import`, historisch `migration_snapshot`. Kein eigener Gesundheitstyp, keine externe Originalerklärung oder Textfassung. „Nicht hinterlegt“ ist kein Ablehnungsnachweis. |
| Änderungsprotokoll beim Kind (`audit.student_field_edits`) | Feldänderungen und Dokumentereignisse. Gesundheitswerte sind im aktuellen Schreibweg maskiert; Dateinamen und Akteure können Personenbezug tragen. Kein vollständiges Weisungsregister. |
| `audit.data_deletions` und Nachweis der Kinderlöschung | Datenart, Mengen und Ausführung; der Workflow erhält einen Löschungsnachweis. Belegt weder physische Dateientfernung noch ausgelaufene Sicherungen. Nicht jede manuelle Feld-/Dateiänderung schreibt in diese Tabelle. |
| Datei-Aufräumaufträge und Laufnachweise | Betrieb muss Erfolg und Restmenge prüfen. Ein angelegter Auftrag ist noch keine gelöschte Datei. |

Die Schule bewahrt ihre Erklärungen in ihrem geschützten Bestand auf.
In GitHub, PRs und technischen Prüfberichten nur Schritte, Status, Zeitpunkt,
Mengen und Code-/Versionsstand verwenden. Keine Namen, Kinder-/Konto-IDs,
Gesundheitsinhalte, Dateinamen, vollständigen Auditzeilen oder Screenshots
mit Kinderdaten übernehmen. IDs sind nicht anonym. Innerhalb des bestehenden
geschützten Weisungsverfahrens darf die nötige Zuordnung erhalten bleiben;
kein zweites Register mit Personendaten für dieses Runbook anlegen.

Datensparsame Abschlussvorlage für das vorhandene Verfahren:

```text
Auftragstyp / betroffene Funktion:
Eingang und von der Schule freigegebener Umfang:
Code-/Auslieferungsstand und Prüfzeitpunkt:
Erfassung gestoppt: ja / nein / teilweise; geprüfte Wege:
DB-Änderung: erfolgreich / fehlgeschlagen / nicht betroffen; Mengen:
Dateien: abgeschlossen / offen / nicht betroffen; Restmenge:
Externe Kopien durch Schule geprüft: ja / offen:
Einschränkung/Aufbewahrung gesondert bestätigt: ja / offen / nicht betroffen:
Sicherungszyklus und Wiederholung nach Restore: bestätigt / offen:
Offene Schritte, zuständige Rolle und Termin:
Bestätigung an Schule: Teilstand / vollständig im bestätigten Umfang:
```

### 5.1 Fotoverlauf vor Abgleich, Häkchen und Upload prüfen

Die Personalansicht und `GET /parent/me/children/{studentId}/consents` liefern
den aktuellen Zustand samt Zeitpunkt, **nicht** den vollständigen Verlauf mit
Quelle. Fehlt dieser zur Prüfung, bittet die Schule das berechtigte moto-Team
über den geschützten Auftrag um eine begrenzte lesende Prüfung. Bis zum Ergebnis
kein Häkchen setzen, kein Upload und keine Import-/Anmeldeübernahme, die den
Fotozustand ändert. Keine generelle Datenbankberechtigung an die Schule vergeben.

Codequelle: `ListByStudentID` in
`backend/modules/auditlog/internal/adapters/auditpostgres/student_consent_change.go`.
Für einen bestehenden berechtigten Lesezugang kann das moto-Team in `psql`
die intern bestätigten Variablen `school_id` und `student_id` binden und
folgenden **nur lesenden** Prüfblock verwenden. Keine DSN oder Rohzeilen in
GitHub übernehmen. Der Block ist anhand des Schemas abgeglichen, in diesem
Slice aber nicht gegen eine Datenbank ausgeführt; die lokale Erprobung bleibt
Teil der Abnahme. Keine Erweiterung auf andere Kinder oder Schulen.

```sql
BEGIN READ ONLY;
SET LOCAL ROLE phoenix_tenant;
SET LOCAL app.current_tenant_id = :'school_id';
SELECT created_at, action, source
FROM audit.student_consent_changes
WHERE tenant_id = :'school_id'::bigint
  AND student_id = :'student_id'::bigint
  AND consent_key = 'photo'
ORDER BY created_at, id;
ROLLBACK;
```

Schule und moto-Team gleichen diesen Verlauf mit dem aktuellen Fotozustand und
den externen Erklärungen ab. `created_at` ordnet die Eintragung in moto, nicht
zwingend die ursprüngliche externe Erklärung. Keine Zeilen sind kein Beleg für
eine Erteilung oder Ablehnung. Bei lückenhaftem Verlauf oder widersprechenden
Zeitpunkten bei den Eltern nachfragen, nicht aus einem alten Haken folgern.
Neue Portaländerungen zwischen Prüfung und Aktion erfordern einen erneuten
Abgleich; die Abfrage sperrt keine gleichzeitige Neuerteilung und ist kein
automatischer Konfliktschutz.

## 6. Abgleich mit bestehenden Vorarbeiten

- **#1583:** Anmeldehinweise und gespeicherte Zustimmungen sind kein Ersatz
  für Zweckprüfung oder externe Nachweise. Eine Datenschutzinformation ist
  keine pauschale Einwilligung. Die vier vorhandenen Protokollschlüssel
  erlauben keinen Rückschluss auf eine Gesundheits-/Raumhistorien-Freigabe.
- **#3740 / #3430:** [Elternerklärungen](../reference/elternerklaerungen.md)
  haben eigene Antworten und Nachweise; sie ändern Fotozustand und andere
  Kinderfelder nicht automatisch. Rechtliche Einsatzfreigabe und Formfragen
  bleiben unter #3740 offen. Für diesen extern geführten Ablauf keinen
  automatischen Widerrufs-Durchgriff aus Elternbriefen zusagen.
- **Slices 3844-b/d/e:** Historienlöschung beim Ausschalten, Feedback-Sperre
  und neutrale Texte nutzen. Diese schulweiten Mechanismen schließen die
  kindbezogenen Lücken nicht. Slice 3844-h wird hier nicht implementiert.

## 7. Abnahme durch Schule und Betrieb, noch offen

Nur lokal mit synthetischen Daten erproben; keine produktiven Personendaten
für Prüfungen verwenden. Die produktive Konfiguration und Auslieferung muss
danach ein berechtigter Mensch gesondert bestätigen.

1. Schule bestätigt Zwecke, Rechtsgrundlagen, externen Nachweis-/Widerrufsweg,
   Verantwortlichkeiten und Aufbewahrung je Funktion, für Hardt gesondert.
2. In einer lokalen Schule zwei synthetische Kinder, in einer zweiten Schule
   ein Kontrollkind nutzen. Für Fotos Erteilung, Ablehnung, Widerruf,
   gespeicherten Zustand, gesperrten Abruf/Upload und Dateientfernung prüfen.
   Elternportal-Widerruf und anschließende Neuerteilung jeweils bei ein- und
   ausgeschalteten Fotos, mit/ohne Beziehungsrecht und nach Betreuungsende prüfen.
   Alten externen Nachweis, jüngeren Portal-Widerruf sowie spätere
   Portal-Neuerteilung ohne geklärten externen Nachweis als Konfliktfälle
   durchspielen. Vor Häkchen, Upload, Import/Anmeldeübernahme und erneutem
   Einschalten den Verlauf aus 5.1 abgleichen, auch bei parallelen Änderungen.
   Schulweite Ersatzmaßnahmen vorher als solche freigeben.
3. Historie ausschalten: ältere Abschlüsse weg, Tagesdaten/offene Besuche
   vorhanden, Nachtlauf nachvollziehbar. Feedback ausschalten: neue Erfassung
   gesperrt, alte Einträge nicht dadurch gelöscht. Gesundheitsfeld leeren und
   mögliche Wiedereingaben prüfen. Lücken nicht als bestandenen Stopp werten.
4. Einzelnes Dokument und auf gesonderte Weisung ein Testkind löschen:
   Vorschau, Rechte, falsche Schule, Fehler-/Wiederholungsfall, Audit,
   verbliebene gemeinsame Daten und Datei-Aufräumaufträge prüfen. Kontrollkind
   und andere Schule unverändert. Einschränkungsauftrag ohne belegten
   Sperrweg bleibt offen.
5. Betrieb belegt Dateiabschluss, Nebensysteme, Sicherungsende und Restore mit
   erneuter Umsetzung der Weisungen. Schule bestätigt das Ergebnis im
   tatsächlich erledigten Umfang. Bis dahin keinen produktiven Vollzug oder
   rechtliche Freigabe aus diesem Dokument ableiten.

## Code-Belege und rechtliche Arbeitsgrundlage

Pfade relativ zur Repository-Wurzel, Stand siehe Kopf:

- Foto: `frontend/src/components/students/student-photo-section.tsx`,
  `backend/modules/peopledirectory/internal/domain/student_photo.go`,
  `internal/application/student_photo.go` unter demselben Modul.
- Foto-Portalweg und Grenzen anderer Portaländerungen:
  `backend/workflows/parentportal/http/routes.go`,
  `backend/workflows/parentportal/internal/care/parent_consent_service.go`,
  `backend/modules/peopledirectory/internal/domain/student_consent.go`,
  `frontend/src/components/parent/child/child-consents-section.tsx`.
- Anmelde-/Importeingänge:
  `backend/modules/enrollment/internal/application/targeted_field_student.go`,
  `backend/modules/enrollment/internal/application/targeted_fields.go`,
  `backend/modules/dataimport/internal/application/importservice/student_import_owner.go`.
- Personal-Änderungsfelder und Eltern-Gesundheitseingabe:
  `backend/modules/careplan/http/types.go`,
  `backend/modules/careplan/http/student_update_handlers.go`,
  `backend/workflows/parentportal/internal/care/parent_master_data_service.go`.
- Historie: `backend/modules/studentpresence/internal/application/presence/cleanup_service.go`;
  API `backend/modules/studentpresence/http/student_resource.go`,
  `backend/modules/studentpresence/inbound/presence/api.go`;
  UI-Bestätigung `frontend/src/components/settings/settings-field.tsx`.
- Feedback: `backend/modules/feedback/internal/application/service.go`,
  `public_operations.go` daneben, `backend/modules/feedback/http/api.go`.
  Geräteeingang: `backend/modules/devicescan/http/data/feedback_resource.go`.
- Gesundheit/Änderungsprotokoll:
  `frontend/src/components/students/personal-info-edit-panel.tsx`,
  `backend/modules/peopledirectory/internal/domain/student_audit.go`,
  `backend/database/migrations/001015475_mask_student_health_audit.go`,
  `001015483_mask_parent_health_history.go` daneben.
- Dokumente: `frontend/src/components/students/dokumente-tab.tsx`,
  `backend/modules/careplan/http/document_permissions.go`,
  `backend/modules/careplan/student_document_storage.go`.
- Kinderlöschung: `backend/workflows/studentdeletion/deletion.go`,
  `dependents.go`, `audit_records.go`, `http/student_delete_handlers.go` daneben.
- Einwilligungsprotokoll:
  `backend/modules/auditlog/records/student_consent_change.go`.

Rechtliche Arbeitsgrundlagen, nur gelesen, keine Rechtsfreigabe durch moto:
[Mindestumfang vom 23.10.2026](https://github.com/moto-nrw/legal/blob/main/research/14-dsgvo-mindestumfang.md),
[interne Checkliste](https://github.com/moto-nrw/legal/blob/main/vertragspaket/intern/checkliste.md),
[AVV-Entwurf Ziff. 2.4, 7, 8 und Anhang 1](https://github.com/moto-nrw/legal/blob/main/vertragspaket/03-avv.md).
Vorschläge für Elternvorlagen und AVV/TOM stehen nur im zugehörigen PR;
das Legal-Repository bleibt unverändert.
