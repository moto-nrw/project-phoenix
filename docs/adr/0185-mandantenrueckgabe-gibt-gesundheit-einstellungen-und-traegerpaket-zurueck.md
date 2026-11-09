---
status: proposed
---

# Mandantenrückgabe gibt Gesundheitsangaben, Einstellungen und ein Trägerpaket zurück

**Status: Vorgeschlagen.** Vorschlag vom 09.11.2026 zu #3838 und #6328,
setzt [ADR 0184](0184-mandantenrueckgabe-oeffnet-feldverschluesselte-bankdaten.md)
fort und ergänzt [ADR 0097](0097-mandantenrueckgabe-hat-einen-eigenen-leseadapter.md).

## Kontext

Nach ADR 0184 blieben drei Lücken im Rückgabepaket von `tenant export`:

- Gesundheitsangaben (`health_data`, Gesundheitsring `HEALTH_DATA_KEYS`) der
  Betreuungsprofile und der Ferienanmeldungen gingen als Chiffrat raus.
- Die Einstellungen der Schule und des Trägers fehlten ganz: Der Katalog schloss
  `config.setting_values.value` und die Werte der Protokolle aus, weil sie
  Geräte-PIN und SFTP-Passwort enthalten können. AGB Anhang C sagt aber
  „Kundenspezifische Einstellungen … ohne Authentifizierungsgeheimnisse“ zu.
- Ein Träger mit mehreren Schulen bekam nur Schulpakete. Jedes trug den ganzen
  Trägerbestand, aber kein Paket die Konten der Träger-Geschäftsstelle, die an
  keiner Schule Mitglied sind. Die Stammdaten dieser Konten fehlten damit.

## Entscheidung

1. **Gesundheit.** `openSealedColumns` gilt auch für `health_data`. Geöffnet
   wird beim Besitzer der Bindung (AAD): Student Directory View für
   `users.student_care_profiles` (Kontext Schule und Betreuungszeile; Care Plan
   lässt dort versiegeln), Enrollment für Ferienanmeldung, -kind und -gast. Die
   Ferienwerte kommen als der JSON-Text, den sie halten.
   `TenantExportSealedFieldsByTable` leitet jede Tabelle an ihren Besitzer; eine
   Tabelle ohne Besitzer öffnet nicht. Die Root bindet `HEALTH_DATA_KEYS`; fehlt
   er, bricht der Export ab. Der Stolperdraht erkennt `health_data`.
2. **Container.** Der Katalog führt `projectedColumns`: Containerspalte → neue
   Spalte mit der geprüften Projektion. Der Rohcontainer bleibt ausgeschlossen.
   Organization Tenancy definiert den Port `TenantExportProjectedFields`. Settings
   gibt den gespeicherten Wert einer registrierten Definition zurück, `null` für
   `FieldPassword` und für einen Schlüssel, den die Registry nicht mehr kennt.
   `platform.schools.settings` gibt nur `logoUrl` und `loginImageUrl` zurück, die
   einzigen Schlüssel, die der Code liest. Ein Container ohne Besitzer bricht
   den Export ab.
3. **Zustellprotokolle.** `platform.email_suppressions` und
   `platform.email_delivery_events` gehören zum Versandprotokoll der Schule
   (Anhang C) und gehen mit; Diagnosetext und Provider-ID bleiben draußen. Der
   Push-Payload wird nur aus `PushPayload` geschrieben und geht mit. Der
   Mail-Payload bleibt draußen: Sicherheitsmails tragen dort Link oder Code; die
   Mail kommt als Vorlage, Empfänger, Status und verknüpfter Datensatz zurück.
4. **Trägerpaket.** `tenant export --organization <id>` exportiert die
   Trägertabellen, die Stammzeilen der Schulen des Trägers und die Konten aus
   `platform.organization_accounts`, alles in einem Snapshot. Mails, Pushes,
   deren Zustellereignisse und Sitzungen des Trägers tragen `organization_id`
   statt `tenant_id`; das Trägerpaket wählt sie darüber, kein Schulpaket
   enthält sie. Was eine Schule
   hält, steht im Manifest als `schoolPackage` ohne Zeilen und kommt mit dem
   Schulpaket. Schuldateien gehören ins Schulpaket; das Trägerpaket kopiert nur
   die Avatare seiner Konten. `--organization` schließt `--school` und
   `--person` aus. Das Schulpaket bleibt unverändert und trägt weiter den
   Trägerbestand.
5. **Personenauskunft.** Zeilen mit geöffneten Gesundheitsangaben tragen in
   `review.json` die Markierung `special-category-health-data`;
   `requiredChecks` nennt die Prüfung der Art.-9-Daten.

## Folgen

- Ein Trägerwechsel umfasst ein Trägerpaket und je Schule ein Schulpaket. Die
  Trägertabellen stehen in beiden; maßgeblich ist das Trägerpaket desselben
  Auftrags.
- Wechselt nur eine Schule eines Trägers, trägt ihr Schulpaket den
  Trägerbestand aller Schulen. Der Träger entscheidet, was er weitergibt.
- Das Rückgabepaket enthält Gesundheitsangaben im Klartext. Übergabe nur
  verschlüsselt nach dem Runbook.
- Compose-Budgets: `modules/enrollment/compose` +5 (`NewHolidayHealthExportFields`),
  `modules/settings/compose` +3 (`NewExportFields`).
