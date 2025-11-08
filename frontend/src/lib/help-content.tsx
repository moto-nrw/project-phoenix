// lib/help-content.tsx
import React from "react";
import type { ReactNode } from "react";

// Specific page help content
export const SPECIFIC_PAGE_HELP: Record<string, ReactNode> = {
  "student-detail": (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          Schülerdetailansicht
        </h3>
        <p className="leading-relaxed text-gray-700">
          Hier finden Sie alle wichtigen Informationen zu einem Schüler sowie
          Werkzeuge zur Verwaltung und Dokumentation.
        </p>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Verfügbare Informationen
        </h4>
        <div className="grid gap-3">
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">
                Persönliche Daten
              </span>
              <p className="text-sm text-gray-600">
                Name, Klasse, Geburtsdatum und Kontaktinformationen
              </p>
            </div>
          </div>
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">
                Aktueller Status
              </span>
              <p className="text-sm text-gray-600">
                Aufenthaltsort und aktuelle Aktivität
              </p>
            </div>
          </div>
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">
                Historie & Verlauf
              </span>
              <p className="text-sm text-gray-600">
                Besuchte Räume, Aktivitäten und Feedback
              </p>
            </div>
          </div>
        </div>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">Schnellaktionen</h4>
        <div className="space-y-2 text-sm">
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>Raumbewegungen dokumentieren</span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>Beobachtungen und Notizen hinzufügen</span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>Aufenthaltsorte bei Konflikten klären</span>
          </div>
        </div>
      </div>
    </div>
  ),
  feedback_history: (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          Feedback Historie
        </h3>
        <p className="leading-relaxed text-gray-700">
          Dokumentation und Übersicht aller pädagogischen Rückmeldungen und
          Beobachtungen zu diesem Schüler.
        </p>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Dokumentierte Inhalte
        </h4>
        <div className="grid gap-3">
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">
                Pädagogische Beobachtungen
              </span>
              <p className="text-sm text-gray-600">
                Wichtige Verhaltensnotizen und Entwicklungen
              </p>
            </div>
          </div>
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">Elterngespräche</span>
              <p className="text-sm text-gray-600">
                Protokolle und Vereinbarungen
              </p>
            </div>
          </div>
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">
                Entwicklungsschritte
              </span>
              <p className="text-sm text-gray-600">
                Positive Fortschritte und Erfolge
              </p>
            </div>
          </div>
        </div>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Verfügbare Funktionen
        </h4>
        <div className="space-y-2 text-sm">
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>Chronologische Darstellung nach Datum</span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>Filter nach Kategorie und Zeitraum</span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>Export für Elterngespräche</span>
          </div>
        </div>
      </div>
    </div>
  ),
  mensa_history: (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          Mensa Historie
        </h3>
        <p className="leading-relaxed text-gray-700">
          Vollständige Übersicht der Mahlzeitenteilnahme, Menüwahlen und
          ernährungsbezogenen Informationen.
        </p>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">Erfasste Daten</h4>
        <div className="grid gap-3">
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">
                Teilnahmehistorie
              </span>
              <p className="text-sm text-gray-600">
                Tägliche Anwesenheit beim Mittagessen
              </p>
            </div>
          </div>
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">Menüwahlen</span>
              <p className="text-sm text-gray-600">
                Gewählte Mahlzeiten und Vorlieben
              </p>
            </div>
          </div>
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">
                Diätische Hinweise
              </span>
              <p className="text-sm text-gray-600">
                Allergien und Unverträglichkeiten
              </p>
            </div>
          </div>
        </div>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Verwaltungsfunktionen
        </h4>
        <div className="space-y-2 text-sm">
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>Monatsübersicht mit Kalendaransicht</span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>An-/Abmeldung für kommende Tage</span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>Kostenübersicht und Abrechnung</span>
          </div>
        </div>
      </div>
    </div>
  ),
  room_history: (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          Raum Historie
        </h3>
        <p className="leading-relaxed text-gray-700">
          Detaillierter Verlauf aller Raumbewegungen mit präzisen Zeitstempeln
          für lückenlose Dokumentation.
        </p>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Erfasste Bewegungen
        </h4>
        <div className="grid gap-3">
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">Raumwechsel</span>
              <p className="text-sm text-gray-600">
                Chronologische Auflistung aller besuchten Räume
              </p>
            </div>
          </div>
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">Zeitstempel</span>
              <p className="text-sm text-gray-600">
                Exakte Ein- und Austrittszeiten
              </p>
            </div>
          </div>
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">
                Aktivitätszuordnung
              </span>
              <p className="text-sm text-gray-600">
                Verknüpfung zu Aktivitäten und Aufsichtspersonen
              </p>
            </div>
          </div>
        </div>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">Anwendungsbereiche</h4>
        <div className="space-y-2 text-sm">
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>Bewegungsmuster und Vorlieben analysieren</span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>Konfliktsituationen schnell aufklären</span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>Abholzeiten und Aufenthaltsdauer prüfen</span>
          </div>
        </div>
      </div>
    </div>
  ),
  "room-detail": (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          Raumdetailansicht
        </h3>
        <p className="leading-relaxed text-gray-700">
          Umfassende Informationen zur Raumnutzung, aktueller Belegung und
          detaillierter Nutzungshistorie.
        </p>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">Rauminformationen</h4>
        <div className="grid gap-3">
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">Basisdaten</span>
              <p className="text-sm text-gray-600">
                Name, Gebäude, Etage und Kapazität
              </p>
            </div>
          </div>
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">
                Aktuelle Belegung
              </span>
              <p className="text-sm text-gray-600">
                Live-Status und aktive Nutzer
              </p>
            </div>
          </div>
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">
                Nutzungshistorie
              </span>
              <p className="text-sm text-gray-600">
                Chronologische Übersicht aller Aktivitäten
              </p>
            </div>
          </div>
        </div>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Darstellungsoptionen
        </h4>
        <div className="space-y-2 text-sm">
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>Farbcodierte Verfügbarkeitsanzeige</span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>Detaillierte Aktivitätszuordnungen</span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>Zeitliche Auswertungen und Statistiken</span>
          </div>
        </div>
      </div>
    </div>
  ),
};

