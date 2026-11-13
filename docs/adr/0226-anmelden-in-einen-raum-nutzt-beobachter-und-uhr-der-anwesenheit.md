---
status: proposed
---

# Anmelden in einen Raum nutzt Beobachter und Uhr der Anwesenheit

Entscheidung vom 2026-11-13 für
[#6953](https://github.com/moto-ogs/project-phoenix/issues/6953). Sie genehmigt
sechs neue Command-Effect-Diagnosen. Sie fügt keinen Owner, keine Policy-Regel,
keine Sperre und keine Laufzeitausnahme hinzu.

## Kontext

Fällt ein Terminal aus, meldet das Team Kinder in der App an. Damit die App dem
Terminal gleichwertig ist, meldet sie ein Kind in einem Schritt an und setzt es
in einen Raum oder eine Aktivität (#6953). Student Presence bietet dafür zwei
neue Befehle an:

- `CheckInStudentsToSessionAuthorized` für eine laufende Sitzung mit Aufsicht,
- `CheckInStudentsToOpenRoomSessionAuthorized` für die Raumsitzung eines
  freigegebenen Raums; der Workflow `open-room-move` ruft ihn über seinen
  Port `Moves.CheckInIntoRoomSession` auf.

Beide führen die bestehende Sammel-Anmeldung (`processSchoolCheckinBatch`) und
danach die bestehende Verschiebung (`moveStudentsToActiveGroupLocked`) in einer
Einheit aus. Die Sammel-Anmeldung liest die Uhr über `ServiceDependencies.Now`,
die Verschiebung meldet `PrepareVisitWrites` an den Beobachter
`application.Service.observe`. Der Binding-Flow-Scan aus
[ADR 0098](0098-command-effect-binding-flow-and-platform-protocols.md) kann
diese Funktionswerte nicht verfolgen. Jeder Port, der sie erreicht, verzeichnet
deshalb `unresolved:function:…`. Für `MoveStudentsToSession` und
`MoveStudentsToActiveGroupAuthorized` sind dieselben Diagnosen schon genehmigt
([ADR 0138](0138-existing-visits-lock-before-sessions-and-room-rows.md) für den
Beobachter, ADR 0098 für die Uhr). Eine ADR, die schon in der Basis liegt, kann
keine neue Diagnose genehmigen.

| Port | Ziel | Digest |
|---|---|---|
| `inbound/presence.PresenceOperations.CheckInStudentsToSessionAuthorized` | `application.Service.observe` | `afade5fb4199e0ad403df633e85b1607` |
| `inbound/presence.PresenceOperations.CheckInStudentsToSessionAuthorized` | `presence.ServiceDependencies.Now` | `8776a410a33ff9b57a6a029acb646f74` |
| `presence.Service.CheckInStudentsToSessionAuthorized` | `application.Service.observe` | `afade5fb4199e0ad403df633e85b1607` |
| `presence.Service.CheckInStudentsToSessionAuthorized` | `presence.ServiceDependencies.Now` | `8776a410a33ff9b57a6a029acb646f74` |
| `presence.Service.CheckInStudentsToOpenRoomSessionAuthorized` | `application.Service.observe` | `afade5fb4199e0ad403df633e85b1607` |
| `presence.Service.CheckInStudentsToOpenRoomSessionAuthorized` | `presence.ServiceDependencies.Now` | `8776a410a33ff9b57a6a029acb646f74` |

## Entscheidung

Die beiden Befehle verwenden die bestehende Anmeldung und die bestehende
Verschiebung unverändert. Ihre Diagnosen nennen diese ADR als `approval_adr`.
Die Uhr liefert nur den Zeitpunkt der Anmeldung, der Beobachter nur Messwerte.
Keiner von beiden erreicht den Zustand eines anderen Owners, nimmt eine Sperre
oder erweitert einen Scope. Die Digests sind dieselben wie bei den genehmigten
Einträgen der Verschiebung, die Zuordnung von Uhr und Beobachter bleibt also
gleich.

Eine eigene Anmeldung ohne Uhr oder eine Verschiebung ohne Beobachter würde die
Sperrreihenfolge und die Messung der bestehenden Wege doppeln. Den Scanner so zu
erweitern, dass er diese Funktionswerte auflöst, gehört nicht zu #6953.

Liegt diese ADR in der Basis, kann sie keine weitere Diagnose genehmigen. Eine
geänderte Zuordnung ändert den Digest und braucht eine neue Genehmigung.
