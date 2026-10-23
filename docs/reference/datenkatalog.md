# Gemeinsamer Datenkatalog für Löschung und Rückgabe

Stand: 23.10.2026. Entscheidung:
[ADR 0092](../adr/0092-mandanten-datenlebenszyklus-nutzt-einen-gemeinsamen-katalog.md).
Grundlage für #3837 und #3838, Slice 3837-a.

## Stand und Quelle

**Im Code klassifiziert:** 247 Tabellen und neun Upload-Kinds in
`backend/modules/organizationtenancy/data_lifecycle_catalog.json`.
`organizationtenancy.DataLifecycleCatalogSource` stellt denselben Katalog zur
Laufzeit als `embed.FS` bereit. `ReadFile("data_lifecycle_catalog.json")` liefert
eine unabhängige Byte-Kopie; JSON wird in `DataLifecycleCatalog` dekodiert. Kein zweiter
Lösch-/Exportkatalog. Der JSON-Drahtvertrag hat Version 1.

Schemaabgleich mit Migrationen bis 1.15.479: `users.external_caregivers`
ersetzt `users.guests` (1.15.477, ADR 0091). Der Eintrag bleibt `tenant` mit
`export: include` und dem Personen-FK `staff_id` auf
`users.staff_school_memberships.id`. Die Migrationen 1.15.478 (`ag-leitung`)
und 1.15.479 (Gast nur lesend) ändern Datensätze in den bereits erfassten
Tabellen `auth.roles`, `auth.role_permissions` und `auth.permissions`, keine
Tabellen oder Spalten. Die Tabellenzahl und die Spaltenausschlüsse bleiben
unverändert. Globale Systemrollen mit `tenant_id IS NULL` sind kein
Schulbestand; erforderliche Rollenbedeutung muss die spätere Rückgabe
als Kontext liefern, nicht durch ungefilterten Export globaler Rollen.

**Im Code umgesetzt, Slice 3838-a:** `tenant export --school … --uploads … --out …`
liest den freigegebenen Katalogbestand in einer schreibgeschützten
`REPEATABLE READ`-Transaktion über eine eigene administrative Leserolle.
Direkte Tabellen werden per `tenant_id`, Kindtabellen per Katalog-FK-Pfad,
Schul-/Trägerwurzeln und gemeinsame Konten per ausdrücklicher Schulzuordnung
ausgewählt. Originaldateien, JSONL-Tabellen, Modellmetadaten und SHA-256-
Prüfsummen liegen in einem neuen privaten Verzeichnis. Ein Teilfehler
verhindert den Fertig-Marker. Zwei-Mandanten-Tests prüfen jede Katalogtabelle,
alle Datei-Kinds und alle ausgeschlossenen Spalten. Ausführung, Übergabe und
Nachweis: [Runbook](../operations/auskunft-und-rueckgabe.md).

**Noch nicht umgesetzt:** vollständige Mandantenlöschung und Personenauskunft.
Die breiten Ausschlüsse geheimer/undurchsichtiger Container bleiben wirksam;
nicht geheimer Settings-/Nachrichtenkontext und rechtlich erforderlicher
Kontext aus internen Protokollen brauchen weitere geprüfte Projektionen.
Der technische Export des freigegebenen Umfangs ist daher noch kein Nachweis
aller Rückgabe-/Data-Act-Pflichten von #3838.

**Produktiv nachzuweisen:** Einsatz und Wirksamkeit der Verfahren,
Nebensysteme, Sicherungszyklus und erneute Löschung nach Wiederherstellung.
Dieser Slice verwendet nur lokale Schema-Metadaten, keine produktiven Daten.

## Klassifizierung

Jeder Tabellenname hat dieselbe Form `schema.tabelle` wie im bestehenden
`tableRetentions`-Register. Die Register beantworten unterschiedliche Fragen:
reguläre Löschfrist dort, Mandantenzuordnung und Rückgabeumfang hier.

- `tenant`: besitzt `tenant_id`. Nur Zeilen des beauftragten Schul-Mandanten
  auswählen. Vorhandene NULL-Mandanten in System-Outbox/-Auth-Daten sind kein
  Auftrag, globale Daten mitzulöschen oder auszugeben.
- `tenantChild`: besitzt kein `tenant_id`. `parent` nennt Quelltabelle/-spalte
  und Zielspalte eines echten FKs; der Pfad muss eine `tenant`-Tabelle erreichen.
- `account`: globales Konto oder kontobezogener Bestand. Keine Löschung
  allein durch Schulzugehörigkeit. Bestehenden Kontenjob und Rechte anderer
  Schulen/Personen berücksichtigen; Rate-Limit-Daten haben nur einen
  E-Mail-Bezug und folgen ihrem bestehenden Aufräumjob.