// Navigation-based help content
export const NAVIGATION_HELP: Record<
  string,
  { title: string; content: ReactNode }
> = {
  "/dashboard": {
    title: "Dashboard Hilfe",
    content: (
      <div className="space-y-6">
        <div>
          <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
            <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
            Dashboard Übersicht
          </h3>
          <p className="leading-relaxed text-gray-700">
            Ihr zentraler Arbeitsplatz für die Verwaltung der Ganztagsbetreuung
            mit schnellem Zugriff auf alle wichtigen Funktionen.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Hauptfunktionen</h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">OGS Gruppen</span>
                <p className="text-sm text-gray-600">
                  Verwalten Sie Ganztagsgruppen und deren Betreuung
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Schüler</span>
                <p className="text-sm text-gray-600">
                  Suchen und verwalten Sie Schülerdaten
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Räume</span>
                <p className="text-sm text-gray-600">
                  Raumverwaltung und -zuweisung
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Aktivitäten</span>
                <p className="text-sm text-gray-600">
                  Erstellen und bearbeiten Sie Aktivitäten
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Weitere Bereiche</h4>
          <div className="space-y-2 text-sm">
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Statistiken:</strong> Einblick in wichtige Kennzahlen
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Vertretungen:</strong> Vertretungsplan verwalten
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Datenbank:</strong> Erweiterte Datenverwaltung
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Einstellungen:</strong> System konfigurieren
              </span>
            </div>
          </div>
        </div>
      </div>
    ),
  },
  "/ogs_groups": {
    title: "OGS-Gruppenansicht Hilfe",
    content: (
      <div className="space-y-6">
        <div>
          <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
            <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
            OGS Gruppenübersicht
          </h3>
          <p className="leading-relaxed text-gray-700">
            Behalten Sie den Überblick über alle Kinder Ihrer Gruppe und deren
            aktuellen Aufenthaltsort in Echtzeit.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Verfügbare Funktionen
          </h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Schüler verfolgen
                </span>
                <p className="text-sm text-gray-600">
                  Sehen Sie in Echtzeit, wo sich alle Kinder Ihrer Gruppe
                  befinden
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Status einsehen
                </span>
                <p className="text-sm text-gray-600">
                  Detaillierte Aufenthalts- und Aktivitätsinformationen
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Filtern und suchen
                </span>
                <p className="text-sm text-gray-600">
                  Nach Name, Jahrgang oder Status sortieren
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Statuserklärungen
          </h4>
          <div className="space-y-2 text-sm">
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Im Gruppenraum:</strong> Kind befindet sich im
                angegebenen OGS-Raum
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Schulhof:</strong> Kind ist auf dem Außengelände
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Toilette:</strong> Kind ist auf der Toilette
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Unterwegs:</strong> Kind ist in einer Aktivität oder
                wechselt gerade einen Raum
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Zuhause:</strong> Kind wurde abgeholt oder ist zuhause
              </span>
            </div>
          </div>
        </div>
      </div>
    ),
  },
  "/students": {
    title: "Schülersuche Hilfe",
    content: (
      <div className="space-y-6">
        <div>
          <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
            <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
            Schülersuche & Verwaltung
          </h3>
          <p className="leading-relaxed text-gray-700">
            Finden Sie schnell und effizient alle Schülerinformationen und
            verwalten Sie deren Daten zentral.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Suchfunktionen</h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Schüler finden
                </span>
                <p className="text-sm text-gray-600">
                  Suche nach Namen, Klasse oder anderen Merkmalen
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Informationen einsehen
                </span>
                <p className="text-sm text-gray-600">
                  Persönliche Daten, Anwesenheiten und Aktivitäten
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Schnellfilter nutzen
                </span>
                <p className="text-sm text-gray-600">
                  Nach Jahrgangsstufe oder Anwesenheitsstatus filtern
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Tipps zur Suche</h4>
          <div className="space-y-2 text-sm">
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                Nutzen Sie die <strong>Filteroptionen</strong> für spezifischere
                Ergebnisse
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>Die Suche berücksichtigt Vor- und Nachnamen</span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                Klicken Sie auf einen Schüler, um sein Profil zu öffnen
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>Exportieren Sie Ergebnisse mit dem Export-Button</span>
            </div>
          </div>
        </div>
      </div>
    ),
  },
  "/rooms": {
    title: "Raumverwaltung Hilfe",
    content: (
      <div className="space-y-6">
        <div>
          <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
            <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
            Raumverwaltung & Belegung
          </h3>
          <p className="leading-relaxed text-gray-700">
            Verwalten Sie alle Räume effizient und behalten Sie den Überblick
            über Belegungen und Verfügbarkeiten.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Grundfunktionen</h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Räume einsehen
                </span>
                <p className="text-sm text-gray-600">
                  Übersicht aller verfügbaren Räume mit Details
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Belegung prüfen
                </span>
                <p className="text-sm text-gray-600">
                  Aktuelle und geplante Raumbelegungen in Echtzeit
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Räume zuweisen
                </span>
                <p className="text-sm text-gray-600">
                  Für Gruppen oder Aktivitäten reservieren
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Erweiterte Funktionen
          </h4>
          <div className="space-y-2 text-sm">
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Raumplan:</strong> Visuelle Darstellung der Raumbelegung
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Kalenderansicht:</strong> Zeitliche Übersicht der
                Raumbelegungen
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Konfliktprüfung:</strong> Automatische Erkennung von
                Doppelbelegungen
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Ausstattungsfilter:</strong> Suche nach Räumen mit
                bestimmter Ausstattung
              </span>
            </div>
          </div>
        </div>
      </div>
    ),
  },
  "/activities": {
    title: "Aktivitäten Hilfe",
    content: (
      <div className="space-y-6">
        <div>
          <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
            <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
            Aktivitätenübersicht
          </h3>
          <p className="leading-relaxed text-gray-700">
            Verwalten Sie alle Aktivitäten, Programme und AGs mit detaillierter
            Übersicht und flexiblen Filteroptionen.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Verwaltungsfunktionen
          </h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Aktivitäten einsehen
                </span>
                <p className="text-sm text-gray-600">
                  Übersicht aller verfügbaren Aktivitäten und AGs
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Filtern & Sortieren
                </span>
                <p className="text-sm text-gray-600">
                  Nach Kategorie, Status, Teilnahmemöglichkeit und Gebäude
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Detailansicht</span>
                <p className="text-sm text-gray-600">
                  Informationen zu Leitung, Teilnehmerzahl und Ort
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Aktivitätskategorien
          </h4>
          <div className="space-y-2 text-sm">
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Kreatives/Musik:</strong> Chor, Theaterprojekt und
                andere kreative Angebote
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>NW/Technik:</strong> Informatik AG und technische
                Aktivitäten
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Bewegen/Ruhe:</strong> Sport-AGs wie Basketball
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Lernen:</strong> Lesegruppe und andere Bildungsangebote
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Hauswirtschaft:</strong> Praktische Aktivitäten
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Natur:</strong> Garten AG und naturnahe Angebote
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Spielen:</strong> Verschiedene Spiel- und
                Freizeitaktivitäten
              </span>
            </div>
          </div>
        </div>
      </div>
    ),
  },
  "/statistics": {
    title: "Statistiken Hilfe",
    content: (
      <div className="space-y-6">
        <div>
          <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
            <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
            Statistiken & Auswertungen
          </h3>
          <p className="leading-relaxed text-gray-700">
            Umfassende Datenauswertungen und Kennzahlen für fundierte
            Entscheidungen und Berichte.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Verfügbare Statistiken
          </h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Anwesenheitsstatistiken
                </span>
                <p className="text-sm text-gray-600">
                  Tägliche, wöchentliche und monatliche Übersichten
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Gruppenzahlen</span>
                <p className="text-sm text-gray-600">
                  Auslastung der OGS-Gruppen und Kapazitäten
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Aktivitätenanalyse
                </span>
                <p className="text-sm text-gray-600">
                  Beliebtheit und Teilnehmerzahlen der Angebote
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Funktionen & Tools
          </h4>
          <div className="space-y-2 text-sm">
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Exportieren:</strong> Daten als Excel oder PDF
                herunterladen
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Zeitraumfilter:</strong> Daten für bestimmte Zeiträume
                anzeigen
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Vergleichsfunktion:</strong> Daten verschiedener
                Zeiträume vergleichen
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Diagramme:</strong> Visuelle Darstellung wichtiger
                Kennzahlen
              </span>
            </div>
          </div>
        </div>
      </div>
    ),
  },
  "/substitutions": {
    title: "Vertretungen Hilfe",
    content: (
      <div className="space-y-6">
        <div>
          <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
            <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
            Vertretungsmanagement
          </h3>
          <p className="leading-relaxed text-gray-700">
            Organisieren Sie Personalausfälle und Vertretungen effizient, um
            eine durchgängige Betreuung sicherzustellen.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Grundfunktionen</h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Ausfälle erfassen
                </span>
                <p className="text-sm text-gray-600">
                  Abwesenheiten von Betreuern systematisch dokumentieren
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Vertretungen planen
                </span>
                <p className="text-sm text-gray-600">
                  Ersatzpersonal für abwesende Betreuer strategisch einteilen
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Konflikte erkennen
                </span>
                <p className="text-sm text-gray-600">
                  Unterbesetzte Gruppen und kritische Situationen identifizieren
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Erweiterte Features
          </h4>
          <div className="space-y-2 text-sm">
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Kalenderansicht:</strong> Übersicht aller geplanten
                Vertretungen
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Benachrichtigungen:</strong> Betroffene Mitarbeiter
                automatisch informieren
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Auslastungsanzeige:</strong> Verfügbarkeit von
                Vertretungskräften prüfen
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Notfallplan:</strong> Strukturiertes Vorgehen bei
                unerwarteten Ausfällen
              </span>
            </div>
          </div>
        </div>
      </div>
    ),
  },
  "/database": {
    title: "Datenbank Hilfe",
    content: (
      <div className="space-y-6">
        <div>
          <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
            <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
            Datenbankmanagement
          </h3>
          <p className="leading-relaxed text-gray-700">
            Zentrale Verwaltung aller Stammdaten und Konfigurationen für eine
            optimale Systemleistung.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Datentypen verwalten
          </h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Gruppen verwalten
                </span>
                <p className="text-sm text-gray-600">
                  Erstellen und bearbeiten von OGS-Gruppen
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Aktivitäten organisieren
                </span>
                <p className="text-sm text-gray-600">
                  Aktivitäten erstellen und Schüler zuweisen
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Räume konfigurieren
                </span>
                <p className="text-sm text-gray-600">
                  Raumkapazitäten und Zuordnungen verwalten
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Schüler administrieren
                </span>
                <p className="text-sm text-gray-600">
                  Schülerdaten pflegen und aktualisieren
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Pädagogische Fachkräfte verwalten
                </span>
                <p className="text-sm text-gray-600">
                  Daten der pädagogischen Fachkräfte und Betreuerdaten
                  organisieren
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Erweiterte Funktionen
          </h4>
          <div className="space-y-2 text-sm">
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Datenimport:</strong> Massendaten importieren und
                synchronisieren
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Datenexport:</strong> Strukturierte Daten für Berichte
                exportieren
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Datenpflege:</strong> Systemweit Daten aktualisieren und
                bereinigen
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Beziehungen verwalten:</strong> Verknüpfungen zwischen
                Datensätzen pflegen
              </span>
            </div>
          </div>
        </div>
      </div>
    ),
  },
  "/staff": {
    title: "Mitarbeiter Hilfe",
    content: (
      <div className="space-y-6">
        <div>
          <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
            <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
            Personalübersicht
          </h3>
          <p className="leading-relaxed text-gray-700">
            Verschaffen Sie sich einen schnellen Überblick über alle Mitarbeiter
            und deren aktuellen Einsatzort im Ganztag.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Verfügbare Informationen
          </h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Mitarbeiterübersicht
                </span>
                <p className="text-sm text-gray-600">
                  Alle pädagogischen Fachkräfte auf einen Blick
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Einsatzorte</span>
                <p className="text-sm text-gray-600">
                  Aktueller Aufenthaltsort und betreute Räume/Gruppen
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Qualifikationen
                </span>
                <p className="text-sm text-gray-600">
                  Spezialisierungen und Zusatzqualifikationen
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Statusanzeigen</h4>
          <div className="space-y-2 text-sm">
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Zuhause:</strong> Mitarbeiter ist nicht im Dienst
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Im Raum:</strong> Mitarbeiter betreut einen spezifischen
                Raum
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Schulhof:</strong> Mitarbeiter beaufsichtigt den
                Außenbereich
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Unterwegs:</strong> Mitarbeiter ist zwischen Räumen oder
                in Bewegung
              </span>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Funktionen</h4>
          <div className="space-y-2 text-sm">
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Suche:</strong> Mitarbeiter nach Namen suchen
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Filter:</strong> Nach Aufenthaltsort filtern
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Informationskarten:</strong> Nur zur Ansicht - keine
                Navigation
              </span>
            </div>
          </div>
        </div>
      </div>
    ),
  },
  "/settings": {
    title: "Einstellungen Hilfe",
    content: (
      <div className="space-y-6">
        <div>
          <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
            <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
            Systemeinstellungen
          </h3>
          <p className="leading-relaxed text-gray-700">
            Konfigurieren Sie persönliche Präferenzen und administrative
            Systemeinstellungen für eine optimale Nutzererfahrung.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Persönliche Einstellungen
          </h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Benutzerprofil
                </span>
                <p className="text-sm text-gray-600">
                  Persönliche Daten und Zugangsdaten verwalten
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Benachrichtigungen
                </span>
                <p className="text-sm text-gray-600">
                  E-Mail und Push-Benachrichtigungen konfigurieren
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Erscheinungsbild
                </span>
                <p className="text-sm text-gray-600">
                  Benutzeroberfläche individuell anpassen
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Administrative Einstellungen
          </h4>
          <div className="space-y-2 text-sm">
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Benutzer verwalten:</strong> Neue Konten erstellen und
                Berechtigungen zuweisen
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Schuljahr konfigurieren:</strong> Ferien und Feiertage
                festlegen
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Systemeinstellungen:</strong> Grundlegende Konfiguration
                des Systems
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Datensicherung:</strong> Backups erstellen und
                wiederherstellen
              </span>
            </div>
          </div>
        </div>
      </div>
    ),
  },
};

