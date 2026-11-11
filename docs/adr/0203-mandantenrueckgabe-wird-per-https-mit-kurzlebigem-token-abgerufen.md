---
status: proposed
---

# Mandantenrückgabe wird per HTTPS mit kurzlebigem Token abgerufen

**Status: Vorgeschlagen.** Vorschlag vom 10.11.2026 zu #3838. Zweiter Teil der
Entscheidung „Data Act Option B“ vom 10.11.2026
([Issue-Kommentar](https://github.com/moto-nrw/project-phoenix/issues/3838#issuecomment-6776609298));
der erste Teil, das Register, ist
[ADR 0201](0201-mandantenrueckgabe-hat-ein-register-und-einen-abrufweg.md).
Ergänzt [ADR 0097](0097-mandantenrueckgabe-hat-einen-eigenen-leseadapter.md).

## Kontext

Art. 30 Abs. 2 Data Act verlangt für Dienste wie moto offene Schnittstellen,
unentgeltlich und dokumentiert, für den Kunden und den vom Kunden benannten
Zielanbieter. Die Rechtsrecherche vom 09.11.2026 hält den betreuten
CLI-Export mit verschlüsselter Übergabe dafür wahrscheinlich nicht für
ausreichend; als Minimum nennt sie einen authentifizierten HTTPS-Abruf des
Pakets mit OpenAPI-Beschreibung, mindestens ab der Wechselanfrage bis zum
Ende des Abrufzeitraums. Die Gründer haben diesen Weg gewählt. Kein
Self-Service-Export, keine allgemeine Export-API, keine Importer, kein SLA.

Das Paket enthält Klartext-IBAN und Gesundheitsangaben. Der Abruf ist deshalb
so sensibel wie das Paket selbst.

## Entscheidung

1. **Wir erstellen und prüfen weiter selbst.** `tenant export` bleibt der
   einzige Weg zum Paket. `tenant return release` nimmt nur ein Paket an,
   dessen `COMPLETE` zur `manifest.json` passt, dessen gelistete Dateien
   Größe und SHA-256 treffen, das keine weitere Datei, keinen Link und kein
   Personen-Prüfpaket (`reviewRequired`) enthält und dessen eigene Zeile in
   `platform.schools` bzw. `platform.organizations` genau die Schule oder den
   Träger nennt, die der Operator angibt. Ein Paket für die falsche Schule
   lässt sich so nicht freigeben. Beim Packen hasht die CLI jede Datei erneut
   und bricht ab, wenn sie sich seit der Prüfung geändert hat oder ein Link
   geworden ist.
2. **Ablage verschlüsselt.** Die CLI packt das Paket als ZIP und verschlüsselt
   es dabei (AES-256-GCM in Blöcken zu 64 KiB, Blockzähler und Endmarke im
   Nonce, Security Runtime) mit einem eigenen Zufallsschlüssel je Paket in
   die Ablage `data-return/` (Volume `phoenix-<env>-data-return`). Der
   Klartext berührt die Platte nicht. Die Datenbank speichert den Schlüssel nur
   eingewickelt unter dem jeweiligen Token und einen HMAC-Prüfwert; das Token
   selbst nur als HMAC. Ablage und Datenbank zusammen öffnen ohne gültiges
   Token kein Paket. Token und Paketschlüssel schreibt die CLI nur in eine neue
   Datei mit Modus 0600 in einem privaten Verzeichnis (`--secrets-out`), nie auf
   die Standardausgabe: die Ausgabe des Containers landet in journald und Loki.
   Mit dem Paketschlüssel stellt der Operator weitere Tokens aus, ohne erneut
   hochzuladen.
3. **Kurzlebige Berechtigung, eine Zeile je Empfänger.**
   `platform.data_return_grants` hält je Token Empfängerart (`customer`,
   `target_provider`) und Name der Empfängerorganisation, Ablauf (höchstens
   30 Tage, nie nach dem Ende des Abrufzeitraums `keep_until`) und
   Downloadbudget (höchstens 20). Widerruf (`tenant return revoke`) und Ablauf
   wirken für neue Anfragen sofort; ein laufender Download prüft alle 8 MiB, ob
   sein Token widerrufen oder bereinigt ist, und bricht dann ab. Nach einem
   Restore der Datenbank widerrufen beide Restore-Wege selbst alle Tokens mit
   Archiv, weil der Restore widerrufene Tokens und Downloadzähler zurückbringt:
   die tägliche Sicherung (`daily_backup.py`) und der Rollback eines Releases
   (`release-backup.sh restore`, automatisch aus `deploy-remote.sh` und
   manuell über `rollback-remote.sh`), jeweils direkt nach `pg_restore`.
   Mangels Operator schreiben sie keinen Eintrag ins Operator-Protokoll,
   sondern halten `revoked_by = 'restore'` an jeder Tokenzeile fest und geben
   `restore_data_return_revoked=<Anzahl>` aus; ein Operator-Widerruf setzt
   `revoked_by = 'operator'`. `tenant return revoke --all` leistet dasselbe
   von Hand. Jedes Token öffnet genau das Archiv seiner Zeile; keine
   Anfrage wählt Schule, Träger oder Datei. Damit gibt es keinen Weg, mit dem
   Token einer Schule das Paket einer anderen zu holen.
4. **HTTPS-Schnittstelle.** `GET /public/data-return/package` nennt Paketart,
   Formatversion, Prüfsummen, Größe, Ablauf und verbleibende Downloads;
   `GET /public/data-return/package/archive` liefert das ZIP mit
   `Content-Length` und `Repr-Digest` (RFC 9530). Das Token kommt nur im
   Header `Authorization: Bearer`, nie in der URL; Sentry filtert den Header.
   Jedes unbrauchbare Token bekommt dieselbe 401. Eigene Ratenbegrenzung je
   Adresse (30 Anfragen pro Minute). `GET /public/data-return/openapi.json`
   beschreibt die Schnittstelle (OpenAPI 3.1). Unentgeltlich, gleich für Kunde
   und Zielanbieter.
5. **Rolle und Nachweis.** Die CLI läuft als NOLOGIN-Gruppe
   `phoenix_data_return` wie der Löschzugang `phoenix_school_erasure`: ohne
   Tabellenrechte, nur mit `EXECUTE` auf `SECURITY DEFINER`-Funktionen der
   Migration 1.15.917. Jede Funktion prüft den aktiven Operator, schreibt und
   ergänzt das Operator-Protokoll in derselben Transaktion (Aktion
   `data_return_*`, je Token bzw. Archiv ein Eintrag, ohne Token, Schlüssel oder
   Personendaten) und sperrt das Archiv per Advisory-Lock, sodass Bereinigung
   und neues Token desselben Archivs nicht ineinanderlaufen. Ein weiteres Token
   übernimmt Abrufzeitraum, Größe und Prüfsummen aus den Zeilen des Archivs,
   nicht vom Aufrufer; Abrufzeitraum (höchstens 366 Tage) und Tokenlaufzeit
   (höchstens 30 Tage) begrenzt auch die Funktion. Die Operator-ID übergibt der
   Aufrufer: Sie ist Zuordnung, keine Anmeldung, wie bei der Schullöschung;
   die Anmeldung ist der dedizierte Login in `phoenix_data_return`. Der
   Webzugang `phoenix_auth` liest nur Tokens, zählt Downloads und schreibt
   Abrufzeilen; `phoenix_admin`, `phoenix_tenant` und `phoenix_demo` haben auf
   die Tabellen keine Rechte, sodass auch eine administrative Einheit des
   Webservers kein Token ändern kann. Jeder Abruf steht
   mit Zeitpunkt, Ergebnis und Bytes in `platform.data_return_downloads`.
6. **Ende.** `tenant return purge` löscht ein Archiv nach dem Abrufzeitraum
   oder sobald alle seine Tokens widerrufen sind, und leert die eingewickelten
   Schlüssel; erst danach entfernt sie die Datei. Dateien der Ablage, die kein
   offenes Token nennt (abgebrochene Freigaben), räumt sie ab einer Stunde
   Alter mit ab. Die Zeilen bleiben als Nachweis der Rückgabe ohne
   Schul-Fremdschlüssel (wie `platform.school_erasures`) und gehen drei Jahre
   nach dem Abrufzeitraum. `tenant erase` verweigert eine Schule oder einen
   Träger, solange ein Paket davon nicht bereinigt ist: Die Löschung erreicht
   die Ablage nicht, das Paket bliebe sonst abrufbar. Der Probelauf verweigert
   ebenso. Ein `BEFORE DELETE`-Trigger auf `platform.schools` und
   `platform.organizations` wiederholt die Prüfung in der Transaktion des
   Löschens, nachdem er die Zeilensperre erhalten hat; die Freigabe sperrt
   dieselbe Zeile mit `FOR KEY SHARE` und verweigert eine Schule oder einen
   Träger, den es nicht mehr gibt oder dessen Löschung begonnen hat (ein
   Löschauftrag der Schule in `platform.school_erasures`, ein gestarteter
   Löschauftrag des Trägers). Ein erfasster Trägerauftrag in der Karenzzeit
   sperrt nicht: Gerade dann braucht der Kunde die Rückgabe. Die Freigabe
   eines Trägerpakets sperrt dazu den offenen Löschauftrag mit `FOR SHARE`,
   bevor sie dessen Start liest; `begin_organization_erasure` zählt die
   unbereinigten Pakete des Trägers, sobald es den Auftrag mit `FOR UPDATE`
   hält, und verweigert den Start mit 55006 (#6665). Freigabe und Löschung
   desselben Subjekts laufen so nie aneinander vorbei.

**Trägerpaket und Schullöschung** (entschieden am 11.11.2026,
[#6644](https://github.com/moto-nrw/project-phoenix/issues/6644)): Ein
offenes Paket des Trägers sperrt die Löschung einer seiner Schulen **nicht**,
weder bis zum Ende des Abrufzeitraums noch bis zur Bereinigung. Es bleibt
abrufbar, bis es bereinigt oder der Träger gelöscht wird; nur die
Trägerlöschung wartet darauf. Gründe:

- Das Paket ist eine Kopie dessen, was dem Träger gehört. Seine Tabellen
  (Verträge, Abrechnung, SEPA, Ferienanmeldungen) haben den Bereich
  `organization` und gehen mit dem Träger oder nach seiner eigenen
  Aufbewahrungsweisung (ADR 0184), nicht mit der Schule
  (`data_lifecycle_catalog.json`). Nach der Schullöschung liegen dieselben
  Unterlagen weiter in der Datenbank; eine Sperre des Pakets schützte nichts,
  was die Löschung entfernt. Was die Schullöschung über den Träger hinaus
  entfernt, steht im Trägerpaket nur als Stammzeile der Schule: Name,
  Anschrift und Kontakt der Einrichtung, keine Daten von Kindern oder
  Familien.
- Verantwortlicher und Kunde dieser Unterlagen ist der Träger, sein Vertrag
  läuft weiter. AVV Ziff. 8.2 (Art. 28 Abs. 3 lit. g DSGVO) knüpft Löschen
  oder Rückgabe an das Ende der Leistung für den jeweiligen Verantwortlichen;
  für den Träger ist sie mit dem Ende einer seiner Schulen nicht beendet. Die
  Aufbewahrungspflichten des Trägers (§ 147 AO, § 257 HGB) laufen ebenfalls
  weiter.
- Eine Sperre ließe nur zwei Wege: die Schullöschung bis zu 366 Tage
  aufschieben, obwohl der Löschauftrag der Schule fällig ist, oder das Paket
  des Trägers widerrufen. Der Widerruf nähme dem Träger den Abruf, den Art. 25
  und 30 Data Act ihm im Abrufzeitraum zusichern, wegen eines Vorgangs, der
  seine Daten nicht berührt.

Verfügbarkeit: Die Schnittstelle besteht dauerhaft; ein Paket ist ab seiner
Freigabe nach der Wechselanfrage bis zum Ende des Abrufzeitraums abrufbar,
Tokens werden in dieser Zeit bei Bedarf neu ausgestellt.

## Folgen

- Jede Freigabe braucht das Paketverzeichnis auf dem Server der Installation
  und einen Einmal-Container mit dem Volume `phoenix-<env>-data-return`.
- Wer ein Token hat, hat das Paket. Tokens gehen nur über einen bestätigten
  zweiten Kanal an die benannte Organisation; die Laufzeit bleibt kurz.
- Die Ablage ist nicht Teil der täglichen Sicherung. Nach einem Restore der
  Datenbank widerruft der Operator alle Tokens, bereinigt und gibt laufende
  Wechsel neu frei; ein fehlendes Archiv beantwortet der Abruf mit 503.
- Der Freigabezugang braucht einen eigenen Login in `phoenix_data_return`,
  den der Restore wie die Löschzugänge sperrt.
- Ob dieser Abrufweg Art. 30 Abs. 2 genügt, bestätigt die anwaltliche Prüfung.

## Verworfen

- **Selbstbedienungs-Export oder allgemeine Daten-API:** nicht Teil der
  Entscheidung vom 10.11.2026.
- **Archiv-Schlüssel aus einem neuen Server-Schlüsselring:** bräuchte neue
  Geheimnisse in allen Umgebungen und läge neben der Ablage; der Schlüssel
  unter dem Token schützt besser und braucht keine neue Konfiguration.
- **Geheimnisse auf der Standardausgabe oder ein eigener Compose-Dienst ohne
  Logging:** die Ausgabe von `server` geht nach journald; ein zweiter Dienst
  schützt nur, solange niemand versehentlich `server` nimmt. Die Datei mit
  0600 hängt nicht an der Compose-Konfiguration.
- **Löschung räumt die Ablage mit ab:** die Löschung bearbeitet die
  Upload-Wurzel; eine zweite Ablage dort mitzulöschen bräuchte einen
  weiteren Mount im Löschlauf. Die Sperre ist einfacher und hält den Widerruf
  im Operator-Protokoll.
- **Token in der URL (Link zum Klicken):** landet in Logs, Verläufen und
  Mail-Scannern.
- **Ablage im Upload-Volume:** das Volume läuft durch Sicherung, Export und
  Löschung der Schulen; das Paket ist kein Schuldatensatz.
