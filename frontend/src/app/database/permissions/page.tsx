"use client";

import { useEffect, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "@/components/dashboard";
import { DatabasePageHeader, SearchFilter, DatabaseListSection } from "@/components/ui";
import { SelectFilter } from "@/components/ui";
import { PermissionListItem } from "@/components/auth";
import { authService } from "@/lib/auth-service";
import type { Permission } from "@/lib/auth-helpers";

export default function PermissionsPage() {
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState("");
  const [resourceFilter, setResourceFilter] = useState<string | null>(null);
  const [actionFilter, setActionFilter] = useState<string | null>(null);

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Function to fetch permissions
  const fetchPermissions = async () => {
    try {
      setLoading(true);
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
      setLoading(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchPermissions();
  }, []);

  if (status === "loading") {
    return <div />;
  }

  // Get unique resources from loaded permissions
  const resourceOptions = Array.from(
    new Set(permissions.map(p => p.resource))
  ).sort().map(resource => ({
    value: resource,
    label: resource
  }));

  // Get unique actions from loaded permissions
  const actionOptions = Array.from(
    new Set(permissions.map(p => p.action))
  ).sort().map(action => ({
    value: action,
    label: action
  }));

  // Filter permissions based on search term and filters
  const filteredPermissions = permissions.filter((permission) => {
    // Safety check for permission structure
    if (!permission || typeof permission !== 'object') {
      return false;
    }
    
    // Check for required properties
    if (!permission.name || !permission.description || !permission.resource || !permission.action) {
      return false;
    }
    
    // Apply filters
    if (resourceFilter && permission.resource !== resourceFilter) return false;
    if (actionFilter && permission.action !== actionFilter) return false;
    
    // Apply search
    if (searchFilter) {
      const searchLower = searchFilter.toLowerCase();
      return (
        permission.name.toLowerCase().includes(searchLower) ||
        permission.description.toLowerCase().includes(searchLower) ||
        permission.resource.toLowerCase().includes(searchLower) ||
        permission.action.toLowerCase().includes(searchLower)
      );
    }
    
    return true;
  });

  // Group permissions by resource for display
  const groupedPermissions = filteredPermissions.reduce((acc, permission) => {
    const resource = permission.resource;
    acc[resource] ??= [];
    acc[resource].push(permission);
    return acc;
  }, {} as Record<string, Permission[]>);

  // Sort grouped permissions
  const sortedGroups = Object.entries(groupedPermissions)
    .sort(([a], [b]) => a.localeCompare(b));

  // Render filters
  const renderFilters = () => (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-2 lg:gap-4">
      <SelectFilter
        id="resourceFilter"
        label="Resource"
        value={resourceFilter}
        onChange={setResourceFilter}
        options={resourceOptions}
        placeholder="Alle Resources"
      />
      <SelectFilter
        id="actionFilter"
        label="Action"
        value={actionFilter}
        onChange={setActionFilter}
        options={actionOptions}
        placeholder="Alle Actions"
      />
    </div>
  );

  // Loading state
  if (loading) {
    return (
      <ResponsiveLayout>
        <div className="max-w-7xl mx-auto p-4 md:p-6 lg:p-8 pb-24 lg:pb-8">
          <div className="flex flex-col items-center justify-center py-12 md:py-16">
            <div className="flex flex-col items-center gap-4">
              <div className="h-10 w-10 md:h-12 md:w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
              <p className="text-sm md:text-base text-gray-600">Daten werden geladen...</p>
            </div>
          </div>
        </div>
      </ResponsiveLayout>
    );
  }

  // Error state
  if (error) {
    return (
      <ResponsiveLayout>
        <div className="max-w-7xl mx-auto p-4 md:p-6 lg:p-8 pb-24 lg:pb-8">
          <div className="flex flex-col items-center justify-center py-8 md:py-12">
            <div className="max-w-md w-full rounded-lg bg-red-50 p-4 md:p-6 text-red-800 shadow-md">
              <h2 className="mb-2 text-lg md:text-xl font-semibold">Fehler</h2>
              <p className="text-sm md:text-base">{error}</p>
              <button
                onClick={fetchPermissions}
                className="mt-4 w-full md:w-auto rounded-lg bg-red-100 px-4 py-2 text-sm md:text-base text-red-800 transition-colors hover:bg-red-200 active:scale-[0.98]"
              >
                Erneut versuchen
              </button>
            </div>
          </div>
        </div>
      </ResponsiveLayout>
    );
  }

  return (
    <ResponsiveLayout>
      <div className="max-w-7xl mx-auto p-4 md:p-6 lg:p-8 pb-24 lg:pb-8">
        <DatabasePageHeader 
          title="System-Berechtigungen" 
          description="Übersicht aller Systemberechtigungen"
        />
        
        <SearchFilter
          searchPlaceholder="Berechtigungen suchen..."
          searchValue={searchFilter}
          onSearchChange={setSearchFilter}
          filters={renderFilters()}
          addButton={undefined}
        />

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
        
        <DatabaseListSection 
          title={`Berechtigungen (${filteredPermissions.length} von ${permissions.length})`}
          itemCount={filteredPermissions.length}
          itemLabel={{ singular: "Berechtigung", plural: "Berechtigungen" }}
        >
          {sortedGroups.length > 0 ? (
            <div className="space-y-8">
              {sortedGroups.map(([resource, perms]) => (
                <div key={resource}>
                  <h4 className="text-sm font-semibold text-gray-700 uppercase tracking-wider mb-3">
                    {resource} ({perms.length})
                  </h4>
                  <div className="space-y-3">
                    {perms.sort((a, b) => a.name.localeCompare(b.name)).map((permission) => (
                      <PermissionListItem key={permission.id} permission={permission} />
                    ))}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="py-8 md:py-12 text-center">
              <div className="flex flex-col items-center gap-4">
                <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                </svg>
                <div>
                  <h3 className="text-base md:text-lg font-medium text-gray-900 mb-1">
                    {searchFilter || resourceFilter || actionFilter
                      ? "Keine Ergebnisse gefunden"
                      : "Keine Berechtigungen vorhanden"}
                  </h3>
                  <p className="text-sm text-gray-600">
                    {searchFilter || resourceFilter || actionFilter
                      ? "Versuchen Sie einen anderen Suchbegriff."
                      : "Es sind keine Berechtigungen definiert."}
                  </p>
                </div>
              </div>
            </div>
          )}
        </DatabaseListSection>

        {/* Permission Groups Explanation */}
        <div className="mt-12 grid gap-6 md:grid-cols-2">
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
      </div>
    </ResponsiveLayout>
  );
}