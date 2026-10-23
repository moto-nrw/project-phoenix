---
status: accepted
---

# Backup-Entschlüsselung bleibt vom Quellhost getrennt

Für Slice 3836-a werden Tagessicherungen vor dem Transport asymmetrisch mit
age verschlüsselt. Der Quellhost erhält nur den öffentlichen Empfänger; private
Identitäten werden vom Betreiber offline verwahrt und erst für einen isolierten
Restore bereitgestellt. Ein gemeinsames Verschlüsselungs-/Entschlüsselungsgeheimnis
auf dem Quellhost würde die Schlüsseltrennung bei dessen Kompromittierung aufheben.

Das erschwert automatische Wiederherstellung und macht die geprüfte Verwahrung
alter Identitäten bis zum Ablauf ihrer Sicherungen erforderlich. Verschlüsselung
ersetzt weder getrennte Zugangsrechte noch den Schutz vor dem Löschen aller
Generationen am Ziel. [Runbook und Nachweisgrenzen](../operations/daily-backup.md):
im Code umgesetzt und lokal getestet, produktive Einrichtung und Nachweise offen.
