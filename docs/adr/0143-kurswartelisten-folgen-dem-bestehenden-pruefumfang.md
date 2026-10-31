---
status: accepted
---

# Kurswartelisten folgen dem bestehenden Prüfumfang

Entscheidung vom 2026-10-31 für [#4767](https://github.com/moto-nrw/project-phoenix/issues/4767).

## Kontext

Die Teamansicht der Kurswartelisten liest offene Angebotsänderungsanfragen.
Ohne den Prüfumfang der Elternanfragen könnte eine auf einzelne Gruppen
beschränkte Fachkraft Namen und Wartelistenplätze anderer Kinder sehen.

Die bestehenden Angebotsentscheidungen lösen den Prüfumfang über
`OfferingChangeDependencies.Scope` auf. Der Resolver liest die berechtigten
Gruppen aus dem verifizierten Request-Kontext und ruft die Identity-Policy auf.
Er ist eine Funktionsabhängigkeit, damit die bestehenden Test-Hooks ihren
Prüfumfang einsetzen können. Die Binding-Flow-Prüfung kann diesen
Funktionswert im Test-Hook-Pfad nicht vollständig verfolgen und meldet daher
`unresolved:callable:.../api.s`.

## Entscheidung

`CourseWaitlists` verwendet vor der Rangbildung denselben Resolver und
dieselbe Filterregel wie die vorhandene Anfragenliste. Nicht mehr aktive,
beendete oder außerhalb des Prüfumfangs liegende Kinder erreichen weder die
Wartelistenberechnung noch die Namensauflösung.

Die fünf neuen, funktionswertbedingten Diagnosen sind damit bewusst auf die
neue Leseoperation beschränkt:

| Port | Bindung |
| --- | --- |
| `careplan.CareOfferingCapability.CourseWaitlists` | `application.CareOfferings.CourseWaitlists` |
| `careplan.OfferingChangeCapability.CourseWaitlists` | `application.OfferingChanges.CourseWaitlists` |
| `careplan.OfferingChangeCapability.CourseWaitlists` | `roots/owners.offeringChangeLifecycle.CourseWaitlists` |
| `careplan.OfferingChangeRequests.CourseWaitlists` | `careplan.CareOfferingCapability.CourseWaitlists` |
| `careplan.OfferingChangeRequests.CourseWaitlists` | `careplan.OfferingChangeCapability.CourseWaitlists` |

Sie beschreiben keine Schreib-, Sperr- oder zusätzlichen Berechtigungseffekte.
Die aufgelösten Aufrufe an `ParentRequestReviews.ReviewScope` und
`PermissionsFromCtx` bleiben lesende Effekte. Der Integrationstest deckt eine
auf Gruppen beschränkte Fachkraft mit sichtbarer und unsichtbarer Anfrage ab.

Nach dem Einchecken dieser Entscheidung darf sie keine weiteren Diagnosen
freigeben.
