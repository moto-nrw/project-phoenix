# MFA-Pflicht für privilegierte Konten (Slice 3841-f)

## Stand und Grenze

**Im Code umgesetzt:** Die effektiven Rechte einer Schule entscheiden, nicht
Rollennamen. Nutzerverwaltung, Rollen-/Rechteverwaltung, Auth-Verwaltung,
Sicherheits-/Systemeinstellungen, Personalverwaltung, Gesundheitsrechte und
`staff:financial` verlangen einen zweiten Nachweis. Die genaue Untergrenze steht
in `backend/modules/identityaccess/internal/domain/mfa_privilege.go`.
`users:create/update/delete` verwalten Personen-/Kinderdaten, nicht Konten oder
Rollen (siehe `mountAdminRoutes`); allein diese Rechte erzwingen keine MFA.
Wildcards zählen entsprechend ihrem tatsächlichen Zugriff. Neue Registry-Rechte
müssen im Klassifikationstest ausdrücklich bewertet werden.

`off` und `required_admins` ändern diese Untergrenze nicht. `required_all`
erweitert die Schulpflicht auf alle Mitarbeitenden. Die Optionswerte bleiben
kompatibel. Elternzugänge bleiben unverändert und geben keine Personalrechte aus.
Operator-MFA bleibt unverändert, einschließlich der vorhandenen Demo-Ausnahme.

**Produktiv nachzuweisen, hier offen:** Migration und Rollout, funktionierender
Mailversand, Zugriff der Betroffenen auf ihr Postfach, Bestandszahlen, Anmeldung
und Wiederherstellung pro betroffener Schule. Ein grüner Test ist kein Nachweis
am produktiven System. Dieser Slice bestätigt weder AK1/2 (Geräte), AK5
(Anmeldeangriffe) noch AK6 (gesonderte Elternbewertung) vollständig.

## Anmeldung und Bestand

1. Öffnen Sie die moto-Adresse Ihrer Schule und melden Sie sich an.
2. Beim ersten Mal wählen Sie `Code an meine E-Mail senden`.
3. Öffnen Sie die E-Mail und geben Sie den sechsstelligen Code ein.
4. Erst nach erfolgreicher Prüfung öffnet moto die Schule.

Ein vorhandener Passkey mit bestätigter Benutzerprüfung ist ebenfalls ein
zweiter Nachweis. Seine Einrichtung für Schulpersonal existiert bereits:
Passkey-Registrierung verlangt einen bestätigten E-Mail-Code, Passkey-Anmeldung
verlangt WebAuthn `VerificationRequired`. Diese Funktion wird nicht auf Eltern
ausgeweitet. moto schule nutzt seine bestehende E-Mail-Code-Einrichtung.

Gemerkte Geräte bleiben schulgebunden und nur bei erlaubter Schulrichtlinie
gültig. Sie stammen aus einer bestätigten MFA-Anmeldung. Vor dem Mint wird
ihre Freigabe unter der Kontosperre erneut geprüft; ein zwischenzeitlicher
Widerruf oder Reset darf keinen neuen Sitzungsnachweis erzeugen. Die Einstellungen für
gemerkte Geräte bleiben deshalb auch bei `off` sichtbar. Ein noch nicht
angemeldetes Konto kann die Einrichtung nicht mit einem alten Cookie überspringen.

Migration `1.15.480` setzt für alle vorhandenen Refresh-Sitzungen den neuen
Nachweis `mfa_verified` auf `false`. Beim nächsten Refresh muss ein inzwischen
MFA-pflichtiges Konto sich neu anmelden. Es erhält dann die Einrichtungsabfrage
oder eine Code-Abfrage. Neue bestätigte Sitzungen behalten ihren Nachweis über
Rotation und zulässige Wiederherstellung verlorener Refresh-Antworten.
Access-Token bleiben bis zu ihrer vorhandenen Ablaufzeit gültig; es gibt keine
nachträgliche Sofortsperre aller Access-Token.

Ein Schulwechsel erhält die Bestätigung der laufenden Anmeldung im OGS-Portal.
Nach Zurücksetzen der zweiten Prüfung gilt diese Bestätigung nicht mehr.
Sie gilt auch nicht nach Abmeldung oder Ablauf der Sitzung.
Fehlt die Bestätigung, melden Sie sich direkt bei der Zielschule an.
Eine Passwort-Zurücksetzung stellt keine Sitzung und keinen MFA-Nachweis aus.

