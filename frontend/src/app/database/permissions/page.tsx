"use client";

import { useEffect, useState } from "react";
import { PageHeader } from "@/components/dashboard";
import { authService } from "@/lib/auth-service";
import type { Permission } from "@/lib/auth-helpers";

export default function PermissionsPage() {
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState("");

  const loadPermissions = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await authService.getPermissions();
      
      // Ensure we have a valid array before setting
      if (Array.isArray(data)) {
        setPermissions(data);
      } else {
        setPermissions([]);
      }
    } catch (err) {
      setError("Fehler beim Laden der Berechtigungen");
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void loadPermissions();
  }, []);

  // Filter permissions based on search term
  const filteredPermissions = permissions.filter((permission) => {
    // Safety check for permission structure
    if (!permission || typeof permission !== 'object') {
      return false;
    }
    
    // Check for required properties
    if (!permission.name || !permission.description || !permission.resource || !permission.action) {
      return false;
    }
    
    return (
      permission.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      permission.description.toLowerCase().includes(searchTerm.toLowerCase()) ||
      permission.resource.toLowerCase().includes(searchTerm.toLowerCase()) ||
      permission.action.toLowerCase().includes(searchTerm.toLowerCase())
    );
  });

  return (
    <div className="min-h-screen bg-gray-50">
      <PageHeader
        title="System-Berechtigungen"
        backUrl="/dashboard"
      />

      <main className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-8">
        {/* Introduction */}
        <div className="mb-8 rounded-lg bg-blue-50 border border-blue-200 p-6">
          <h2 className="text-lg font-semibold text-blue-900 mb-2">
            Über System-Berechtigungen
          </h2>
          <p className="text-blue-800">
            Diese Seite zeigt alle im System definierten Berechtigungen. Diese sind fest im Backend implementiert 
            und können nicht über die Benutzeroberfläche geändert werden. Berechtigungen werden Rollen zugewiesen, 
            die wiederum Benutzern zugeordnet werden können.
          </p>
        </div>

        {/* Search Bar */}
        <div className="mb-6">
          <input
            type="text"
            placeholder="Berechtigungen suchen..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full max-w-md rounded-lg border border-gray-300 px-4 py-2 focus:border-blue-500 focus:outline-none"
          />
        </div>

        {/* Loading State */}
        {isLoading && (
          <div className="text-center py-12">
            <span className="text-gray-500">Lade Berechtigungen...</span>
          </div>
        )}

        {/* Error State */}
        {error && (
          <div className="mb-6 rounded-lg bg-red-50 border border-red-200 p-4">
            <p className="text-red-800">{error}</p>
          </div>
        )}

        {/* Permissions Grid */}
        {!isLoading && !error && (
          <div className="bg-white shadow overflow-hidden sm:rounded-lg">
            <div className="px-4 py-5 sm:px-6">
              <h3 className="text-lg leading-6 font-medium text-gray-900">
                Verfügbare Berechtigungen ({filteredPermissions.length})
              </h3>
            </div>
            <div className="border-t border-gray-200">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Name
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Beschreibung
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Resource
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      Action
                    </th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {filteredPermissions.map((permission) => (
                    <tr key={permission.id} className="hover:bg-gray-50">
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className="font-mono text-sm text-gray-900">
                          {permission.name}
                        </span>
                      </td>
                      <td className="px-6 py-4">
                        <span className="text-sm text-gray-600">
                          {permission.description}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-gray-100 text-gray-800">
                          {permission.resource}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-blue-100 text-blue-800">
                          {permission.action}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              
              {filteredPermissions.length === 0 && (
                <div className="text-center py-12">
                  <p className="text-gray-500">
                    {searchTerm
                      ? "Keine Berechtigungen gefunden, die Ihrer Suche entsprechen."
                      : "Keine Berechtigungen vorhanden."}
                  </p>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Permission Groups Explanation */}
        <div className="mt-8 grid gap-6 md:grid-cols-2">
          <div className="bg-white overflow-hidden shadow rounded-lg">
            <div className="px-6 py-4 bg-green-50">
              <h3 className="text-lg font-medium text-green-900">Resources</h3>
            </div>
            <div className="px-6 py-4">
              <dl className="space-y-3">
                <div>
                  <dt className="text-sm font-medium text-gray-500">users</dt>
                  <dd className="text-sm text-gray-900">Verwaltung von Benutzerkonten, Rollen und Zugriffsrechten</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">roles, permissions, auth</dt>
                  <dd className="text-sm text-gray-900">Rollenverwaltung, Berechtigungen und Authentifizierung</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">activities</dt>
                  <dd className="text-sm text-gray-900">AGs und Nachmittagsaktivitäten für Schüler</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">rooms</dt>
                  <dd className="text-sm text-gray-900">Räume und Klassenzimmer im Schulgebäude</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">groups</dt>
                  <dd className="text-sm text-gray-900">Schulklassen und Schülergruppen</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">visits</dt>
                  <dd className="text-sm text-gray-900">Anwesenheitsverfolgung von Schülern in Räumen</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">schedules</dt>
                  <dd className="text-sm text-gray-900">Zeitpläne und Terminverwaltung</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">config</dt>
                  <dd className="text-sm text-gray-900">Systemkonfiguration und Einstellungen</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">feedback</dt>
                  <dd className="text-sm text-gray-900">Rückmeldungen und Kommentare</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">iot</dt>
                  <dd className="text-sm text-gray-900">RFID-Leser und IoT-Geräte</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">system, admin</dt>
                  <dd className="text-sm text-gray-900">Systemverwaltung und Administratorrechte</dd>
                </div>
              </dl>
            </div>
          </div>

          <div className="bg-white overflow-hidden shadow rounded-lg">
            <div className="px-6 py-4 bg-blue-50">
              <h3 className="text-lg font-medium text-blue-900">Actions</h3>
            </div>
            <div className="px-6 py-4">
              <dl className="space-y-3">
                <div>
                  <dt className="text-sm font-medium text-gray-500">create</dt>
                  <dd className="text-sm text-gray-900">Neue Einträge erstellen</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">read</dt>
                  <dd className="text-sm text-gray-900">Daten anzeigen und einsehen</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">update</dt>
                  <dd className="text-sm text-gray-900">Bestehende Daten bearbeiten</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">delete</dt>
                  <dd className="text-sm text-gray-900">Einträge löschen</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">list</dt>
                  <dd className="text-sm text-gray-900">Übersichten und Listen abrufen</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">manage</dt>
                  <dd className="text-sm text-gray-900">Volle Kontrolle über die Resource</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">assign</dt>
                  <dd className="text-sm text-gray-900">Zuweisungen vornehmen (z.B. Schüler zu Gruppen)</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">enroll</dt>
                  <dd className="text-sm text-gray-900">Schüler in Aktivitäten einschreiben</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">*</dt>
                  <dd className="text-sm text-gray-900">Alle Aktionen (Administratorrechte)</dd>
                </div>
              </dl>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}