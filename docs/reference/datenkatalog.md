# Gemeinsamer Datenkatalog für Löschung und Rückgabe

Stand: 23.10.2026. Entscheidung:
[ADR 0092](../adr/0092-mandanten-datenlebenszyklus-nutzt-einen-gemeinsamen-katalog.md).
Grundlage für #3837 und #3838, Slices 3837-a/b.

## Stand und Quelle

**Im Code klassifiziert:** 248 Tabellen und neun Upload-Kinds in
`backend/modules/organizationtenancy/data_lifecycle_catalog.json`.
`organizationtenancy.DataLifecycleCatalogSource` stellt denselben Katalog zur
Laufzeit als `embed.FS` bereit. `ReadFile("data_lifecycle_catalog.json")` liefert
eine unabhängige Byte-Kopie; JSON wird in `DataLifecycleCatalog` dekodiert. Kein zweiter
Lösch-/Exportkatalog. Der JSON-Drahtvertrag hat Version 1.

Ausgangsschema mit Migrationen bis 1.15.479: `users.external_caregivers`
ersetzt `users.guests` (1.15.477, ADR 0091). Der Eintrag bleibt `tenant` mit
`export: include` und dem Personen-FK `staff_id` auf
`users.staff_school_memberships.id`. Die Migrationen 1.15.478 (`ag-leitung`)
und 1.15.479 (Gast nur lesend) ändern Datensätze in den bereits erfassten
Tabellen `auth.roles`, `auth.role_permissions` und `auth.permissions`, keine
Tabellen oder Spalten. Die Tabellenzahl und die Spaltenausschlüsse bleiben
unverändert; 1.15.481 ergänzt anschließend die Nachweistabelle.
Globale Systemrollen mit `tenant_id IS NULL` sind kein
Schulbestand; erforderliche Rollenbedeutung muss die spätere Rückgabe
als Kontext liefern, nicht durch ungefilterten Export globaler Rollen.

**Im Code umgesetzt (Datenbankteil, 3837-b):** Migration 1.15.481 stellt
`platform.erase_school` und die getrennte Nachweistabelle
`platform.school_erasures` bereit. Ein aus diesem Katalog abgeleiteter,
installierter Plan enthält 221 Löschziele mit `tenant`, `tenantChild` oder
`school`. Die explizit gesperrten Rechnungsreferenzen
`platform.billing_key_date_counts` bleiben zehn Jahre ab Kalenderjahresende
erhalten und sind von der Datenrückgabe an Dritte ausgeschlossen (ADR 0092). Ein Wächter gleicht ihn mit dem Produktionskatalog ab.
FK-Reihenfolge, Trigger-Nachläufe, Restprüfung, Mandantentrennung und
Wiederholung nach DB-Fehlern werden lokal geprüft. Der Nachweis bleibt nach
dem DB-Schritt bei `db_done`, nicht `completed`.
[Vertrag und Grenzen](mandantenloeschung-datenbank.md).

**CLI und Dateiphase im Code umgesetzt (3837-c):**
[Runbook](../operations/tenant-erasure.md), Migration `1.15.490`.
**Offen:** produktiver Nachweis, manuelle Nebensystemschritte, Trägerwurzel,
Wiederholung nach Restore (3837-d), Personen-/Gesamtexport (#3838). Die Katalogeinträge selbst sind
keine Berechtigungen und kein produktiver Durchführungsnachweis.

**Produktiv nachzuweisen:** Einsatz und Wirksamkeit der späteren Verfahren,
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
(`kind/tenant_name`) oder `account` (`avatars/global/name` und die Altablage
`avatars/<tenantId>/name`). Avatarverweise stehen in `auth.accounts.avatar` und
`users.profiles.avatar`; offene Löschaufträge in `auth.account_avatar_file_cleanup`
gehören ebenfalls zum Abgleich beider Ablagen. Neue Avatar-Dateien erhalten
`0600`, ihr Zielverzeichnis `0700`; die Altberechtigungen erfordern das Verfahren
in [Private Fotodateien](../operations/private-fotodateien-3840.md).
Auch 3837-c muss beide Ablagen berücksichtigen. Globale Avatare bleiben beim
geteilten Kontenlebenszyklus. Originaldateien, Metadaten und
offene Datei-Löschaufträge gehören zusammen. Ein verwaistes Objekt darf nicht stillschweigend fehlen.

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
   `TestEveryUploadKindHasDataLifecycleClassification` sowie
   `TestSchoolErasurePlanCoversCatalogAndDeleteTriggers` und die
   `TestSchoolErasure*`-Verhaltenstests.
5. Bei Verfahrensänderungen den Katalogstand im späteren datensparsamen
   Durchführungsnachweis festhalten. Schemaabdeckung allein beweist weder
   Mandantentrennung des Verfahrens noch erfolgreiche Löschung/Rückgabe.

## Nicht im PostgreSQL-/Upload-Katalog

Loki, Sentry, PostHog, journald und Sicherungen sind separate Systeme.
Deren Bestände, Fristen, Zuständigkeiten, Fehlerbehandlung und menschliche
Nachweise müssen die Folge-Slices samt Betrieb konkret abdecken. Es wird
hier kein produktiver Lösch-/Exportablauf oder Runbook vorgetäuscht.
