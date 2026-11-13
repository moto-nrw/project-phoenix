---
status: accepted
---

# Abholzeit zurücksetzen liest den heutigen Tag

Entscheidung vom 2026-11-03 für [#5643](https://github.com/moto-ogs/project-phoenix/issues/5643).

## Kontext

Eine von Hand eingetragene Abholzeit ist eine Zeile je Wochentag ohne
Gültigkeitszeitraum. `ResetStudentPickupDayToOffering` löschte sie, sobald am
gewählten Datum eine gebuchte Abholzeit galt. An einem künftigen Tag nach dem
Stichtag einer Buchungsänderung verlor die Schule damit die eingetragene Zeit
auch an den Tagen davor, an denen noch die alte Buchung galt.

Der Hinweis nach der Freigabe (#5618) prüft dieselbe Frage schon mit
`pickupResetKeepsEarlierDays`: Er bietet das Zurücksetzen erst an, wenn kein
Tag ab heute bis zum Stichtag die eingetragene Zeit noch braucht. Dafür liest
er den heutigen Tag über `BookingMaterializationDependencies.Today`.

## Entscheidung

Das Zurücksetzen prüft dasselbe selbst. Vom nächsten Termin des Wochentags ab
heute bis vor das gewählte Datum muss die Buchung an jedem Tag die
eingetragene Zeit setzen. Sonst lehnt der Befehl mit
`careplan.PickupResetStillNeededError` ab und ändert nichts. Die Antwort
`students.pickup_reset_still_needed` nennt den letzten Tag, der die Zeit noch
braucht, und den ersten Tag, an dem das Zurücksetzen geht.

Handzeiten bekommen keinen Gültigkeitszeitraum. Das wäre eine Migration und
eine Änderung aller Leser der Abholzeiten; die Prüfung schützt die früheren
Tage ohne sie.

Der Befehl liest dafür dieselbe Uhr wie die bereits freigegebene
Buchungsänderung. Die Binding-Flow-Prüfung kann ihren Funktionswert im
Test-Hook-Pfad nicht verfolgen und meldet je Bindung
`unresolved:callable:.../api.now` und
`unresolved:function:.../BookingMaterializationDependencies.Today`. Diese
Entscheidung gibt genau diese Diagnosen für `ResetStudentPickupDayToOffering`
frei:

| Port | Bindung |
| --- | --- |
| `careplan.BookingMaterializationCapability.ResetStudentPickupDayToOffering` | `application.BookingMaterialization.ResetStudentPickupDayToOffering` |
| `careplan.BookingMaterializationCapability.ResetStudentPickupDayToOffering` | `application.bookingCommands.ResetStudentPickupDayToOffering` |
| `careplan.CareOfferingCapability.ResetStudentPickupDayToOffering` | `application.CareOfferings.ResetStudentPickupDayToOffering` |
| `careplan.OfferingPickupTimes.ResetStudentPickupDayToOffering` | `careplan.CareOfferingCapability.ResetStudentPickupDayToOffering` |

Sie beschreiben keine Schreib-, Sperr- oder Berechtigungseffekte. Der Befehl
liest nur das Datum. Ein Integrationstest deckt die Ablehnung und den
erlaubten Fall ab.

Nach dem Einchecken dieser Entscheidung darf sie keine weiteren Diagnosen
freigeben.
