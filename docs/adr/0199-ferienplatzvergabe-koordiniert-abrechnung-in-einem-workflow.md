---
status: accepted
---

# Ferienplatzvergabe koordiniert Abrechnung in einem Workflow

Die Büroentscheidung über Ferienplätze und die Neuberechnung der
Geschwisterbeiträge benötigen eine gemeinsame Sperrreihenfolge und Transaktion
(#4700, PR #6448). Nach ADR 0067 ist dies ein Workflow. Der neue datenlose
Workflow-Owner `holiday-allocation` unter `workflows/holidayallocation` ersetzt
den HTTP-Koordinator und den Callback in Parent Billing. Die Root verdrahtet
nur die Fähigkeiten, die Workflow-Composition bindet `tenant.WithinTraeger`.
Policy-Epoch 100 registriert ausschließlich neue Workflow-Pakete nach dem
Verfahren aus ADR 0013 und ADR 0070; Tabellen und bestehende Owner bleiben gleich.

Der Workflow öffnet oder verbindet die Träger-Transaktion und hält zuerst die
Rechnungen durch Parent Billing, dann den Zeitraum durch Enrollment. Enrollment
prüft den Vorschlag unter seinen Kriterien- und aufsteigend genommenen
Wochensperren und schreibt Plätze, Entscheidungen, Verlauf und Mail-Outbox.
Anschließend berechnet Parent Billing die bestätigten Geschwistergruppen aller
direkt platzierten und nachgerückten Anmeldungen neu. Jeder Fehler verwirft die
gesamte Einheit. Kein Owner ruft dafür einen fremden Entscheidungs-Callback auf.
Die bisher ungenutzten Gruppen- und Rechnungsabfragen vor der Entscheidung
entfallen; die Neuberechnung liest den Stand weiterhin unter denselben Sperren.

Die bestehenden Pfade für Umbuchung und Kapazitätsänderung gehören nicht zu
diesem Schnitt; ihr gemeinsamer Umzug bleibt Gegenstand von #6555.
`TestHolidayAllocationHoldsFreePlacesForThePendingBookings` prüft das echte
Rollback von Entscheidung, Nachrücken, Beiträgen und beiden Mails bei einem
abgelehnten Beitrag. Der Workflow-Test prüft Sperrreihenfolge, gemeinsame
Transaktion, beide Arten platzierter Anmeldungen und Abbruch an jeder Stufe.

Der separat aufrufbare Rechnungssperrbefehl prüft Träger und Konto und bildet
Sperrfehler auf den öffentlichen Vertrag ab. Er ist ein Owner-Edge-Kommando
nach ADR 0075, kein ungeprüfter Forward. Dafür steigt ausschließlich
`modulePassthroughOwnerBudgets["modules/parentbilling"]` von 11 auf 12
(zugehörige Summe `modulePassthroughOwnerBudgetTotal`: 1406 auf 1407). Diese ADR
autorisiert genau diesen zusätzlichen Eintrag. Scanner, Schwellenwerte und
alle anderen Pass-through-Budgets bleiben unverändert; der bisherige Entscheidungs-Callback
wird entfernt. Die aufrufende Koordination bleibt vollständig im Workflow.

Die neue Composition bindet ausschließlich die Owner-Fähigkeiten und den
Träger-Transaktionsläufer. Ihre gemessenen 13 Zeilen erhalten den neuen Eintrag
`moduleComposeBudgets["workflows/holidayallocation/compose"] = 13`; die Summe
`moduleComposeBudgetTotal` steigt von 13255 auf 13268. Diese ADR autorisiert
ausschließlich diese zusätzliche Construction-Grenze; bestehende
Composition-Budgets und die LOC-Messung bleiben unverändert.
