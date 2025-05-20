"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { authService } from "@/lib/auth-service";
import { PageHeader } from "@/components/dashboard";
import { Button, Input, Card } from "@/components/ui";
import type { Role, Permission } from "@/lib/auth-helpers";
import Link from "next/link";

import { use } from 'react';

export default function RoleDetailsPage({ params }: { params: Promise<{ id: string }> }) {
  // Use React.use to unwrap the params promise
  const resolvedParams = use(params);
  const router = useRouter();
  const [role, setRole] = useState<Role | null>(null);
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [rolePermissions, setRolePermissions] = useState<Permission[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isEditing, setIsEditing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [formData, setFormData] = useState({
    name: "",
    description: "",
  });

  const loadRoleData = async () => {
    try {
      setIsLoading(true);
      setError(null);
      
      const [roleData, allPermissions, assignedPermissions] = await Promise.all([
        authService.getRole(resolvedParams.id),
        authService.getPermissions(),
        authService.getRolePermissions(resolvedParams.id),
      ]);
      
      setRole(roleData);
      setPermissions(allPermissions);
      setRolePermissions(assignedPermissions);
      setFormData({
        name: roleData.name,
        description: roleData.description || "",
      });
    } catch (err) {
      setError("Fehler beim Laden der Rolle");
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void loadRoleData();
  }, [resolvedParams.id]);

  const handleSave = async () => {
    if (!role) return;
    
    try {
      setError(null);
      await authService.updateRole(role.id, formData);
      await loadRoleData();
      setIsEditing(false);
    } catch (err) {
      setError("Fehler beim Speichern der Rolle");
      console.error(err);
    }
  };

  const handlePermissionToggle = async (permission: Permission) => {
    if (!role) return;
    
    try {
      const isAssigned = rolePermissions.some(p => p.id === permission.id);
      
      if (isAssigned) {
        await authService.removePermissionFromRole(role.id, permission.id);
      } else {
        await authService.assignPermissionToRole(role.id, permission.id);
      }
      
      await loadRoleData();
    } catch (err) {
      setError("Fehler beim Ändern der Berechtigungen");
      console.error(err);
    }
  };

  if (isLoading) {
    return (
      <div className="min-h-screen">
        <PageHeader title="Rolle wird geladen..." backUrl="/database/roles" />
        <main className="container mx-auto p-4 py-8">
          <div className="flex justify-center">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
          </div>
        </main>
      </div>
    );
  }

  if (!role) {
    return (
      <div className="min-h-screen">
        <PageHeader title="Rolle nicht gefunden" backUrl="/database/roles" />
        <main className="container mx-auto p-4 py-8">
          <div className="text-center text-red-600">
            Die angeforderte Rolle konnte nicht gefunden werden.
          </div>
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      <PageHeader
        title={isEditing ? `Rolle "${role.name}" bearbeiten` : `Rolle: ${role.name}`}
        backUrl="/database/roles"
      />

      <main className="container mx-auto p-4 py-8">
        {error && (
          <div className="bg-red-50 border border-red-200 text-red-600 px-4 py-3 rounded-md mb-6">
            {error}
          </div>
        )}

        <div className="grid gap-6 max-w-4xl mx-auto">
          {/* Rolle Details */}
          <Card>
            <div className="p-6">
              <div className="flex justify-between items-center mb-6">
                <h2 className="text-xl font-semibold">Rollendetails</h2>
                <div className="flex gap-2">
                  <Button
                    variant="destructive"
                    onClick={async () => {
                      if (confirm("Rolle wirklich löschen? Diese Aktion kann nicht rückgängig gemacht werden.")) {
                        try {
                          setError(null);
                          console.log("Attempting to delete role with ID:", role.id);
                          
                          // Direct API call instead of using authService
                          const response = await fetch(`/api/auth/roles/${role.id}`, {
                            method: 'DELETE',
                          });
                          
                          if (!response.ok) {
                            const errorData = await response.text();
                            console.error("Delete role failed:", response.status, errorData);
                            throw new Error(`Failed to delete role: ${response.status} ${errorData}`);
                          }
                          
                          console.log("Role deleted successfully");
                          router.push("/database/roles");
                        } catch (err) {
                          setError("Fehler beim Löschen der Rolle");
                          console.error("Error deleting role:", err);
                        }
                      }
                    }}
                  >
                    Löschen
                  </Button>
                  <Button
                    variant={isEditing ? "default" : "outline"}
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
                </div>
              </div>

              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    Name
                  </label>
                  {isEditing ? (
                    <Input
                      value={formData.name}
                      onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    />
                  ) : (
                    <div className="px-3 py-2 border border-gray-200 rounded-md bg-gray-50">
                      {role.name}
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
                      {role.description || "-"}
                    </div>
                  )}
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Erstellt am
                    </label>
                    <div className="px-3 py-2 border border-gray-200 rounded-md bg-gray-50">
                      {new Date(role.createdAt).toLocaleString("de-DE")}
                    </div>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Aktualisiert am
                    </label>
                    <div className="px-3 py-2 border border-gray-200 rounded-md bg-gray-50">
                      {new Date(role.updatedAt).toLocaleString("de-DE")}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </Card>

          {/* Berechtigungen */}
          <Card>
            <div className="p-6">
              <h2 className="text-xl font-semibold mb-6">Berechtigungen</h2>
              
              <div className="space-y-2">
                {permissions.map((permission) => {
                  const isAssigned = rolePermissions.some(p => p.id === permission.id);
                  
                  return (
                    <div
                      key={permission.id}
                      className="flex items-center p-3 border border-gray-200 rounded-md hover:bg-gray-50"
                    >
                      <input
                        type="checkbox"
                        id={`permission-${permission.id}`}
                        checked={isAssigned}
                        onChange={() => handlePermissionToggle(permission)}
                        className="mr-3 h-4 w-4"
                      />
                      <label
                        htmlFor={`permission-${permission.id}`}
                        className="flex-1 cursor-pointer"
                      >
                        <div className="font-medium">{permission.name}</div>
                        <div className="text-sm text-gray-600">
                          {permission.description}
                        </div>
                        <div className="text-xs text-gray-500 mt-1">
                          Resource: {permission.resource} | Action: {permission.action}
                        </div>
                      </label>
                    </div>
                  );
                })}
              </div>
            </div>
          </Card>
        </div>
      </main>
    </div>
  );
}