// lib/help-content.tsx
import React from "react";
import type { ReactNode } from "react";

// Reusable component for info list items with title and description
function InfoListItem({
  title,
  description,
}: Readonly<{ title: string; description: string }>) {
  return (
    <div className="flex items-start space-x-3">
      <span className="mt-2 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-gray-400"></span>
      <div>
        <span className="font-medium text-gray-900">{title}</span>
        <p className="text-sm text-gray-600">{description}</p>
      </div>
    </div>
  );
}

// Reusable component for simple bullet point items
function BulletItem({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <div className="flex items-center space-x-2">
      <span className="h-2 w-2 rounded-full bg-gray-400"></span>
      <span>{children}</span>
    </div>
  );
}

// Reusable component for color-coded status bullet items
function StatusBulletItem({
  color,
  children,
}: Readonly<{ color: string; children: ReactNode }>) {
  return (
    <div className="flex items-center space-x-2">
      <span className={`h-2 w-2 rounded-full ${color}`}></span>
      <span>{children}</span>
    </div>
  );
}

// Reusable component for standard CRUD operations list
function CrudOperationsList({ entityName }: Readonly<{ entityName: string }>) {
  return (
    <div className="rounded-lg bg-gray-50 p-4">
      <h4 className="mb-3 font-semibold text-gray-900">
        Verfügbare Operationen
      </h4>
      <div className="space-y-2 text-sm">
        <BulletItem>
          <strong>Anlegen:</strong> Neue {entityName} hinzufügen
        </BulletItem>
        <BulletItem>
          <strong>Bearbeiten:</strong> Bestehende Daten ändern
        </BulletItem>
        <BulletItem>
          <strong>Anzeigen:</strong> Details einsehen
        </BulletItem>
        <BulletItem>
          <strong>Löschen:</strong> Datensätze entfernen
        </BulletItem>
      </div>
    </div>
  );
}

// Reusable component for database management help sections
function DatabaseSectionHelp({
  title,
  description,
  entityName,
}: Readonly<{
  title: string;
  description: string;
  entityName: string;
}>) {
  return (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          {title}
        </h3>
        <p className="leading-relaxed text-gray-700">{description}</p>
      </div>
      <CrudOperationsList entityName={entityName} />
    </div>
  );
}