- `school`, `organization`, `tenantReference`: Schul-/Trägerwurzeln bzw.
  bewusst nicht FK-gebundene Schulreferenzen. `scopeColumn` und `reason`
  machen die Auswahl ausdrücklich, statt sie aus CASCADE-FKs abzuleiten.
- `platform`, `demo`: begründeter moto-/Demobestand außerhalb des
  Kundenschul-Lebenszyklus, nicht pauschal „keine Personendaten“.

`export` ist `include`, `excludeSecret` oder begründetes `excludeInternal`.
Tabellenausschlüsse gelten für alle Spalten. Bei `include` gelten zusätzlich
`excludeSecretColumns`; `allowedSensitiveColumns` begründet bewusst
freigegebene verdächtige Namen, etwa Postleitzahl oder Inhaltsprüfsumme.
`include` ist **keine Freigabe für Rohdaten, andere Mandanten oder andere
Personen**. Ein späterer Export muss Feldprojektionen und den Auftragszweck
anwenden. Interne Protokolle können erforderlichen Auskunftskontext enthalten;
ein Tabellenausschluss ersetzt dessen rechtliche Prüfung nicht.

`personKeys` enthält direkte FK-Belege zu Kind, Erziehungsberechtigten,
Beschäftigten, Person, Konto oder Eltern-Kind-Beziehung. Der Wächter prüft die
Belege, nicht ihre Vollständigkeit für Art.-15-Auskunft. Indirekte Beziehungen,
Snapshots, JSON/Freitext und Drittpersonenrechte bleiben in #3838 zu bearbeiten.

Datei-Kinds verwenden `tenantDir` (`kind/tenant/name`), `tenantPrefix`
(`kind/tenant_name`) oder `account` (aktuell `avatars/global/name`, über die
Kontoreferenz). Originaldateien, Metadaten und offene Datei-Löschaufträge
gehören zusammen. Ein verwaistes Objekt darf nicht stillschweigend fehlen.

## Pflege und Prüfung

1. Neue/umbenannte/entfernte Tabelle im **Produktionskatalog** ändern, auch
   technische Tabellen. Scope und Rückgabeumfang begründen. Die bisherige
   Fristen-/RLS-Klassifizierung unabhängig weiter pflegen.
2. Neue geheimnisverdächtige Spalten ausschließen oder ihre Freigabe mit
   konkreter Bedeutung begründen. Auch anders benannte Container prüfen:
   `settings`, Zustellpayloads und Credential-JSON können Geheimnisse enthalten.
   Die jetzigen breiten Spaltenausschlüsse sind konservativ; spätere
   key-/feldbezogene Projektionen müssen nicht geheime Einstellungen und
   erforderlichen Nachrichtenkontext verfügbar machen. Sonst ist der Export
   noch nicht vollständig.
3. Neue Dateiablage mit Kind und Layout in denselben Katalog aufnehmen.
   Der Quellenwächter durchsucht Produktions-Go-Dateien nach Upload-Literalen,
   Upload-`Join`-Pfaden, Namespace-Konstanten, `TenantKey`-/Private-Object-Aufrufen
   und `ObjectStore.Kind`. Er ist kein vollständiger Datenflussbeweis:
   neue dynamische Konstruktionen oder andere Sprachen brauchen eine
   Erweiterung samt negativem Test, nicht einen stillen Ausschluss.
4. Prüfen, aus `backend/` über den Repo-Runner:
   `../scripts/run-go-toolchain.sh go test -tags=testhooks ./database/migrations ./modules/organizationtenancy`.
   Die lokale Test-DB initialisiert sich selbst. Wächter:
   `TestEveryTableHasDataLifecycleClassification` und
   `TestEveryUploadKindHasDataLifecycleClassification`.
5. Bei Verfahrensänderungen den Katalogstand im späteren datensparsamen
   Durchführungsnachweis festhalten. Schemaabdeckung allein beweist weder
   Mandantentrennung des Verfahrens noch erfolgreiche Löschung/Rückgabe.

## Nicht im PostgreSQL-/Upload-Katalog

Loki, Sentry, PostHog, journald und Sicherungen sind separate Systeme.
Deren Bestände, Fristen, Zuständigkeiten, Fehlerbehandlung und menschliche
Nachweise müssen die Folge-Slices samt Betrieb konkret abdecken. Es wird
hier kein produktiver Lösch-/Exportnachweis vorgetäuscht. Der interne
Rückgabeablauf ist im oben verlinkten Runbook vorbereitet.
