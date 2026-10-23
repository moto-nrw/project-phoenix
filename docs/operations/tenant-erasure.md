# Mandantenlöschung: internes Verfahren (3837-c)

Stand: 23.10.2026. Refs #3837, Teil von #3846.

## Stand und Grenzen

**Im Code umgesetzt:** `tenant erase`, schreibfreier Probelauf mit Anzahlen
je DB-Tabelle und Dateiart, Migration `1.15.490`, dauerhaftes Dateiinventar
als Anzahlen, DB-Löschung über `platform.erase_school`, wiederaufnehmbare
Dateiphase und JSON-Nachweis. `completed` bedeutet ausschließlich: DB- und
lokale Dateiphase dieses Schulauftrags fertig. Es bestätigt weder
Nebensysteme noch Backups oder eine rechtliche Gesamtfreigabe.

**Produktiv nachzuweisen:** Migration, Rollenvergabe, richtiges Upload-Volume,
Stillstand aller Schreiber, erfolgreiche Durchführung, Nebensysteme und
Fristausführung. Lokale Tests sind kein produktiver Nachweis.

**Offen:** endgültige Löschung der Trägerwurzel. Der DB-Vertrag erlaubt nur
Schullöschungen. `--organization` bearbeitet deren Schulaufträge, lässt die
Wurzel bestehen und endet ausdrücklich mit Fehlerstatus für den offenen
Trägerauftrag. Keine Träger-Gesamtlöschung bestätigen. Restore-Verfahren:
Slice 3837-d. Nebensystemfristen und deren produktive Nachweise: 3837-f.

## 1. Auftrag und Wartungsfenster freigeben (Mensch)

1. Im zugriffsbeschränkten internen Vorgang Weisung und Berechtigung,
   Schul-/Träger-ID, Umfang, Freigabe, Rückgabe und Zeitpunkt prüfen.
   Data-Act-Abruffrist und gesetzliche Aufbewahrung getrennt prüfen.
   Keine pauschale zusätzliche 30-Tage-Wartefrist; kein vorheriges Soft Delete.
   Offene Rückgabe, Abruffrist oder Aufbewahrungsentscheidung sperrt den Lauf.
2. Gesperrte Abrechnungsreferenzen bleiben nach dem
   [DB-Vertrag](../reference/mandantenloeschung-datenbank.md) erhalten.
   Weitere Ausnahmen vorab klären, nicht durch manuelle Kopien umgehen.
