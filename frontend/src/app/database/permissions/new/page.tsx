"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { authService } from "@/lib/auth-service";
import { PageHeader } from "@/components/dashboard";
import { Button, Input, Card } from "@/components/ui";

export default function NewPermissionPage() {
  const router = useRouter();
  const [formData, setFormData] = useState({
    name: "",
    description: "",
    resource: "",
    action: "",
  });
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    try {
      setIsLoading(true);
      setError(null);
      
      if (!formData.name.trim() || !formData.resource.trim() || !formData.action.trim()) {
        setError("Bitte füllen Sie alle Pflichtfelder aus");
        return;
      }

      await authService.createPermission({
        name: formData.name,
        description: formData.description,
        resource: formData.resource,
        action: formData.action,
      });
      
      router.push("/database/permissions");
    } catch (err) {
      setError("Fehler beim Erstellen der Berechtigung");
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen">
      <PageHeader
        title="Neue Berechtigung erstellen"
        backUrl="/database/permissions"
      />

      <main className="container mx-auto p-4 py-8">
        <Card className="max-w-2xl mx-auto">
          <div className="p-6">
            <form onSubmit={handleSubmit} className="space-y-6">
              {error && (
                <div className="bg-red-50 border border-red-200 text-red-600 px-4 py-3 rounded-md">
                  {error}
                </div>
              )}

              <div>
                <label htmlFor="name" className="block text-sm font-medium text-gray-700 mb-2">
                  Name *
                </label>
                <Input
                  id="name"
                  type="text"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  placeholder="z.B. Schüler lesen"
                  required
                />
              </div>

              <div>
                <label htmlFor="description" className="block text-sm font-medium text-gray-700 mb-2">
                  Beschreibung
                </label>
                <textarea
                  id="description"
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  placeholder="Beschreibung der Berechtigung..."
                  className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                  rows={3}
                />
              </div>

              <div>
                <label htmlFor="resource" className="block text-sm font-medium text-gray-700 mb-2">
                  Resource *
                </label>
                <Input
                  id="resource"
                  type="text"
                  value={formData.resource}
                  onChange={(e) => setFormData({ ...formData, resource: e.target.value })}
                  placeholder="z.B. students"
                  required
                />
                <p className="text-sm text-gray-500 mt-1">
                  Die Resource, auf die sich diese Berechtigung bezieht (z.B. students, groups, rooms)
                </p>
              </div>

              <div>
                <label htmlFor="action" className="block text-sm font-medium text-gray-700 mb-2">
                  Action *
                </label>
                <Input
                  id="action"
                  type="text"
                  value={formData.action}
                  onChange={(e) => setFormData({ ...formData, action: e.target.value })}
                  placeholder="z.B. read"
                  required
                />
                <p className="text-sm text-gray-500 mt-1">
                  Die Aktion, die diese Berechtigung erlaubt (z.B. read, write, delete)
                </p>
              </div>

              <div className="flex gap-4">
                <Button
                  type="submit"
                  disabled={isLoading}
                  className="flex-1"
                >
                  {isLoading ? "Wird erstellt..." : "Berechtigung erstellen"}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => router.back()}
                  disabled={isLoading}
                  className="flex-1"
                >
                  Abbrechen
                </Button>
              </div>
            </form>
          </div>
        </Card>
      </main>
    </div>
  );
}