// Function to get help content based on current pathname
export function getHelpContent(pathname: string): {
  title: string;
  content: ReactNode;
} {
  // Check for specific page patterns first
  if (/\/students\/[^\/]+$/.test(pathname)) {
    return {
      title: "Schülerdetail Hilfe",
      content: SPECIFIC_PAGE_HELP["student-detail"],
    };
  }

  if (/\/students\/[^\/]+\/feedback_history/.test(pathname)) {
    return {
      title: "Feedback Historie Hilfe",
      content: SPECIFIC_PAGE_HELP.feedback_history,
    };
  }

  if (/\/students\/[^\/]+\/mensa_history/.test(pathname)) {
    return {
      title: "Mensa Historie Hilfe",
      content: SPECIFIC_PAGE_HELP.mensa_history,
    };
  }

  if (/\/students\/[^\/]+\/room_history/.test(pathname)) {
    return {
      title: "Raum Historie Hilfe",
      content: SPECIFIC_PAGE_HELP.room_history,
    };
  }

  if (/\/rooms\/[^\/]+$/.test(pathname)) {
    return {
      title: "Raumdetail Hilfe",
      content: SPECIFIC_PAGE_HELP["room-detail"],
    };
  }

  // Check for main navigation routes
  for (const [route, helpData] of Object.entries(NAVIGATION_HELP)) {
    if (route === "/dashboard" && pathname === "/dashboard") {
      return helpData;
    } else if (route !== "/dashboard" && pathname.startsWith(route)) {
      return helpData;
    }
  }

  // Default help content
  return {
    title: "Allgemeine Hilfe",
    content: (
      <div>
        <p>
          Willkommen im <strong>moto</strong> Hilfesystem!
        </p>
        <p className="mt-3">
          Diese Seite bietet kontextbezogene Hilfe basierend auf Ihrer aktuellen
          Position in der Anwendung.
        </p>
        <p className="mt-3">
          Navigieren Sie zu verschiedenen Bereichen, um spezifische Hilfe zu
          erhalten.
        </p>
      </div>
    ),
  };
}
