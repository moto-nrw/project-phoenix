# Modulschalter je Schule

**Stand:** 24.10.2026\
**Quelle für:** gebuchte Module einer Schule, Leistungskatalog Schleswig-Holstein 4.16. Entscheidungen: Kommentar vom 24.10.2026 in [#4812](https://github.com/moto-nrw/project-phoenix/issues/4812). Umsetzung der Grundlage: [#4892](https://github.com/moto-nrw/project-phoenix/issues/4892). Folge-Issues: Routen [#4893](https://github.com/moto-nrw/project-phoenix/issues/4893), Navigation [#4894](https://github.com/moto-nrw/project-phoenix/issues/4894), Jobs und Portale [#4895](https://github.com/moto-nrw/project-phoenix/issues/4895), Geräte [#4896](https://github.com/moto-nrw/project-phoenix/issues/4896).

moto Basis hat jede Schule. Dazu bucht eine Schule Zusatzmodule. Je Zusatzmodul gibt es einen Schalter je Schule, den nur das moto-Team im Operator-Portal umlegt. Schul-Admins sehen ihre Module lesend unter Einstellungen, Reiter „Module“.

## Regeln

- **Ebene: je Schule.** Die Schule bleibt der Mandant. Eine Vorgabe je Träger kommt erst mit [ADR 0086](../adr/0086-traeger-werte-liegen-zwischen-standard-und-schule.md) und ist nicht Teil dieser Liste.
- **Bestandsschutz.** Jeder Modulschalter hat die Voreinstellung „gebucht“. Keine Schule verliert mit dem Deployment eine Funktion. Danach gleicht das moto-Team die Schalter an die Verträge an. Auch eine neu angelegte Schule startet mit allen Modulen; das moto-Team bestellt ab, was der Vertrag nicht enthält.
- **Abbestellen löscht keine Daten.** Ein abbestelltes Modul ist sofort ausgeblendet und gesperrt. Die Daten bleiben, bis ihre Löschfrist abläuft ([Löschfristen](loeschfristen.md)). Arbeitszeitnachweise bleiben nach § 16 Abs. 2 ArbZG zwei Jahre erhalten. Einen eigenen Nur-Lesen-Modus gibt es nicht.
- **Export nach Abbestellung.** Braucht eine Schule Daten eines abbestellten Moduls, bucht das moto-Team das Modul vorübergehend, die Schule exportiert, und das moto-Team bestellt wieder ab. Jede Änderung steht im Einstellungs-Audit.
- **Feinsteuerung bleibt.** Schul-Einstellungen wie `operations.meal_plan_enabled` oder `enrollment.enabled` schalten innerhalb eines Moduls. Eine Funktion ist nur sichtbar, wenn das Modul gebucht und die Einstellung an ist.
- **Keine Env-Fallbacks.** Die Schalter sind Einstellungen der Registry (`AccessOperatorManaged`, [ADR 0048](../adr/0048-loeschfristen-traegt-das-moto-team-je-schule-ein.md)). Sie werden über den Settings-Service gelesen.

## Modulliste

Die Liste steht im Code in `backend/modules/settings/records/modules.go`. Das Frontend spiegelt die Schlüssel in `frontend/src/lib/modules.ts`; ein Test hält beide gleich.

| Modul | Schlüssel | Einstellung | Setzt voraus | Gibt es in moto |
|---|---|---|---|---|
| NFC-Anwesenheit | `nfc_anwesenheit` | `module.nfc_anwesenheit` | Basis | ja |
| Mittagessen und Catering | `mittagessen` | `module.mittagessen` | Basis | ja |
| Eltern-App | `eltern_app` | `module.eltern_app` | Basis | ja |
| Online-Anmeldung | `online_anmeldung` | `module.online_anmeldung` | Basis | ja |
| Unterricht am Vormittag | `unterricht_vormittag` | `module.unterricht_vormittag` | Basis | ja |
| Dienstplan | `dienstplan` | `module.dienstplan` | Basis | ja |
| Zeiterfassung | `zeiterfassung` | `module.zeiterfassung` | Basis | ja |
| Personalakte | `personalakte` | `module.personalakte` | Basis | ja |
| Träger-Übersicht | `traeger_uebersicht` | `module.traeger_uebersicht` | Basis | noch nicht |
| Ferienbetreuung | `ferienbetreuung` | `module.ferienbetreuung` | Online-Anmeldung | noch nicht |
| Verträge und Dokumente | `vertraege` | `module.vertraege` | Basis | noch nicht |
| Elternbeiträge und Abrechnung | `elternbeitraege` | `module.elternbeitraege` | Verträge und Dokumente | noch nicht |

Ein Modul, das es noch nicht gibt, hat trotzdem einen Schalter. Er hält fest, was die Schule gebucht hat.

`module.nfc_anwesenheit` hat `attendance.nfc_enabled` abgelöst. Migration 1.15.501 hat für jede Schule den bisherigen Wert übernommen; eine Schule ohne Wert hatte NFC aus und behält es aus.

### Abhängigkeiten

- Ferienbetreuung setzt Online-Anmeldung voraus.
- Elternbeiträge und Abrechnung setzt Verträge und Dokumente voraus.
- Dienstplan setzt nur Basis voraus. Seit #4894 hängen `Dienstplan`, `Mein Dienstplan` und die Dienstarten nicht mehr an `timetable.enabled`, sondern am Modul Dienstplan.
- Der Reiter Arbeitszeitmodell gehört zu Zeiterfassung.
- Stempeln am Kiosk braucht Zeiterfassung und NFC-Anwesenheit. Das ist keine Abhängigkeit zwischen Schaltern: Beide Module bleiben einzeln buchbar, das Stempeln prüft beide (#4896).

Beim Schalten prüft moto beide Richtungen. Wer ein Modul ohne seine Voraussetzung bucht, bekommt `settings.module_requires`. Wer eine Voraussetzung abbestellt, die ein gebuchtes Modul braucht, bekommt `settings.module_required_by`. Zurücksetzen auf die Voreinstellung zählt als Buchen. Die Prüfung nimmt eine Sperre je Schule, damit zwei gleichzeitige Änderungen sich nicht gegenseitig übersehen.

## Leistungskatalog Schleswig-Holstein 4.1 bis 4.15

Katalogpunkt 4.16 fragt, ob sich die Funktionen 4.1 bis 4.15 modular hinzufügen und entfernen lassen ([#2899](https://github.com/moto-nrw/project-phoenix/issues/2899)). Jede Funktion gehört zu genau einem Modul oder zur Basis. Was Basis ist, kann eine Schule nicht abbestellen, weil moto ohne diese Funktion keine OGS führen kann.

| Katalog | Funktion | Zuordnung | Begründung |
|---|---|---|---|
| 4.1 | Digitale Elterneinwilligungen | Eltern-App | Eltern geben Einwilligungen im Elternportal ab. Einwilligungen im Anmeldeformular gehören zu Online-Anmeldung. |
| 4.2 | Kursangebote | Basis | AGs und Aktivitäten sind der Kern der Planung. Ohne sie gibt es keine Anwesenheit in Räumen. |
| 4.3 | Kursanmeldung und Zuweisung | Basis | Das Team weist Kinder Kursen zu. Die Anmeldung durch Eltern gehört zu Eltern-App. |
| 4.4 | Kursleistungsübersicht | Basis | Auswertung der Kurse aus Basis-Daten (gibt es noch nicht, #2891). |
| 4.5 | Anwesenheitsmanagement | Basis | Anwesenheit am Computer und Handy ist Basis. Erfassung mit Armband oder Karte am Tablet ist NFC-Anwesenheit. |
| 4.6 | Betreuungsmanagement | Basis | Kinder, Gruppen und Betreuungszeiten sind die Grundlage jeder anderen Funktion. |
| 4.7 | Abholmanagement | Basis | Abholzeiten und Abholberechtigte pflegt das Team. Änderungen durch Eltern gehören zu Eltern-App. |
| 4.8 | Dienst- und Einsatzplanung | Dienstplan | |
| 4.9 | Kalender | Basis | Termine und Schließtage braucht jede Planung. |
| 4.10 | Meldung bei Betreuungsausfällen | Eltern-App | Die Meldung erreicht Eltern über Elternportal und Push. |
| 4.11 | Dateiablage | Basis | Ablage des Teams ohne eigenes Modul im Modulkatalog. |
| 4.12 | Kommunikationsmodul | Eltern-App | Nachrichten und Elternbriefe an Eltern gehören zu Eltern-App. Der Team-Chat ist Basis. |
| 4.13 | Essen-Anmeldung | Mittagessen und Catering | |
| 4.14 | Abrechnungsfunktion | Elternbeiträge und Abrechnung | Gibt es noch nicht (#2639). |
| 4.15 | Weitergehendes Personalmanagement | Personalakte | Arbeitszeit und Urlaub gehören zu Zeiterfassung. |

## Schnittstelle für Routen, Jobs und Portale

Die Folge-Issues setzen auf diese Teile auf. Neue Module kommen nur über die Modulliste dazu.

**Backend, Paket `modules/settings` (vorhanden):**

- `settings.ModuleKey` und die Konstanten `settings.ModuleNFCAnwesenheit` bis `settings.ModuleElternbeitraege`; die Liste mit Namen und Voraussetzungen in `modules/settings/records` (`Modules()`, `LookupModule`, `ModuleDependents`).
- Je Modul die Einstellung `module.<schlüssel>`. Lesen heißt: `ResolveBool` des Settings-Service für die Schule der Anfrage, `ResolveBoolForTenant` außerhalb der Tenant-Middleware. Beide gehen über den Request-Cache, nie über eine Umgebungsvariable.
- `settings.ModuleSettingKeys()` für ein gebündeltes Lesen, `settings.BookedModules(readBool)` für die gebuchten Module aus einem schon geladenen Satz Einstellungen (so liest die Mandanten-Auflösung).

**Backend, Helfer `ModuleBooking` (kommt mit #4893):**

Das Deadcode-Gate (`scripts/check-deadcode.sh`) lässt keine Produktionsfunktion zu, die nur Tests erreichen. Der Helfer entsteht deshalb im selben PR wie sein erster Aufrufer, der Routen-Sperre. Die Form ist festgelegt; ein fertiger Entwurf mit Tests steht im ersten Commit von #4892 (`backend/modules/settings/modules.go`, `modules_test.go`):

- `settings.NewModuleBooking(reader)`; `reader` ist der Settings-Service (`ResolveBool`, `ResolveBoolForTenant`).
- `IsBooked(ctx, module)` und `Require(ctx, tenantID, modules...)` für Routen in der Tenant-Middleware (#4893). `tenantID` benennt die Schule nur im Fehler.
- `IsBookedForTenant(ctx, tenantID, module)` und `RequireForTenant(...)` außerhalb der Middleware: Scheduler, Login und Refresh der Portale, Geräte (#4895, #4896).
- `Require` liefert `*settings.ModuleNotBookedError{TenantID, Module}` für das erste fehlende Modul. Daraus macht #4893 den Fehlercode für nicht gebuchte Module mit dem Modulschlüssel im Fehler-Umschlag. Ein Lesefehler bleibt ein Fehler und wird nie als „gebucht“ oder „nicht gebucht“ gewertet. Ein unbekannter Schlüssel ist ein Fehler.

**Frontend:**

- `booked_modules` in `/auth/tenant/resolve`: die gebuchten Schlüssel in Katalogreihenfolge, für jedes Teammitglied ohne `config:read`.
- `frontend/src/lib/modules.ts`: Typ `ModuleKey`, `MODULE_KEYS`, `moduleSettingKey`, `parseBookedModules`. Unbekannte Schlüssel fallen weg, eine fehlende Liste bucht nichts.
- `useBookedModules()` und `useModuleBooked(module)` aus `~/lib/tenant-context`. Ohne Mandantendaten ist nichts gebucht. `useNFCEnabled()` folgt dem Modul NFC-Anwesenheit.
- Eine Änderung eines Modulschalters leert den Cache der Mandantendaten sofort (`TENANT_RESOLVE_AFFECTING_KEYS`).

## Navigation, Kacheln und erklärender Zustand (#4894)

Das Modul-Register `frontend/src/lib/module-register.ts` ordnet jede Seite der Mitarbeiter-Navigation, jeden Bereich der Seitenleiste, jeden Baustein der Startseite und jeden Reiter der Personalakte einem Modul oder der Basis zu. Seitenleiste, mobile Navigation, Startseite, Datenverwaltung und die Personalakte lesen nur dieses Register; `module-register.test.ts` schlägt an, wenn eine Seite oder ein Baustein fehlt.

- **Darstellung C.** Wer die Schule leitet (Adminzuschnitt oder `config:manage`), sieht den Eintrag eines nicht gebuchten Moduls mit Schloss. Der Eintrag führt auf die Seite. Alle anderen sehen ihn nicht. Die Reiter unten in der mobilen Navigation zeigen nur offene Seiten; ein Eintrag mit Schloss steht im Mehr-Menü.
- **Feinsteuerung.** Schul-Einstellungen wie `operations.meal_plan_enabled` oder `timetable.enabled` gelten nur bei gebuchtem Modul. Bei nicht gebuchtem Modul sieht die Leitung das Schloss, auch wenn die Einstellung aus ist.
- **Startseite und Reiter.** Bausteine und Reiter der Personalakte eines nicht gebuchten Moduls fehlen für alle, ohne Schloss: eine Kachel ohne Daten belegt nur Platz, und die Leitung sieht das Modul in der Navigation und unter Einstellungen.
- **Erklärender Zustand.** `ModuleRouteGate` im geschützten Layout zeigt auf jeder Seite eines nicht gebuchten Moduls statt des Inhalts, zu welchem Modul sie gehört und wer handelt: die Leitung das moto-Team, alle anderen ihre Leitung. Lehnt eine Anfrage mit `module.not_booked` ab, zeigen `TenantPage`, `LoadErrorAlert` und `FormErrorAlert` denselben Zustand als Hinweis statt als Fehler, und die Mandantendaten laden neu.
- **Bewusste Abweichungen vom Backend.** Aktivitäten (`/activities`, `/database/activities`) gehören im Register zu NFC-Anwesenheit, weil sie vor den Modulen nur mit NFC zu sehen waren. Geräte (`/database/devices`) folgen NFC-Anwesenheit, obwohl das Backend sie erst mit #4896 sperrt.