3. [DB-Vorprüfung](sql/mandantenloeschung-vorpruefung-3837.sql) und
   [Deploy-Vorbedingungen](../reference/mandantenloeschung-datenbank.md#mensch-schritte-vor-deploy-und-erster-löschung)
   abarbeiten. Migration und passendes Backend gemeinsam bereitstellen.
4. Dedizierten internen CLI-Zugang mit `NOINHERIT` und Mitgliedschaft in
   `phoenix_school_erasure` einrichten, **ohne** Tabellenrechte, Superuser,
   `BYPASSRLS` oder Mitgliedschaft in Web-/Abrechnungsrollen. Der bestehende
   `DB_DSN` muss für diesen Lauf diesen Zugang verwenden. Zugangsdaten nur
   über den freigegebenen Secret-Prozess bereitstellen, nie in Argumente,
   Shell-History, Ausgabe oder dieses Runbook schreiben. Kein neues Env-Schema.
5. Wartungsfenster für die gesamte Installation vereinbaren. Alle API-,
   Worker-, Scheduler-, Import- und Upload-Schreiber anhalten und laufende
   Uploads abwarten. DB-Locks können Schreibzugriffe **anderer Schulen**
   blockieren. Bis Abschluss der Dateiphase geschlossen halten. Upload-Volume
   und alle verwendeten Upload-Wurzeln inventarisieren; der Befehl bearbeitet
   genau eine Wurzel. Bei mehreren Wurzeln keinen Gesamtabschluss freigeben,
   bevor das Betriebsverfahren alle Ablagen zusammengeführt/geprüft hat.

Die Operator-ID ist eine interne ID eines aktiven Operators. Sie ist keine
technische Anmeldung und ersetzt die Auftragsprüfung nicht. `--order` enthält
nur eine Vorgangsnummer (3–40 Großbuchstaben/Ziffern/Bindestriche), keine
Namen, Mailadressen, Inhalte oder Geheimnisse. Keine Roh-DB-Fehler protokollieren.

## 2. Probelauf und Ausführung

Die folgenden Aufrufe führt eine berechtigte Person im Server-Container aus.
Sie enthalten keine Hostnamen oder Zugangsdaten. `SCHOOL_ID`, `OPERATOR_ID`
und `ORDER_REF` mit den freigegebenen **internen** Werten belegen; keine
Personendaten einsetzen. `public/uploads` ist die übliche Ablage relativ zum
Arbeitsverzeichnis des Server-Containers. Vorher am Volume prüfen, nicht
ungeprüft übernehmen. Eine fehlende Upload-Wurzel führt zum Abbruch; ein
falsches, aber leeres Verzeichnis ist technisch nicht als falsch erkennbar.

```sh
# Keine DB-/Dateiänderungen, kein neuer Auftrag:
docker compose run --rm --no-deps server /app/main tenant erase \
  --school "$SCHOOL_ID" --operator "$OPERATOR_ID" --order "$ORDER_REF" \
  --uploads-root public/uploads --dry-run \
  --lock-timeout 5s --statement-timeout 5m

# Erst nach dokumentierter Freigabe und gestoppten Schreibern:
docker compose run --rm --no-deps server /app/main tenant erase \
  --school "$SCHOOL_ID" --operator "$OPERATOR_ID" --order "$ORDER_REF" \
  --uploads-root public/uploads --confirm "school:$SCHOOL_ID" \
  --prerequisites-checked --writers-stopped \
  --lock-timeout 5s --statement-timeout 5m
```

Die Aufrufe verwenden die Binary des Produktionsimages. Im Entwicklungsimage
`/app/main` durch `go run .` ersetzen. Der bestehende Dienst `server` und alle
anderen Schreiber bleiben gestoppt. Der Einmal-Container muss identisches
Image, Konfiguration und **dasselbe Upload-Volume** verwenden. Vorher den
Mount prüfen, keine neue Upload-Ablage erzeugen. Der DB-Pool benötigt
mindestens zwei Verbindungen (Koordinationssperre und Phasen-Commit).
Lokal gilt `scripts/dev-native.sh backend go run . tenant ...`; dieses
Runbook ist keine Erlaubnis für Agenten, Produktionssysteme zu bedienen.

Vor Ausführung die Probelaufmengen mit dem freigegebenen Bestand abgleichen.
Der Probelauf ist eine Momentaufnahme und keine Sperre. Alle genannten
Dateiarten müssen sichtbar sein, auch mit Anzahl 0. Globale
`avatars/global/` bleiben unberührt; Altbestand `avatars/<school-id>/`,
verwaiste Dateien und Cleanup-Reste in Schul-Namespaces werden entfernt.
Unbekannte Dateiarten oder unsichere Pfade/Symlinks stoppen das Verfahren.

Für einen Träger `--school` durch `--organization "$ORGANIZATION_ID"` ersetzen
und `--confirm "organization:$ORGANIZATION_ID"` verwenden. Vorher jede
Schule des Trägers prüfen; es gibt keine automatische Berechtigung durch
bloße Trägerzuordnung. Derselbe Vorgang umfasst bei Wiederaufnahme auch
bereits entfernte Schulen mit derselben Referenz/Operator-ID. Die
Trägerwurzel bleibt offen und der Aufruf endet deshalb ungleich 0.

## 3. Abbruch und Wiederaufnahme

1. Exit-Code und jede Schulquittung prüfen. Erfolg eines SQL-Aufrufs oder
   einzelner Schulen ist kein Gesamtabschluss. Fehlertexte sind datensparsam;
   SQLSTATE steht, soweit commitbar, im Nachweis. Bei Prozess-/Verbindungsverlust
   kann nur der letzte separat committete Zustand erhalten bleiben.
2. Bei DB-Fehlern bleibt die Löschung atomar zurückgerollt. Lock-/Statement-
   Timeout ist ein Abbruch, kein Erfolg. Ursache im Wartungsfenster beheben,
   nicht blind Zeitlimits erhöhen. Alle Schreiber weiter angehalten lassen.
3. Mit exakt derselben Schule, Referenz und Operator-ID erneut ausführen.
   Keine neue Auftrags-ID erzeugen. Vor DB-Löschung werden Dateimengen
   dauerhaft gespeichert; es werden keine Dateinamen oder Inhalte gespeichert.
   Der deterministische Namespace bleibt nach Metadatenverlust auffindbar.
4. Nach `db_done_at` ist der DB-Schritt ein No-op. Die Dateiphase entfernt
   noch vorhandene Dateien idempotent und prüft danach alle Namespaces erneut.
   Ein Dateifehler setzt, soweit erreichbar, `failed`; ohne beide Phasen gibt
   es kein `completed`. Abgeschlossene Wiederholung liefert denselben Nachweis.
5. Nie mit Roh-SQL einen Abschluss setzen. Fehlende Berechtigungen,
   Symlinks oder falsches Volume beheben und erneut ausführen. Bereits vor
   Einführung des CLI nur per DB gelöschte Aufträge ohne Dateiinventar können
   nicht automatisch abgeschlossen werden; gesondert prüfen und nachweisen.

## 4. Nachweis exportieren und Nebensysteme abarbeiten (Mensch)

```sh
umask 077
docker compose run --rm --no-deps -T server /app/main tenant receipt --id "$ERASURE_ID" \
  > tenant-erasure-receipt.json
```

Die JSON-Quittung enthält interne IDs, Vorgangsreferenz, Verfahrens-/Katalog-
version, Phasen, Zeitpunkte, Anzahlen und gegebenenfalls SQLSTATE, keine
Dateinamen oder gelöschten Inhalte. Zugriffsbeschränkt im Vorgang ablegen.
Für den Kunden die eigene Schulquittung und den gesonderten Abschluss der
Mensch-Schritte bereitstellen; keine fremden Schulen/Operator-Stammdaten
beilegen. CLI-Ausgabe kann zusätzliche Konfigurationshinweise vor JSON
enthalten; vor Weitergabe nur das JSON-Objekt übernehmen und validieren.

1. **Sentry:** betroffene Projekte/Environments und Aufbewahrungsfristen
   feststellen. Im freigegebenen Backend nach Schul-ID/Tags suchen, ohne
   Rohereignisse zu exportieren. Löschbare zuordenbare Events/Anhänge nach
   Anbieterfunktion entfernen und deren Durchführung/Frist dokumentieren.
   Globale Events mit Drittbezug nicht pauschal löschen. Fehlende selektive
   Löschbarkeit als offene Arbeit mit Verantwortlichem und Frist festhalten.
2. **PostHog:** Schulgruppen-/Mandanteneigenschaften und zuordenbare Profile,
   Events, Aufzeichnungen und Exporte prüfen. Selektive Löschung nach dem
   freigegebenen Anbieterweg durchführen; gemeinsame Identitäten nicht ohne
   Prüfung anderer Schulen entfernen. Keine neue Personenliste erzeugen.
3. **Loki/journald/Hostlogs:** Stream- und Indexfristen, Archive und Exporte
   erfassen. Wo sichere selektive Löschung freigegeben ist, durchführen;
   sonst Zugriff sperren, keine reguläre Verarbeitung und dokumentierten
   Ablauf kontrollieren. Fristen/Nachweise aus 3837-f ergänzen, nicht erfinden.
4. **Weitere Systeme:** Schulbezogene Zustell-/Exportziele, lokale temporäre
   Dateien, Support-Kopien und Auftragsverarbeiter im tatsächlichen Inventar
   prüfen. Der CLI löscht weder fremde SFTP-Ziele noch bereits versandte
   E-Mails. Löschung/Rückgabe beim zuständigen Betreiber anfordern und belegen.
5. **Backups:** betroffene Sicherungsstände und dokumentierten Zyklus im
   Löschregister erfassen, regulären Zugriff/Verarbeitung ausschließen,
   planmäßiges Auslaufen prüfen. Unabhängiges Löschregister vorhalten.
   Nach Restore nach 3837-d alle Löschaufträge vor Wiederaufnahme regulärer
   Verarbeitung wiederholen, auch wenn die restaurierte DB sie nicht kennt.
   Das Restore-Runbook ist gesondert zu liefern und produktiv zu erproben.

Ohne diese Schritte keinen vollständigen Kundenabschluss bestätigen. Die
CLI-Quittung bleibt ein technischer Teilnachweis.

## 5. Fristbereinigung und lokale Prüfung

Regelmäßige, überwachte Betriebsaufgabe einrichten und deren Ausführung
belegen (CLI richtet keinen Scheduler ein):

- Mit `phoenix_school_erasure`: `SELECT platform.delete_expired_school_erasures();`
  Abgeschlossene Nachweise laufen nach drei Jahren aus; offene Aufträge nicht.
- Mit der getrennten Rolle `phoenix_billing_retention`:
  `SELECT platform.delete_expired_billing_key_date_counts();`
  Gesperrte Abrechnungsreferenzen nach zehn Jahren ab Kalenderjahresende.
  Keine dieser Zusatzrechte an Web- oder Schullogins vergeben.

Die Wirksamkeit lokal mit synthetischen Daten prüfen:

```sh
cd backend
../scripts/run-go-toolchain.sh go test -tags=testhooks \
  ./cmd -run TestErasureCLI -count=1
../scripts/run-go-toolchain.sh go test -tags=testhooks \
  ./modules/organizationtenancy/internal/application ./modules/delivery/objects \
  -run 'TestErasure|TestTenantErasure' -count=1
```

Die Tests prüfen Probelauf ohne Änderungen, Tabellenmengen zweier Schulen,
DB-Rollback, Teilausfall nach Dateilöschung, Wiederaufnahme, fremde und globale
Dateien, Symlinks, Dateikatalog-Wächter und identischen Wiederholungsnachweis.
Die vorhandenen `TestSchoolErasure*` prüfen ergänzend das breite DB-Schema,
FK-/Trigger-Nachläufe und die unveränderten Inhalte der anderen Schule.