## Verlorener zweiter Faktor

1. Identität über das vereinbarte Supportverfahren prüfen. Keine Codes oder
   Geheimnisse im Ticket, Chat, Screenshot oder Nachweis ablegen.
2. Falls vorhanden, mit einem eigenen gültigen Passkey anmelden. Andernfalls
   den Zugang zum persönlichen E-Mail-Postfach wiederherstellen lassen.
3. Bei falscher oder leerer E-Mail-Adresse muss der berechtigte Support zuerst
   die Adresse über die bestehende Kontoverwaltung berichtigen. Ohne erreichbares
   Postfach und ohne gültigen Passkey ist keine Anmeldung möglich. Die
   Einrichtungsseite verweist an die Schulverwaltung; eine nicht zustellbare Mail
   darf keinen Erfolg und keine Sitzung erzeugen.
4. Falls nötig, die vorhandene Funktion zum Zurücksetzen der Zwei-Faktor-Anmeldung
   mit Begründung verwenden. Sie entzieht auch bisherigen Sitzungen ihren
   MFA-Nachweis und widerruft gemerkte Geräte. Danach erneut anmelden und den
   Code bestätigen. Die Person bleibt bis dahin ohne privilegierte Sitzung.

**Keine Ausnahme für privilegierte Ziele:** Weder Schule noch Operator können
`force_off` dafür neu setzen. Altwerte bleiben als historische Auditdaten
bestehen, sind aber bei der Rechteprüfung unwirksam. Ablehnungen nutzen das
bestehende Auditprotokoll. Eine zusätzliche Ausnahmeoberfläche entfällt bewusst.
Wiederherstellung stellt den Faktor wieder her, nicht eine Passwort-Ausnahme.

## Bestandsabfrage für befugten Betrieb

Datei: [mfa-pflicht-3841-counts.sql](sql/mfa-pflicht-3841-counts.sql).
Nur über einen freigegebenen administrativen Datenbankzugang ausführen.
Die Abfrage liefert ausschließlich Schul-IDs und Anzahlen, keine Konten,
Adressen, Codes oder Schlüssel. `without_any_second_factor` zählt Konten ohne
E-Mail-MFA und ohne gültigen Passkey. `without_email_address` zeigt Fälle ohne
E-Mail-Weg, nicht die tatsächliche Zustellbarkeit.

```sh
PGOPTIONS='-c default_transaction_read_only=on' \
  psql "$DB_DSN" -v ON_ERROR_STOP=1 \
  -f docs/operations/sql/mfa-pflicht-3841-counts.sql
```

Hier wurde nur die lokale Testdatenbank benutzt. Produktive Zahlen bleiben
Mensch-Schritt. Anzahlen, Release-Version und Ergebnis des Anmelde-/Supporttests
im geschützten Betriebsnachweis festhalten. Keine personenbezogenen Listen im PR.

## Lokale Entwicklung

Die Produktionsregel hat keinen Dev-Schalter. Der dev-only Seed-Adapter schließt
MFA über die echten Enrollment-/Verify-Endpunkte und das lokale Mailpit ab,
wie bisher beim Operator. Ein Seed-Admin erhält dadurch eine bestätigte Sitzung;
spätere manuelle Anmeldungen nutzen den Code aus Mailpit oder einen eigenen
Passkey. Mailpit-Automatik gehört nur in `cmd/devtools`, nicht in die API.

## Vorschlag AVV/TOM

Anhang 1, Ziff. 2 „Zugang“, als konkreten Text prüfen:

> Privilegierte Kundenkonten einschließlich Verwaltungsrechten sowie Zugriffe
> auf Gesundheitsdaten und Personalfinanzdaten verlangen einen zusätzlichen
> Anmeldenachweis unabhängig von der Schuleinstellung. Sitzungen ohne diesen
> Nachweis werden bei der nächsten Erneuerung zur erneuten Anmeldung gezwungen.
> Wiederherstellung stellt einen geeigneten Anmeldeweg wieder her und hebt die
> Pflicht nicht auf. Gemerkte Geräte sind schulgebunden und widerrufbar.

Bis zum produktiven Nachweis als Zielstand kennzeichnen. Das Legal-Repo wird
nicht geändert. Umfang und Verwendung von E-Mail-Codes, Passkeys und gemerkten
Geräten mit den tatsächlich betriebenen Abläufen abgleichen.