// Specific page help content
export const SPECIFIC_PAGE_HELP: Record<string, ReactNode> = {
  "student-detail": (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          {"Schülerdetails"}
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
          <InfoListItem
            title="Persönliche Daten"
            description="Name, Klasse, Geburtsdatum und Kontaktinformationen"
          />
          <InfoListItem
            title="Aktueller Status"
            description="Aufenthaltsort und aktuelle Aktivität"
          />
          <InfoListItem
            title="Historie & Verlauf"
            description="Besuchte Räume, Aktivitäten und Feedback"
          />
          <InfoListItem
            title="Erziehungsberechtigte"
            description="Kontaktdaten der Erziehungsberechtigten"
          />
        </div>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">Schnellaktionen</h4>
        <div className="space-y-2 text-sm">
          <BulletItem>Raumbewegungen dokumentieren</BulletItem>
          <BulletItem>Beobachtungen und Notizen hinzufügen</BulletItem>
          <BulletItem>Aufenthaltsorte bei Konflikten klären</BulletItem>
        </div>
      </div>
    </div>
  ),
  feedback_history: (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          {"Feedback Historie"}
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
          <InfoListItem
            title="Pädagogische Beobachtungen"
            description="Wichtige Verhaltensnotizen und Entwicklungen"
          />
          <InfoListItem
            title="Elterngespräche"
            description="Protokolle und Vereinbarungen"
          />
          <InfoListItem
            title="Entwicklungsschritte"
            description="Positive Fortschritte und Erfolge"
          />
        </div>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Verfügbare Funktionen
        </h4>
        <div className="space-y-2 text-sm">
          <BulletItem>Chronologische Darstellung nach Datum</BulletItem>
          <BulletItem>Filter nach Kategorie und Zeitraum</BulletItem>
          <BulletItem>Export für Elterngespräche</BulletItem>
        </div>
      </div>
    </div>
  ),
  mensa_history: (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          {"Mensa Historie"}
        </h3>
        <p className="leading-relaxed text-gray-700">
          Vollständige Übersicht der Mahlzeitenteilnahme, Menüwahlen und
          ernährungsbezogenen Informationen.
        </p>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">Erfasste Daten</h4>
        <div className="grid gap-3">
          <InfoListItem
            title="Teilnahmehistorie"
            description="Tägliche Anwesenheit beim Mittagessen"
          />
          <InfoListItem
            title="Menüwahlen"
            description="Gewählte Mahlzeiten und Vorlieben"
          />
          <InfoListItem
            title="Diätische Hinweise"
            description="Allergien und Unverträglichkeiten"
          />
        </div>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">
          Verwaltungsfunktionen
        </h4>
        <div className="space-y-2 text-sm">
          <BulletItem>Monatsübersicht mit Kalendaransicht</BulletItem>
          <BulletItem>An-/Abmeldung für kommende Tage</BulletItem>
          <BulletItem>Kostenübersicht und Abrechnung</BulletItem>
        </div>
      </div>
    </div>
  ),
  room_history: (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          {"Raum Historie"}
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
          <InfoListItem
            title="Raumwechsel"
            description="Chronologische Auflistung aller besuchten Räume"
          />
          <InfoListItem
            title="Zeitstempel"
            description="Exakte Ein- und Austrittszeiten"
          />
          <InfoListItem
            title="Aktivitätszuordnung"
            description="Verknüpfung zu Aktivitäten und Aufsichtspersonen"
          />
        </div>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">Anwendungsbereiche</h4>
        <div className="space-y-2 text-sm">
          <BulletItem>Bewegungsmuster und Vorlieben analysieren</BulletItem>
          <BulletItem>Konfliktsituationen schnell aufklären</BulletItem>
          <BulletItem>Abholzeiten und Aufenthaltsdauer prüfen</BulletItem>
        </div>
      </div>
    </div>
  ),
  "room-detail": (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          {"Raumdetailansicht"}
        </h3>
        <p className="leading-relaxed text-gray-700">
          Umfassende Informationen zur Raumnutzung, aktueller Belegung und
          detaillierter Nutzungshistorie.
        </p>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">Rauminformationen</h4>
        <div className="grid gap-3">
          <InfoListItem
            title="Basisdaten"
            description="Name, Gebäude und Etage"
          />
          <InfoListItem title="Aktuelle Belegung" description="Live-Status" />
          <InfoListItem
            title="Nutzungshistorie"
            description="Chronologische Übersicht aller Aktivitäten"
          />
        </div>
      </div>
    </div>
  ),
  "database-students": (
    <DatabaseSectionHelp
      title="Schüler Verwaltung"
      description="Verwalte alle Schülerdaten zentral an einem Ort."
      entityName="Schüler"
    />
  ),
  "database-teachers": (
    <DatabaseSectionHelp
      title="Betreuer Verwaltung"
      description="Verwalte alle Betreuerdaten zentral an einem Ort."
      entityName="Betreuer"
    />
  ),
  "database-rooms": (
    <DatabaseSectionHelp
      title="Räume Verwaltung"
      description="Verwalte alle Räume zentral an einem Ort."
      entityName="Räume"
    />
  ),
  "database-activities": (
    <DatabaseSectionHelp
      title="Aktivitäten Verwaltung"
      description="Verwalte alle Aktivitäten zentral an einem Ort."
      entityName="Aktivitäten"
    />
  ),
  "database-groups": (
    <DatabaseSectionHelp
      title="Gruppen Verwaltung"
      description="Verwalte alle Gruppen zentral an einem Ort."
      entityName="Gruppen"
    />
  ),
  "database-roles": (
    <DatabaseSectionHelp
      title="Rollen Verwaltung"
      description="Verwalte alle Benutzerrollen zentral an einem Ort."
      entityName="Rollen"
    />
  ),
  "database-devices": (
    <DatabaseSectionHelp
      title="Geräte Verwaltung"
      description="Verwalte alle IoT-Geräte und RFID-Reader zentral an einem Ort."
      entityName="Geräte"
    />
  ),
  "database-permissions": (
    <DatabaseSectionHelp
      title="Berechtigungen Verwaltung"
      description="Verwalte alle Systemberechtigungen zentral an einem Ort."
      entityName="Berechtigungen"
    />
  ),
  invitations: (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
          <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
          {"Einladungen"}
        </h3>
        <p className="leading-relaxed text-gray-700">
          Verwalte Einladungen für neue Nutzer per E-Mail.
        </p>
      </div>

      <div className="rounded-lg bg-gray-50 p-4">
        <h4 className="mb-3 font-semibold text-gray-900">Funktionen</h4>
        <div className="grid gap-3">
          <InfoListItem
            title="Nutzer einladen"
            description="E-Mail-Einladung an neue Nutzer versenden"
          />
          <InfoListItem
            title="Einladungen einsehen"
            description="Übersicht aller offenen Einladungen"
          />
          <InfoListItem
            title="Einladungen zurückziehen"
            description="Offene Einladungen löschen"
          />
          <InfoListItem
            title="Einladung erneut senden"
            description="E-Mail-Einladung nochmals versenden"
          />
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
            {"Dashboard Übersicht"}
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
            <InfoListItem
              title="Kinder anwesend"
              description="Gesamtzahl der aktuell anwesenden Kinder (klickbar zur Schülersuche)"
            />
            <InfoListItem
              title="In Räumen"
              description="Kinder, die sich gerade in Räumen befinden"
            />
            <InfoListItem
              title="Unterwegs"
              description="Kinder unterwegs von Raum zu Raum"
            />
            <InfoListItem
              title="Schulhof"
              description="Anzahl der Kinder auf dem Schulhof"
            />
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Ressourcenübersicht
          </h4>
          <div className="grid gap-3">
            <InfoListItem
              title="Aktive Gruppen"
              description="Anzahl laufender OGS-Gruppen (klickbar zur Gruppenübersicht)"
            />
            <InfoListItem
              title="Aktive Aktivitäten"
              description="Anzahl laufender Aktivitäten (klickbar zur Aktivitätenübersicht)"
            />
            <InfoListItem
              title="Freie Räume"
              description="Verfügbare Räume (klickbar zur Raumverwaltung)"
            />
            <InfoListItem
              title="Auslastung"
              description="Prozentuale Raumauslastung"
            />
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Live-Übersichten</h4>
          <div className="space-y-2 text-sm">
            <BulletItem>
              <strong>Letzte Bewegungen:</strong> Aktuelle Raumwechsel von
              Gruppen
            </BulletItem>
            <BulletItem>
              <strong>Laufende Aktivitäten:</strong> Details zu aktuellen
              Aktivitäten mit Teilnehmerzahl
            </BulletItem>
            <BulletItem>
              <strong>Aktive Gruppen:</strong> Übersicht aller aktiven Gruppen
              mit Standort
            </BulletItem>
            <BulletItem>
              <strong>Personal heute:</strong> Betreuer im Dienst
            </BulletItem>
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
            {"OGS Gruppenübersicht"}
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
            <InfoListItem
              title="Schüler verfolgen"
              description="Sehen Sie in Echtzeit, wo sich alle Kinder befinden"
            />
            <InfoListItem
              title="Live-Updates"
              description="Automatische Aktualisierung bei Standortwechseln (Status-Indikator oben)"
            />
            <InfoListItem
              title="Mehrere Gruppen"
              description="Bei Leitung mehrerer Gruppen: Schneller Wechsel per Tabs"
            />
            <InfoListItem
              title="Schüler-Details"
              description="Auf Karte tippen für vollständige Schüler-Kartei"
            />
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Filter & Suche</h4>
          <div className="grid gap-3">
            <InfoListItem
              title="Namenssuche"
              description="Schnelle Suche nach Vor- oder Nachname"
            />
            <InfoListItem
              title="Klassenstufe"
              description="Filtern nach Jahrgang (1, 2, 3, 4)"
            />
            <InfoListItem
              title="Aufenthaltsort-Filter"
              description="6 Filteroptionen für verschiedene Standorte"
            />
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Aufenthaltsorte</h4>
          <div className="space-y-2 text-sm">
            <StatusBulletItem color="bg-emerald-500">
              <strong>Gruppenraum:</strong> Kind befindet sich im zugewiesenen
              OGS-Raum
            </StatusBulletItem>
            <StatusBulletItem color="bg-blue-500">
              <strong>Fremder Raum:</strong> Kind ist in einem anderen Raum
            </StatusBulletItem>
            <StatusBulletItem color="bg-fuchsia-500">
              <strong>Unterwegs:</strong> Kind wechselt gerade den Raum oder ist
              in Bewegung
            </StatusBulletItem>
            <StatusBulletItem color="bg-yellow-500">
              <strong>Schulhof:</strong> Kind ist auf dem Außengelände
            </StatusBulletItem>
            <StatusBulletItem color="bg-red-500">
              <strong>Zuhause:</strong> Kind wurde abgeholt und ist zuhause
            </StatusBulletItem>
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
            {"Aktuelle Aufsicht Übersicht"}
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
            <InfoListItem
              title="Namenssuche"
              description="Schnelle Suche nach Vor- oder Nachname"
            />
            <InfoListItem
              title="Gruppen-Filter"
              description="Filtern nach OGS-Gruppe"
            />
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
            {"Schülersuche"}
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
            <InfoListItem
              title="Kinder deiner OGS-Gruppe"
              description="Vollständige Standorte sichtbar"
            />
            <InfoListItem
              title="Fremde Kinder"
              description="Nur Anwesenheitsstatus: Anwesend (in der OGS), Zuhause (krank oder abgeholt)"
            />
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Filter</h4>
          <div className="grid gap-3">
            <InfoListItem
              title="Namenssuche"
              description="Schnelle Suche nach Vor- oder Nachname"
            />
            <InfoListItem
              title="Gruppen & Jahrgänge"
              description="Filtern nach OGS-Gruppe oder Klassenstufe"
            />
            <InfoListItem
              title="Anwesenheitsstatus"
              description="Filtern nach Aufenthaltsort"
            />
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
            {"Raumverwaltung & Belegung"}
          </h3>
          <p className="leading-relaxed text-gray-700">
            Verwalten Sie alle Räume effizient und behalten Sie den Überblick
            über Belegungen und Verfügbarkeiten.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Grundfunktionen</h4>
          <div className="grid gap-3">
            <InfoListItem
              title="Räume einsehen"
              description="Übersicht aller verfügbaren Räume mit Details"
            />
            <InfoListItem
              title="Belegung prüfen"
              description="Aktuelle und geplante Raumbelegungen in Echtzeit"
            />
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
            {"Aktivitäten"}
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
            <InfoListItem
              title="Aktivitäten einsehen"
              description="Übersicht aller Aktivitäten und AGs"
            />
            <InfoListItem
              title="Aktivitäten erstellen"
              description="Neue Aktivitäten hinzufügen"
            />
            <InfoListItem
              title="Aktivitäten bearbeiten"
              description="Bestehende Aktivitäten anpassen"
            />
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
            {"Statistiken & Auswertungen"}
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
            <InfoListItem
              title="Anwesenheitsstatistiken"
              description="Tägliche, wöchentliche und monatliche Übersichten"
            />
            <InfoListItem
              title="Gruppenzahlen"
              description="Auslastung der OGS-Gruppen und Kapazitäten"
            />
            <InfoListItem
              title="Aktivitätenanalyse"
              description="Beliebtheit und Teilnehmerzahlen der Angebote"
            />
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Funktionen & Tools
          </h4>
          <div className="space-y-2 text-sm">
            <BulletItem>
              <strong>Exportieren:</strong> Daten als Excel oder PDF
              herunterladen
            </BulletItem>
            <BulletItem>
              <strong>Zeitraumfilter:</strong> Daten für bestimmte Zeiträume
              anzeigen
            </BulletItem>
            <BulletItem>
              <strong>Vergleichsfunktion:</strong> Daten verschiedener Zeiträume
              vergleichen
            </BulletItem>
            <BulletItem>
              <strong>Diagramme:</strong> Visuelle Darstellung wichtiger
              Kennzahlen
            </BulletItem>
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
            {"Vertretungsmanagement"}
          </h3>
          <p className="leading-relaxed text-gray-700">
            Organisieren Sie Personalausfälle und Vertretungen effizient, um
            eine durchgängige Betreuung sicherzustellen.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Funktionen</h4>
          <div className="grid gap-3">
            <InfoListItem
              title="Vertretungen planen"
              description="Ersatzpersonal für abwesende Betreuer einteilen"
            />
            <InfoListItem
              title="Fachkräfte zuweisen"
              description="Pädagogische Fachkräfte für beliebige Tage zuweisen"
            />
            <InfoListItem
              title="Vertretungen entfernen"
              description="Zugewiesene Vertretungen bei Bedarf wieder entfernen"
            />
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
            {"Datenverwaltung"}
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
            <InfoListItem
              title="Schüler"
              description="Schülerdaten verwalten und bearbeiten"
            />
            <InfoListItem
              title="Betreuer"
              description="Daten der Betreuer verwalten"
            />
            <InfoListItem title="Räume" description="Räume verwalten" />
            <InfoListItem
              title="Aktivitäten"
              description="Aktivitäten verwalten"
            />
            <InfoListItem title="Gruppen" description="Gruppen verwalten" />
            <InfoListItem
              title="Rollen"
              description="Benutzerrollen und Berechtigungen verwalten"
            />
            <InfoListItem
              title="Geräte"
              description="IoT-Geräte und RFID-Reader verwalten"
            />
            <InfoListItem
              title="Berechtigungen"
              description="Systemberechtigungen ansehen"
            />
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
            {"Personalübersicht"}
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
            <InfoListItem
              title="Mitarbeiterübersicht"
              description="Alle pädagogischen Fachkräfte auf einen Blick"
            />
            <InfoListItem
              title="Einsatzorte"
              description="Aktueller Aufenthaltsort und betreute Räume/Gruppen"
            />
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Statusanzeigen</h4>
          <div className="space-y-2 text-sm">
            <BulletItem>
              <strong>Zuhause:</strong> Mitarbeiter ist nicht im Dienst
            </BulletItem>
            <BulletItem>
              <strong>Im Raum:</strong> Mitarbeiter betreut einen spezifischen
              Raum
            </BulletItem>
            <BulletItem>
              <strong>Schulhof:</strong> Mitarbeiter beaufsichtigt den
              Außenbereich
            </BulletItem>
            <BulletItem>
              <strong>Unterwegs:</strong> Mitarbeiter ist zwischen Räumen oder
              in Bewegung
            </BulletItem>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Funktionen</h4>
          <div className="space-y-2 text-sm">
            <BulletItem>
              <strong>Suche:</strong> Mitarbeiter nach Namen suchen
            </BulletItem>
            <BulletItem>
              <strong>Filter:</strong> Nach Aufenthaltsort filtern
            </BulletItem>
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
            {"Einstellungen"}
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
            <InfoListItem
              title="Profil"
              description="Vor- und Nachname ändern"
            />
            <InfoListItem title="Sicherheit" description="Passwort ändern" />
          </div>
        </div>
      </div>
    ),
  },

  "/suggestions": {
    title: "Feedback Hilfe",
    content: (
      <div className="space-y-6">
        <div>
          <h3 className="mb-3 flex items-center text-lg font-semibold text-gray-900">
            <span className="mr-3 h-2 w-2 rounded-full bg-blue-500"></span>
            {"Feedback & Vorschläge"}
          </h3>
          <p className="leading-relaxed text-gray-700">
            Das Feedback-Board ermöglicht es allen Mitarbeitenden, Ideen und
            Verbesserungsvorschläge einzubringen und über bestehende Vorschläge
            abzustimmen.
          </p>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Vorschläge erstellen & verwalten
          </h4>
          <div className="space-y-2 text-sm">
            <BulletItem>
              <strong>Neuer Beitrag:</strong> Erstelle einen Vorschlag mit Titel
              und Beschreibung
            </BulletItem>
            <BulletItem>
              <strong>Bearbeiten:</strong> Eigene Beiträge nachträglich ändern
            </BulletItem>
            <BulletItem>
              <strong>Löschen:</strong> Eigene Beiträge entfernen
            </BulletItem>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Abstimmung</h4>
          <div className="space-y-2 text-sm">
            <BulletItem>
              <strong>Daumen hoch/runter:</strong> Stimme für oder gegen einen
              Vorschlag
            </BulletItem>
            <BulletItem>
              <strong>Stimme ändern:</strong> Klicke auf die andere Richtung, um
              deine Stimme zu wechseln
            </BulletItem>
            <BulletItem>
              <strong>Stimme zurücknehmen:</strong> Klicke erneut auf deine
              aktive Stimme, um sie zu entfernen
            </BulletItem>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">
            Sortierung & Filter
          </h4>
          <div className="space-y-2 text-sm">
            <BulletItem>
              <strong>Beliebteste:</strong> Sortiert nach Bewertung (Upvotes
              minus Downvotes)
            </BulletItem>
            <BulletItem>
              <strong>Neueste:</strong> Sortiert nach Erstellungsdatum
            </BulletItem>
            <BulletItem>
              <strong>Suchfeld:</strong> Durchsuche Titel, Beschreibung und
              Autorname
            </BulletItem>
          </div>
        </div>

        <div className="rounded-lg bg-gray-50 p-4">
          <h4 className="mb-3 font-semibold text-gray-900">Status-Anzeige</h4>
          <div className="space-y-2 text-sm">
            <StatusBulletItem color="bg-gray-400">
              <strong>Offen:</strong> Neuer Vorschlag, noch nicht bearbeitet
            </StatusBulletItem>
            <StatusBulletItem color="bg-blue-500">
              <strong>Geplant:</strong> Vorschlag wird zur Umsetzung eingeplant
            </StatusBulletItem>
            <StatusBulletItem color="bg-green-500">
              <strong>Umgesetzt:</strong> Vorschlag wurde erfolgreich umgesetzt
            </StatusBulletItem>
            <StatusBulletItem color="bg-red-500">
              <strong>Abgelehnt:</strong> Vorschlag wird nicht umgesetzt
            </StatusBulletItem>
          </div>
        </div>
      </div>
    ),
  },
};

