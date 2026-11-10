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
   lässt sich so nicht freigeben.
2. **Ablage verschlüsselt.** Die CLI packt das Paket als ZIP und verschlüsselt
   es dabei (AES-256-GCM in Blöcken zu 64 KiB, Blockzähler und Endmarke im
   Nonce, Security Runtime) mit einem eigenen Zufallsschlüssel je Paket in
   die Ablage `data-return/` (Volume `phoenix-<env>-data-return`). Der
   Klartext berührt die Platte nicht. Die Datenbank speichert den Schlüssel nur
   eingewickelt unter dem jeweiligen Token und einen HMAC-Prüfwert; das Token
   selbst nur als HMAC. Ablage und Datenbank zusammen öffnen ohne gültiges
   Token kein Paket. Den Paketschlüssel bekommt der Operator einmal angezeigt;
   damit stellt er weitere Tokens aus, ohne erneut hochzuladen.
3. **Kurzlebige Berechtigung, eine Zeile je Empfänger.**
   `platform.data_return_grants` hält je Token Empfängerart (`customer`,
   `target_provider`) und Name der Empfängerorganisation, Ablauf (höchstens
   30 Tage, nie nach dem Ende des Abrufzeitraums `keep_until`) und
   Downloadbudget (höchstens 20). Widerruf (`tenant return revoke`) und Ablauf
   wirken sofort. Jedes Token öffnet genau das Archiv seiner Zeile; keine
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
5. **Nachweis.** Freigabe, weiteres Token, Widerruf, Auflistung und Bereinigung
   prüfen den aktiven Operator und stehen im Operator-Protokoll (Aktion
   `data_return_*`, ohne Token, Schlüssel oder Personendaten). Jeder Abruf steht
   mit Zeitpunkt, Ergebnis und Bytes in `platform.data_return_downloads`.
6. **Ende.** `tenant return purge` löscht ein Archiv nach dem Abrufzeitraum
   oder sobald alle seine Tokens widerrufen sind, und leert die eingewickelten
   Schlüssel. Die Zeilen bleiben als Nachweis der Rückgabe ohne
   Schul-Fremdschlüssel (wie `platform.school_erasures`) und gehen drei Jahre
   nach dem Abrufzeitraum. Vor einer Löschung von Schule oder Träger wird
   bereinigt.

Verfügbarkeit: Die Schnittstelle besteht dauerhaft; ein Paket ist ab seiner
Freigabe nach der Wechselanfrage bis zum Ende des Abrufzeitraums abrufbar,
Tokens werden in dieser Zeit bei Bedarf neu ausgestellt.

## Folgen

- Jede Freigabe braucht das Paketverzeichnis auf dem Server der Installation
  und einen Einmal-Container mit dem Volume `phoenix-<env>-data-return`.
- Wer ein Token hat, hat das Paket. Tokens gehen nur über einen bestätigten
  zweiten Kanal an die benannte Organisation; die Laufzeit bleibt kurz.
- Die Ablage ist nicht Teil der täglichen Sicherung. Nach einem Restore fehlt
  ein Archiv; der Abruf antwortet 503, der Operator gibt neu frei.
- Ob dieser Abrufweg Art. 30 Abs. 2 genügt, bestätigt die anwaltliche Prüfung.

## Verworfen

- **Selbstbedienungs-Export oder allgemeine Daten-API:** nicht Teil der
  Entscheidung vom 10.11.2026.
- **Archiv-Schlüssel aus einem neuen Server-Schlüsselring:** bräuchte neue
  Geheimnisse in allen Umgebungen und läge neben der Ablage; der Schlüssel
  unter dem Token schützt besser und braucht keine neue Konfiguration.
- **Token in der URL (Link zum Klicken):** landet in Logs, Verläufen und
  Mail-Scannern.
- **Ablage im Upload-Volume:** das Volume läuft durch Sicherung, Export und
  Löschung der Schulen; das Paket ist kein Schuldatensatz.
