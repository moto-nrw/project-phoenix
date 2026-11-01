# Bundle-Referenzen nach unten nachziehen (#5483)

## Messplan

Ausgangsstand: `2446b9837128419bb7fdc98da871d0848e813834`, aktuelles
`origin/development` beim Arbeitsbeginn, einschließlich #5463.

Zwei aufeinanderfolgende kalte Linux-Analysen desselben archivierten
Frontend-Quellbaums mit eingefrorenem Lockfile prüfen den aktuellen Abstand
zur Referenz und das Messrauschen. Die Messungen umfassen alle Client-Chunks,
auch dynamisch geladene; gemeinsame Chunks werden je Route gezählt.

Nur nachweislich zu hohe Referenzen werden gesenkt. Keine automatische
Budgeterhöhung, keine Änderung der PR-Delta-Toleranz und keine Freigabe
geerbter Überschreitungen. Ergebnisse und Reproduktionsbefehle folgen mit
den Messungen. Die spätere Warnschwelle wird anhand ihrer Streuung gewählt.