// Pattern-based routes for specific pages with dynamic IDs
const PATTERN_ROUTES: ReadonlyArray<{
  pattern: RegExp;
  title: string;
  key: keyof typeof SPECIFIC_PAGE_HELP;
}> = [
  {
    pattern: /\/students\/[^/]+$/,
    title: "Schülerdetails Hilfe",
    key: "student-detail",
  },
  {
    pattern: /\/students\/[^/]+\/feedback_history/,
    title: "Feedback Historie Hilfe",
    key: "feedback_history",
  },
  {
    pattern: /\/students\/[^/]+\/mensa_history/,
    title: "Mensa Historie Hilfe",
    key: "mensa_history",
  },
  {
    pattern: /\/students\/[^/]+\/room_history/,
    title: "Raum Historie Hilfe",
    key: "room_history",
  },
  { pattern: /\/rooms\/[^/]+$/, title: "Raumdetail Hilfe", key: "room-detail" },
];

// Database sub-routes mapping
const DATABASE_ROUTES: ReadonlyArray<{
  prefix: string;
  title: string;
  key: keyof typeof SPECIFIC_PAGE_HELP;
}> = [
  {
    prefix: "/database/students",
    title: "Schüler Verwaltung Hilfe",
    key: "database-students",
  },
  {
    prefix: "/database/teachers",
    title: "Betreuer Verwaltung Hilfe",
    key: "database-teachers",
  },
  {
    prefix: "/database/rooms",
    title: "Räume Verwaltung Hilfe",
    key: "database-rooms",
  },
  {
    prefix: "/database/activities",
    title: "Aktivitäten Verwaltung Hilfe",
    key: "database-activities",
  },
  {
    prefix: "/database/groups",
    title: "Gruppen Verwaltung Hilfe",
    key: "database-groups",
  },
  {
    prefix: "/database/roles",
    title: "Rollen Verwaltung Hilfe",
    key: "database-roles",
  },
  {
    prefix: "/database/devices",
    title: "Geräte Verwaltung Hilfe",
    key: "database-devices",
  },
  {
    prefix: "/database/permissions",
    title: "Berechtigungen Verwaltung Hilfe",
    key: "database-permissions",
  },
];

