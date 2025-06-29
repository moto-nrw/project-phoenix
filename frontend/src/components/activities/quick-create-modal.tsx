"use client";

import React, { useState, useEffect } from "react";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { getCategories, type ActivityCategory } from "~/lib/activity-api";

interface QuickCreateActivityModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: () => void;
}

interface QuickCreateForm {
  name: string;
  category_id: string;
  max_participants: string;
}

export function QuickCreateActivityModal({
  isOpen,
  onClose,
  onSuccess
}: QuickCreateActivityModalProps) {
  const [form, setForm] = useState<QuickCreateForm>({
    name: "",
    category_id: "",
    max_participants: "15"
  });
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Load categories when modal opens
  useEffect(() => {
    if (isOpen) {
      void loadCategories();
      // Reset form when modal opens
      setForm({
        name: "",
        category_id: "",
        max_participants: "15"
      });
      setError(null);
    }
  }, [isOpen]);

  const loadCategories = async () => {
    try {
      setLoading(true);
      const categoriesData = await getCategories();
      setCategories(categoriesData ?? []);
    } catch (err) {
      console.error("Failed to load categories:", err);
      setError("Failed to load categories");
    } finally {
      setLoading(false);
    }
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target;
    setForm(prev => ({
      ...prev,
      [name]: value
    }));
    // Clear error when user starts typing
    if (error) setError(null);
  };

  const validateForm = (): string | null => {
    if (!form.name.trim()) {
      return "Activity name is required";
    }
    if (!form.category_id) {
      return "Please select a category";
    }
    const maxParticipants = parseInt(form.max_participants);
    if (isNaN(maxParticipants) || maxParticipants < 1) {
      return "Max participants must be a positive number";
    }
    return null;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    const validationError = validateForm();
    if (validationError) {
      setError(validationError);
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      // Prepare the request data
      const requestData = {
        name: form.name.trim(),
        category_id: parseInt(form.category_id),
        max_participants: parseInt(form.max_participants)
      };

      // Call the quick-create API endpoint
      const response = await fetch("/api/activities/quick-create", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(requestData),
      });

      if (!response.ok) {
        throw new Error(`Failed to create activity: ${response.status}`);
      }

      await response.json();
      
      // Handle success
      if (onSuccess) {
        onSuccess();
      }
      onClose();
    } catch (err) {
      console.error("Error creating activity:", err);
      
      // Extract meaningful error message from API response
      let errorMessage = "Failed to create activity";
      
      if (err instanceof Error) {
        const message = err.message;
        
        // Handle specific error cases with user-friendly messages
        if (message.includes("user is not a teacher")) {
          errorMessage = "Nur pädagogische Fachkräfte können Aktivitäten erstellen. Bitte wenden Sie sich an eine pädagogische Fachkraft oder einen Administrator.";
        } else if (message.includes("401")) {
          errorMessage = "Sie haben keine Berechtigung, Aktivitäten zu erstellen.";
        } else if (message.includes("403")) {
          errorMessage = "Zugriff verweigert. Bitte prüfen Sie Ihre Berechtigungen.";
        } else if (message.includes("400")) {
          errorMessage = "Ungültige Eingabedaten. Bitte überprüfen Sie Ihre Eingaben.";
        } else {
          errorMessage = message;
        }
      }
      
      setError(errorMessage);
    } finally {
      setIsSubmitting(false);
    }
  };

  const footer = (
    <>
      <button
        type="button"
        onClick={onClose}
        className="rounded-lg bg-gray-200 px-4 py-2 text-gray-800 transition-colors hover:bg-gray-300 disabled:opacity-50"
        disabled={isSubmitting}
      >
        Abbrechen
      </button>
      
      <button
        type="submit"
        form="quick-create-form"
        disabled={isSubmitting || loading}
        className="px-4 py-2 bg-blue-500 hover:bg-blue-600 rounded-lg text-white transition-colors disabled:opacity-50"
      >
        {isSubmitting ? "Erstellen..." : "Aktivität erstellen"}
      </button>
    </>
  );

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title="Aktivität erstellen"
      size="sm"
      footer={footer}
    >
      {loading ? (
        <div className="flex items-center justify-center py-8">
          <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
          <p className="ml-3 text-gray-600">Categories werden geladen...</p>
        </div>
      ) : (
        <form id="quick-create-form" onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="rounded-lg bg-red-50 p-4 text-red-800">
              <p className="text-sm">{error}</p>
            </div>
          )}

          <Input
            label="Aktivitätsname"
            name="name"
            value={form.name}
            onChange={handleInputChange}
            placeholder="z.B. Bastelstunde"
            required
          />

          <div>
            <label htmlFor="category_id" className="block text-sm font-medium text-gray-700 mb-2">
              Kategorie
            </label>
            <div className="relative">
              <select
                id="category_id"
                name="category_id"
                value={form.category_id}
                onChange={handleInputChange}
                className="block w-full appearance-none rounded-lg border-0 px-4 py-3 pr-10 text-base text-gray-900 bg-white shadow-sm ring-1 ring-inset ring-gray-200 focus:ring-2 focus:ring-inset focus:ring-gray-900 transition-all duration-200"
                required
              >
                <option value="">Kategorie wählen...</option>
                {categories.map((category) => (
                  <option key={category.id} value={category.id}>
                    {category.name}
                  </option>
                ))}
              </select>
              <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
                <svg className="h-5 w-5 text-gray-400" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                  <path fillRule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clipRule="evenodd" />
                </svg>
              </div>
            </div>
          </div>

          <div>
            <label htmlFor="max_participants" className="block text-sm font-medium text-gray-700 mb-2">
              Max. Teilnehmer
            </label>
            <div className="relative flex items-center">
              <button
                type="button"
                onClick={() => {
                  const current = parseInt(form.max_participants);
                  if (current > 1) {
                    setForm(prev => ({ ...prev, max_participants: (current - 1).toString() }));
                  }
                }}
                className="absolute left-0 h-full w-12 flex items-center justify-center text-gray-400 hover:text-gray-600 hover:bg-gray-50 rounded-l-lg transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-gray-900 disabled:opacity-50 disabled:cursor-not-allowed"
                disabled={parseInt(form.max_participants) <= 1}
                aria-label="Teilnehmer reduzieren"
              >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 12h-15" />
                </svg>
              </button>
              
              <input
                id="max_participants"
                name="max_participants"
                type="number"
                value={form.max_participants}
                onChange={handleInputChange}
                min="1"
                max="50"
                className="block w-full rounded-lg border-0 px-16 py-3 text-center text-base font-medium text-gray-900 bg-white shadow-sm ring-1 ring-inset ring-gray-200 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-gray-900 transition-all duration-200 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                required
              />
              
              <button
                type="button"
                onClick={() => {
                  const current = parseInt(form.max_participants);
                  if (current < 50) {
                    setForm(prev => ({ ...prev, max_participants: (current + 1).toString() }));
                  }
                }}
                className="absolute right-0 h-full w-12 flex items-center justify-center text-gray-400 hover:text-gray-600 hover:bg-gray-50 rounded-r-lg transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-gray-900 disabled:opacity-50 disabled:cursor-not-allowed"
                disabled={parseInt(form.max_participants) >= 50}
                aria-label="Teilnehmer erhöhen"
              >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                </svg>
              </button>
            </div>
          </div>

          <div className="text-sm text-gray-600 bg-gray-50 p-3 rounded-lg">
            <p className="font-medium mb-1">Hinweis:</p>
            <p>Die Aktivität ist sofort für NFC-Terminals verfügbar.</p>
          </div>
        </form>
      )}
    </FormModal>
  );
}