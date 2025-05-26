"use client";

import { useEffect, useState, useCallback, use } from "react";
import { useRouter } from "next/navigation";
import { authService } from "@/lib/auth-service";
import { PageHeader } from "@/components/dashboard";
import { Button, Input, Card } from "@/components/ui";
import type { Permission } from "@/lib/auth-helpers";

export default function PermissionDetailsPage({ params }: { params: Promise<{ id: string }> }) {
  // Use React.use to unwrap the params promise
  const resolvedParams = use(params);
  const router = useRouter();
  const [permission, setPermission] = useState<Permission | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isEditing, setIsEditing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [formData, setFormData] = useState({
    name: "",
    description: "",
    resource: "",
    action: "",
  });

  const loadPermissionData = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      
      const permissionData = await authService.getPermission(resolvedParams.id);
      
      setPermission(permissionData);
      setFormData({
        name: permissionData.name,
        description: permissionData.description || "",
        resource: permissionData.resource,
        action: permissionData.action,
      });
    } catch (err) {
      setError("Fehler beim Laden der Berechtigung");
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  }, [resolvedParams.id]);

  useEffect(() => {
    void loadPermissionData();
  }, [resolvedParams.id, loadPermissionData]);

  const handleSave = async () => {
    if (!permission) return;
    
    try {
      setError(null);
      await authService.updatePermission(permission.id, formData);
      await loadPermissionData();
      setIsEditing(false);
    } catch (err) {
      setError("Fehler beim Speichern der Berechtigung");
      console.error(err);
    }
  };

  const handleDelete = async () => {
    if (!permission || !confirm("Möchten Sie diese Berechtigung wirklich löschen?")) return;
    
    try {
      await authService.deletePermission(permission.id);
      router.push("/database/permissions");
    } catch (err) {
      setError("Fehler beim Löschen der Berechtigung");
      console.error(err);
    }
  };

  if (isLoading) {
    return (
      <div className="min-h-screen">
        <PageHeader title="Berechtigung wird geladen..." backUrl="/database/permissions" />
        <main className="container mx-auto p-4 py-8">
          <div className="flex justify-center">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
          </div>
        </main>
      </div>
    );
  }

  if (!permission) {
    return (
      <div className="min-h-screen">
        <PageHeader title="Berechtigung nicht gefunden" backUrl="/database/permissions" />
        <main className="container mx-auto p-4 py-8">
          <div className="text-center text-red-600">
            Die angeforderte Berechtigung konnte nicht gefunden werden.
          </div>
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      <PageHeader
        title={isEditing ? `Berechtigung "${permission.name}" bearbeiten` : `Berechtigung: ${permission.name}`}
        backUrl="/database/permissions"
      />

      <main className="container mx-auto p-4 py-8">
        {error && (
          <div className="bg-red-50 border border-red-200 text-red-600 px-4 py-3 rounded-md mb-6">
            {error}
          </div>
        )}

        <Card className="max-w-2xl mx-auto">
          <div className="p-6">
            <div className="flex justify-between items-center mb-6">
              <h2 className="text-xl font-semibold">Berechtigungsdetails</h2>
              <div className="flex gap-2">
                <Button
                  variant={isEditing ? "primary" : "outline"}
                  onClick={() => {
                    if (isEditing) {
                      void handleSave();
                    } else {
                      setIsEditing(true);
                    }
                  }}
                >
                  {isEditing ? "Speichern" : "Bearbeiten"}
                </Button>
                <Button
                  variant="danger"
                  onClick={handleDelete}
                >
                  Löschen
                </Button>
              </div>
            </div>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Name
                </label>
                {isEditing ? (
                  <Input
                    label="Name"
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  />
                ) : (
                  <div className="px-3 py-2 border border-gray-200 rounded-md bg-gray-50">
                    {permission.name}
                  </div>
                )}
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Beschreibung
                </label>
                {isEditing ? (
                  <textarea
                    value={formData.description}
                    onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                    rows={3}
                  />
                ) : (
                  <div className="px-3 py-2 border border-gray-200 rounded-md bg-gray-50">
                    {permission.description || "-"}
                  </div>
                )}
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Resource
                </label>
                {isEditing ? (
                  <Input
                    label="Resource"
                    value={formData.resource}
                    onChange={(e) => setFormData({ ...formData, resource: e.target.value })}
                  />
                ) : (
                  <div className="px-3 py-2 border border-gray-200 rounded-md bg-gray-50">
                    {permission.resource}
                  </div>
                )}
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Action
                </label>
                {isEditing ? (
                  <Input
                    label="Action"
                    value={formData.action}
                    onChange={(e) => setFormData({ ...formData, action: e.target.value })}
                  />
                ) : (
                  <div className="px-3 py-2 border border-gray-200 rounded-md bg-gray-50">
                    {permission.action}
                  </div>
                )}
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Erstellt am
                  </label>
                  <div className="px-3 py-2 border border-gray-200 rounded-md bg-gray-50">
                    {new Date(permission.createdAt).toLocaleString("de-DE")}
                  </div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Aktualisiert am
                  </label>
                  <div className="px-3 py-2 border border-gray-200 rounded-md bg-gray-50">
                    {new Date(permission.updatedAt).toLocaleString("de-DE")}
                  </div>
                </div>
              </div>
            </div>

            {isEditing && (
              <div className="mt-6">
                <Button
                  variant="outline"
                  onClick={() => {
                    setIsEditing(false);
                    setFormData({
                      name: permission.name,
                      description: permission.description || "",
                      resource: permission.resource,
                      action: permission.action,
                    });
                  }}
                  className="w-full"
                >
                  Abbrechen
                </Button>
              </div>
            )}
          </div>
        </Card>
      </main>
    </div>
  );
}