// Default help content shown when no route matches
const DEFAULT_HELP_CONTENT: { title: string; content: ReactNode } = {
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

/**
 * Match pathname against pattern-based routes (regex matching)
 */
function matchPatternRoute(
  pathname: string,
): { title: string; content: ReactNode } | null {
  for (const route of PATTERN_ROUTES) {
    if (route.pattern.test(pathname)) {
      return { title: route.title, content: SPECIFIC_PAGE_HELP[route.key] };
    }
  }
  return null;
}

/**
 * Match pathname against database sub-routes (prefix matching)
 */
function matchDatabaseRoute(
  pathname: string,
): { title: string; content: ReactNode } | null {
  for (const route of DATABASE_ROUTES) {
    if (pathname.startsWith(route.prefix)) {
      return { title: route.title, content: SPECIFIC_PAGE_HELP[route.key] };
    }
  }
  return null;
}

/**
 * Match pathname against navigation routes
 */
function matchNavigationRoute(
  pathname: string,
): { title: string; content: ReactNode } | null {
  for (const [route, helpData] of Object.entries(NAVIGATION_HELP)) {
    const isMatch =
      route === "/dashboard"
        ? pathname === "/dashboard"
        : pathname.startsWith(route);
    if (isMatch) {
      return helpData;
    }
  }
  return null;
}

// Function to get help content based on current pathname
export function getHelpContent(pathname: string): {
  title: string;
  content: ReactNode;
} {
  // Check for /students/search FIRST (before the general /students/[id] pattern)
  if (pathname.startsWith("/students/search")) {
    return NAVIGATION_HELP["/students"]!;
  }

  // Check pattern routes (regex matching for dynamic paths)
  const patternMatch = matchPatternRoute(pathname);
  if (patternMatch) return patternMatch;

  // Check database sub-routes
  const dbMatch = matchDatabaseRoute(pathname);
  if (dbMatch) return dbMatch;

  // Check invitations page
  if (pathname.startsWith("/invitations")) {
    return {
      title: "Einladungen Hilfe",
      content: SPECIFIC_PAGE_HELP.invitations,
    };
  }

  // Check main navigation routes
  const navMatch = matchNavigationRoute(pathname);
  if (navMatch) return navMatch;

  // Default help content
  return DEFAULT_HELP_CONTENT;
}
