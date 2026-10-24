---
status: accepted
---

# „Neue Nachricht“ markiert Kinder ohne erreichbare Eltern

Entscheidung zu [#4362](https://github.com/moto-nrw/project-phoenix/issues/4362).
Die Kinderliste in „Neue Nachricht“ zeigt alle Kinder der Schule. Ob ein Kind
Eltern mit App-Zugang hat, sah man bisher erst im zweiten Schritt. Kinder ohne
erreichbare Eltern stehen weiter in der Liste, tragen aber „Kein
Eltern-Zugang“ und lassen sich nicht wählen. Die Elternauswahl wäre für sie
leer. Lädt die Antwort nicht, bleibt die Liste ohne Markierung nutzbar.

„Erreichbar“ folgt genau der Regel der Elternauswahl
(`ListGuardiansForStudent`): ein Elternteil mit Portal-Konto, mit
`parent_portal.access` für dieses Kind und mit aktiver Mitgliedschaft in der
Schule. People Directory beantwortet die Frage für alle Kinder der Schule in
zwei Statements (Verknüpfungen, dann ein Mitgliedschafts-Abgleich). Beide
Abfragen bauen auf derselben Teilabfrage auf, damit die Regel nur an einer
Stelle steht. Communication bietet die Antwort als
`ListParentMessageReachableStudents` an (`GET /api/messages/reachable-students`).
Der Service gibt die Liste nur an Personen heraus, die alle Kinder lesen
dürfen (Admin oder verifiziertes Personal); sonst antwortet er mit 403.

Das bestehende Feld an `GET /api/students` wurde bewusst nicht erweitert. Die
Liste liegt in der Presence-Projektion und wird von vielen Seiten genutzt. Die
Frage nach den Eltern gehört zu Communication und People Directory. Ein Feld
dort hätte eine neue Leseverbindung zwischen den Ownern und eine zusätzliche
Abfrage auf jeder Schülerliste bedeutet.

Nach ADR 0057 Punkt 5 und ADR 0070 Punkt 4 erlaubt diese Entscheidung genau
diese Erhöhungen der Pass-through-Budgets:

- `modules/communication`: 57 auf 58, für
  `parentMessaging.ListParentMessageReachableStudents`. Das ist eine
  Beobachtungs-Hülle (Form 1 aus ADR 0075): Sie misst den Aufruf und bildet
  den Fehler auf den öffentlichen Vertrag ab, wie jede andere Methode der
  Fassade.
- `modulePassthroughOwnerBudgetTotal`: 1409 auf 1410, derselbe eine Treffer.

Weitere Budgets, Regeln oder Ausnahmen ändern sich nicht. Die Einträge bleiben
danach shrink-only.
