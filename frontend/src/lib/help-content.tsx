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
          Schülerdetails
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
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">
                Erziehungsberechtigte
              </span>
              <p className="text-sm text-gray-600">
                Kontaktdaten der Erziehungsberechtigten
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
              <p className="text-sm text-gray-600">Name, Gebäude und Etage</p>
            </div>
          </div>
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">
                Aktuelle Belegung
              </span>
              <p className="text-sm text-gray-600">Live-Status</p>
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
    </div>
  ),
  "database-students": (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          Schüler Verwaltung
        </h3>
        <p className="leading-relaxed text-gray-700">
          Verwalte alle Schülerdaten zentral an einem Ort.
        </p>
      </div>
      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Verfügbare Operationen
        </h4>
        <div className="space-y-2 text-sm">
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anlegen:</strong> Neue Schüler hinzufügen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Bearbeiten:</strong> Bestehende Daten ändern
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anzeigen:</strong> Details einsehen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Löschen:</strong> Datensätze entfernen
            </span>
          </div>
        </div>
      </div>
    </div>
  ),
  "database-teachers": (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          Betreuer Verwaltung
        </h3>
        <p className="leading-relaxed text-gray-700">
          Verwalte alle Betreuerdaten zentral an einem Ort.
        </p>
      </div>
      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Verfügbare Operationen
        </h4>
        <div className="space-y-2 text-sm">
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anlegen:</strong> Neue Betreuer hinzufügen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Bearbeiten:</strong> Bestehende Daten ändern
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anzeigen:</strong> Details einsehen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Löschen:</strong> Datensätze entfernen
            </span>
          </div>
        </div>
      </div>
    </div>
  ),
  "database-rooms": (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          Räume Verwaltung
        </h3>
        <p className="leading-relaxed text-gray-700">
          Verwalte alle Räume zentral an einem Ort.
        </p>
      </div>
      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Verfügbare Operationen
        </h4>
        <div className="space-y-2 text-sm">
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anlegen:</strong> Neue Räume hinzufügen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Bearbeiten:</strong> Bestehende Daten ändern
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anzeigen:</strong> Details einsehen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Löschen:</strong> Datensätze entfernen
            </span>
          </div>
        </div>
      </div>
    </div>
  ),
  "database-activities": (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          Aktivitäten Verwaltung
        </h3>
        <p className="leading-relaxed text-gray-700">
          Verwalte alle Aktivitäten zentral an einem Ort.
        </p>
      </div>
      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Verfügbare Operationen
        </h4>
        <div className="space-y-2 text-sm">
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anlegen:</strong> Neue Aktivitäten hinzufügen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Bearbeiten:</strong> Bestehende Daten ändern
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anzeigen:</strong> Details einsehen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Löschen:</strong> Datensätze entfernen
            </span>
          </div>
        </div>
      </div>
    </div>
  ),
  "database-groups": (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          Gruppen Verwaltung
        </h3>
        <p className="leading-relaxed text-gray-700">
          Verwalte alle Gruppen zentral an einem Ort.
        </p>
      </div>
      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Verfügbare Operationen
        </h4>
        <div className="space-y-2 text-sm">
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anlegen:</strong> Neue Gruppen hinzufügen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Bearbeiten:</strong> Bestehende Daten ändern
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anzeigen:</strong> Details einsehen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Löschen:</strong> Datensätze entfernen
            </span>
          </div>
        </div>
      </div>
    </div>
  ),
  "database-roles": (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          Rollen Verwaltung
        </h3>
        <p className="leading-relaxed text-gray-700">
          Verwalte alle Benutzerrollen zentral an einem Ort.
        </p>
      </div>
      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Verfügbare Operationen
        </h4>
        <div className="space-y-2 text-sm">
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anlegen:</strong> Neue Rollen hinzufügen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Bearbeiten:</strong> Bestehende Daten ändern
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anzeigen:</strong> Details einsehen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Löschen:</strong> Datensätze entfernen
            </span>
          </div>
        </div>
      </div>
    </div>
  ),
  "database-devices": (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          Geräte Verwaltung
        </h3>
        <p className="leading-relaxed text-gray-700">
          Verwalte alle IoT-Geräte und RFID-Reader zentral an einem Ort.
        </p>
      </div>
      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Verfügbare Operationen
        </h4>
        <div className="space-y-2 text-sm">
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anlegen:</strong> Neue Geräte hinzufügen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Bearbeiten:</strong> Bestehende Daten ändern
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anzeigen:</strong> Details einsehen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Löschen:</strong> Datensätze entfernen
            </span>
          </div>
        </div>
      </div>
    </div>
  ),
  "database-permissions": (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          Berechtigungen Verwaltung
        </h3>
        <p className="leading-relaxed text-gray-700">
          Verwalte alle Systemberechtigungen zentral an einem Ort.
        </p>
      </div>
      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Verfügbare Operationen
        </h4>
        <div className="space-y-2 text-sm">
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anlegen:</strong> Neue Berechtigungen hinzufügen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Bearbeiten:</strong> Bestehende Daten ändern
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Anzeigen:</strong> Details einsehen
            </span>
          </div>
          <div className="flex items-center space-x-2">
            <span className="h-2 w-2 rounded-full bg-gray-400"></span>
            <span>
              <strong>Löschen:</strong> Datensätze entfernen
            </span>
          </div>
        </div>
      </div>
    </div>
  ),
  invitations: (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          Einladungen
        </h3>
        <p className="leading-relaxed text-gray-700">
          Verwalte Einladungen für neue Nutzer per E-Mail.
        </p>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">Funktionen</h4>
        <div className="grid gap-3">
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">Nutzer einladen</span>
              <p className="text-sm text-gray-600">
                E-Mail-Einladung an neue Nutzer versenden
              </p>
            </div>
          </div>
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">
                Einladungen einsehen
              </span>
              <p className="text-sm text-gray-600">
                Übersicht aller offenen Einladungen
              </p>
            </div>
          </div>
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">
                Einladungen zurückziehen
              </span>
              <p className="text-sm text-gray-600">
                Offene Einladungen löschen
              </p>
            </div>
          </div>
          <div className="flex items-start space-x-3">
            <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
            <div>
              <span className="font-medium text-gray-900">
                Einladung erneut senden
              </span>
              <p className="text-sm text-gray-600">
                E-Mail-Einladung nochmals versenden
              </p>
            </div>
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
            Ihr zentrales Analyse-Dashboard mit Echtzeit-Übersicht über alle
            wichtigen Kennzahlen der Ganztagsbetreuung.
          </p>
          <p className="mt-3 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700">
            <strong>Hinweis:</strong> Das Dashboard ist nur für Administratoren
            zugänglich.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Anwesenheitsübersicht
          </h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Kinder anwesend
                </span>
                <p className="text-sm text-gray-600">
                  Gesamtzahl der aktuell anwesenden Kinder (klickbar zur
                  Schülersuche)
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">In Räumen</span>
                <p className="text-sm text-gray-600">
                  Kinder, die sich gerade in Räumen befinden
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Unterwegs</span>
                <p className="text-sm text-gray-600">
                  Kinder unterwegs von Raum zu Raum
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Schulhof</span>
                <p className="text-sm text-gray-600">
                  Anzahl der Kinder auf dem Schulhof
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Ressourcenübersicht
          </h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Aktive Gruppen
                </span>
                <p className="text-sm text-gray-600">
                  Anzahl laufender OGS-Gruppen (klickbar zur Gruppenübersicht)
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Aktive Aktivitäten
                </span>
                <p className="text-sm text-gray-600">
                  Anzahl laufender Aktivitäten (klickbar zur
                  Aktivitätenübersicht)
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Freie Räume</span>
                <p className="text-sm text-gray-600">
                  Verfügbare Räume (klickbar zur Raumverwaltung)
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Auslastung</span>
                <p className="text-sm text-gray-600">
                  Prozentuale Raumauslastung
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Live-Übersichten</h4>
          <div className="space-y-2 text-sm">
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Letzte Bewegungen:</strong> Aktuelle Raumwechsel von
                Gruppen
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Laufende Aktivitäten:</strong> Details zu aktuellen
                Aktivitäten mit Teilnehmerzahl
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Aktive Gruppen:</strong> Übersicht aller aktiven Gruppen
                mit Standort
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-gray-400"></span>
              <span>
                <strong>Personal heute:</strong> Betreuer im Dienst
              </span>
            </div>
          </div>
        </div>
      </div>
    ),
  },
  "/ogs-groups": {
    title: "OGS-Gruppenansicht Hilfe",
    content: (
      <div className="space-y-6">
        <div>
          <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
            <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
            OGS Gruppenübersicht
          </h3>
          <p className="leading-relaxed text-gray-700">
            Übersicht aller Kinder in deiner OGS-Gruppe oder deinen OGS-Gruppen
            mit Echtzeit-Standortverfolgung und Live-Updates.
          </p>
          <p className="mt-3 leading-relaxed text-gray-700">
            Tippe auf ein Kind, um weitere Informationen einzusehen.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Hauptfunktionen</h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Schüler verfolgen
                </span>
                <p className="text-sm text-gray-600">
                  Sehen Sie in Echtzeit, wo sich alle Kinder befinden
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Live-Updates</span>
                <p className="text-sm text-gray-600">
                  Automatische Aktualisierung bei Standortwechseln
                  (Status-Indikator oben)
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Mehrere Gruppen
                </span>
                <p className="text-sm text-gray-600">
                  Bei Leitung mehrerer Gruppen: Schneller Wechsel per Tabs
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Schüler-Details
                </span>
                <p className="text-sm text-gray-600">
                  Auf Karte tippen für vollständige Schüler-Kartei
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Filter & Suche</h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Namenssuche</span>
                <p className="text-sm text-gray-600">
                  Schnelle Suche nach Vor- oder Nachname
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Klassenstufe</span>
                <p className="text-sm text-gray-600">
                  Filtern nach Jahrgang (1, 2, 3, 4)
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Aufenthaltsort-Filter
                </span>
                <p className="text-sm text-gray-600">
                  6 Filteroptionen für verschiedene Standorte
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Aufenthaltsorte</h4>
          <div className="space-y-2 text-sm">
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-emerald-500"></span>
              <span>
                <strong>Gruppenraum:</strong> Kind befindet sich im zugewiesenen
                OGS-Raum
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-blue-500"></span>
              <span>
                <strong>Fremder Raum:</strong> Kind ist in einem anderen Raum
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-fuchsia-500"></span>
              <span>
                <strong>Unterwegs:</strong> Kind wechselt gerade den Raum oder
                ist in Bewegung
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-yellow-500"></span>
              <span>
                <strong>Schulhof:</strong> Kind ist auf dem Außengelände
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="h-2 w-2 rounded-full bg-red-500"></span>
              <span>
                <strong>Zuhause:</strong> Kind wurde abgeholt und ist zuhause
              </span>
            </div>
          </div>
        </div>
      </div>
    ),
  },
  "/active-supervisions": {
    title: "Aktuelle Aufsicht Hilfe",
    content: (
      <div className="space-y-6">
        <div>
          <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
            <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
            Aktuelle Aufsicht Übersicht
          </h3>
          <p className="leading-relaxed text-gray-700">
            Übersicht aller Kinder, die aktuell in deinem Raum sind, in dem du
            eine Aktivität leitest.
          </p>
          <p className="mt-3 leading-relaxed text-gray-700">
            Tippe auf ein Kind, um weitere Informationen einzusehen.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Filter & Suche</h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Namenssuche</span>
                <p className="text-sm text-gray-600">
                  Schnelle Suche nach Vor- oder Nachname
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Gruppen-Filter
                </span>
                <p className="text-sm text-gray-600">Filtern nach OGS-Gruppe</p>
              </div>
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
            Schülersuche
          </h3>
          <p className="leading-relaxed text-gray-700">
            Hier werden alle Kinder der OGS angezeigt.
          </p>
          <p className="mt-3 leading-relaxed text-gray-700">
            Tippe auf ein Kind, um weitere Informationen einzusehen.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Sichtbare Informationen
          </h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Kinder deiner OGS-Gruppe
                </span>
                <p className="text-sm text-gray-600">
                  Vollständige Standorte sichtbar
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Fremde Kinder</span>
                <p className="text-sm text-gray-600">
                  Nur Anwesenheitsstatus: Anwesend (in der OGS), Zuhause (krank
                  oder abgeholt)
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Filter</h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Namenssuche</span>
                <p className="text-sm text-gray-600">
                  Schnelle Suche nach Vor- oder Nachname
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Gruppen & Jahrgänge
                </span>
                <p className="text-sm text-gray-600">
                  Filtern nach OGS-Gruppe oder Klassenstufe
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Anwesenheitsstatus
                </span>
                <p className="text-sm text-gray-600">
                  Filtern nach Aufenthaltsort
                </p>
              </div>
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
            Aktivitäten
          </h3>
          <p className="leading-relaxed text-gray-700">
            Hier findest du alle Aktivitäten, die bisher in der App erstellt
            wurden.
          </p>
          <p className="mt-3 leading-relaxed text-gray-700">
            Du kannst die Aktivitäten bearbeiten und neue Aktivitäten erstellen.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Funktionen</h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Aktivitäten einsehen
                </span>
                <p className="text-sm text-gray-600">
                  Übersicht aller Aktivitäten und AGs
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Aktivitäten erstellen
                </span>
                <p className="text-sm text-gray-600">
                  Neue Aktivitäten hinzufügen
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Aktivitäten bearbeiten
                </span>
                <p className="text-sm text-gray-600">
                  Bestehende Aktivitäten anpassen
                </p>
              </div>
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
          <h4 className="mb-3 font-semibold text-gray-900">Funktionen</h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Vertretungen planen
                </span>
                <p className="text-sm text-gray-600">
                  Ersatzpersonal für abwesende Betreuer einteilen
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Fachkräfte zuweisen
                </span>
                <p className="text-sm text-gray-600">
                  Pädagogische Fachkräfte für beliebige Tage zuweisen
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Vertretungen entfernen
                </span>
                <p className="text-sm text-gray-600">
                  Zugewiesene Vertretungen bei Bedarf wieder entfernen
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    ),
  },
  "/database": {
    title: "Datenverwaltung Hilfe",
    content: (
      <div className="space-y-6">
        <div>
          <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
            <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
            Datenverwaltung
          </h3>
          <p className="leading-relaxed text-gray-700">
            Zentrale Verwaltung aller Stammdaten mit Zugriff auf verschiedene
            Datenebenen.
          </p>
          <p className="mt-3 leading-relaxed text-gray-700">
            In jeder Ebene können Daten angelegt, geändert, angezeigt oder
            gelöscht werden.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Verfügbare Datenebenen (8)
          </h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Schüler</span>
                <p className="text-sm text-gray-600">
                  Schülerdaten verwalten und bearbeiten
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Betreuer</span>
                <p className="text-sm text-gray-600">
                  Daten der Betreuer verwalten
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Räume</span>
                <p className="text-sm text-gray-600">Räume verwalten</p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Aktivitäten</span>
                <p className="text-sm text-gray-600">Aktivitäten verwalten</p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Gruppen</span>
                <p className="text-sm text-gray-600">Gruppen verwalten</p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Rollen</span>
                <p className="text-sm text-gray-600">
                  Benutzerrollen und Berechtigungen verwalten
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Geräte</span>
                <p className="text-sm text-gray-600">
                  IoT-Geräte und RFID-Reader verwalten
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">
                  Berechtigungen
                </span>
                <p className="text-sm text-gray-600">
                  Systemberechtigungen ansehen
                </p>
              </div>
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
            Einstellungen
          </h3>
          <p className="leading-relaxed text-gray-700">
            Verwalte deine persönlichen Daten und Zugangsdaten.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Verfügbare Einstellungen
          </h4>
          <div className="grid gap-3">
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Profil</span>
                <p className="text-sm text-gray-600">
                  Vor- und Nachname ändern
                </p>
              </div>
            </div>
            <div className="flex items-start space-x-3">
              <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
              <div>
                <span className="font-medium text-gray-900">Sicherheit</span>
                <p className="text-sm text-gray-600">Passwort ändern</p>
              </div>
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
  // Check for /students/search FIRST (before the general /students/[id] pattern)
  if (
    pathname === "/students/search" ||
    pathname.startsWith("/students/search")
  ) {
    return NAVIGATION_HELP["/students"]!;
  }

  // Check for specific page patterns (student detail page with ID)
  if (/\/students\/[^\/]+$/.test(pathname)) {
    return {
      title: "Schülerdetails Hilfe",
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

  // Check for database sub-pages
  if (
    pathname === "/database/students" ||
    pathname.startsWith("/database/students")
  ) {
    return {
      title: "Schüler Verwaltung Hilfe",
      content: SPECIFIC_PAGE_HELP["database-students"],
    };
  }

  if (
    pathname === "/database/teachers" ||
    pathname.startsWith("/database/teachers")
  ) {
    return {
      title: "Betreuer Verwaltung Hilfe",
      content: SPECIFIC_PAGE_HELP["database-teachers"],
    };
  }

  if (
    pathname === "/database/rooms" ||
    pathname.startsWith("/database/rooms")
  ) {
    return {
      title: "Räume Verwaltung Hilfe",
      content: SPECIFIC_PAGE_HELP["database-rooms"],
    };
  }

  if (
    pathname === "/database/activities" ||
    pathname.startsWith("/database/activities")
  ) {
    return {
      title: "Aktivitäten Verwaltung Hilfe",
      content: SPECIFIC_PAGE_HELP["database-activities"],
    };
  }

  if (
    pathname === "/database/groups" ||
    pathname.startsWith("/database/groups")
  ) {
    return {
      title: "Gruppen Verwaltung Hilfe",
      content: SPECIFIC_PAGE_HELP["database-groups"],
    };
  }

  if (
    pathname === "/database/roles" ||
    pathname.startsWith("/database/roles")
  ) {
    return {
      title: "Rollen Verwaltung Hilfe",
      content: SPECIFIC_PAGE_HELP["database-roles"],
    };
  }

  if (
    pathname === "/database/devices" ||
    pathname.startsWith("/database/devices")
  ) {
    return {
      title: "Geräte Verwaltung Hilfe",
      content: SPECIFIC_PAGE_HELP["database-devices"],
    };
  }

  if (
    pathname === "/database/permissions" ||
    pathname.startsWith("/database/permissions")
  ) {
    return {
      title: "Berechtigungen Verwaltung Hilfe",
      content: SPECIFIC_PAGE_HELP["database-permissions"],
    };
  }

  // Check for invitations page
  if (pathname === "/invitations" || pathname.startsWith("/invitations")) {
    return {
      title: "Einladungen Hilfe",
      content: SPECIFIC_PAGE_HELP.invitations,
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
