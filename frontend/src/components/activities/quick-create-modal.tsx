"use client";

import React, { useState, useEffect } from "react";
import { FormModal } from "~/components/ui/form-modal";
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
        className="px-6 py-2.5 rounded-xl text-gray-700 font-medium bg-white border border-gray-200 hover:bg-gray-50 hover:border-gray-300 transition-all duration-200 disabled:opacity-50"
        disabled={isSubmitting}
      >
        Abbrechen
      </button>
      
      <button
        type="submit"
        form="quick-create-form"
        disabled={isSubmitting || loading}
        className="px-6 py-2.5 rounded-xl font-medium bg-gradient-to-r from-[#83CD2D] to-[#70B525] hover:from-[#90D83D] hover:to-[#83CD2D] text-white shadow-sm hover:shadow-md hover:shadow-[#83CD2D]/25 transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed transform hover:scale-[1.02] active:scale-[0.98]"
      >
        {isSubmitting ? (
          <span className="flex items-center gap-2">
            <svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            Erstellen...
          </span>
        ) : "Aktivität erstellen"}
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
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
            <p className="text-gray-600">Kategorien werden geladen...</p>
          </div>
        </div>
      ) : (
        <form id="quick-create-form" onSubmit={handleSubmit} className="space-y-6">
          {error && (
            <div className="relative overflow-hidden rounded-2xl bg-red-50/80 backdrop-blur-sm border border-red-100 p-4">
              <div className="absolute inset-0 bg-gradient-to-br from-red-50/50 to-pink-50/50 opacity-50"></div>
              <div className="relative flex items-start gap-3">
                <div className="w-8 h-8 rounded-lg bg-red-100 flex items-center justify-center flex-shrink-0">
                  <svg className="w-4 h-4 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
                  </svg>
                </div>
                <p className="text-sm text-red-800">{error}</p>
              </div>
            </div>
          )}

          {/* Activity Name Card */}
          <div className="relative overflow-hidden rounded-2xl bg-gradient-to-br from-blue-50/50 to-indigo-50/50 p-5 border border-blue-100/50">
            <div className="absolute top-2 right-2 w-16 h-16 bg-blue-100/20 rounded-full blur-2xl"></div>
            <div className="absolute bottom-2 left-2 w-12 h-12 bg-indigo-100/20 rounded-full blur-xl"></div>
            <div className="relative">
              <label htmlFor="name" className="block text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
                <div className="w-5 h-5 rounded bg-gradient-to-br from-blue-400 to-indigo-400 flex items-center justify-center">
                  <span className="text-white text-xs font-bold">1</span>
                </div>
                Aktivitätsname
              </label>
              <input
                id="name"
                name="name"
                value={form.name}
                onChange={handleInputChange}
                placeholder="z.B. Bastelstunde, Fußball, Musikkreis..."
                className="block w-full rounded-xl border-0 px-4 py-3.5 text-base text-gray-900 bg-white/80 backdrop-blur-sm shadow-sm ring-1 ring-inset ring-gray-200/50 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-blue-500 focus:bg-white transition-all duration-200"
                required
              />
            </div>
          </div>

          {/* Category Card */}
          <div className="relative overflow-hidden rounded-2xl bg-gradient-to-br from-emerald-50/50 to-teal-50/50 p-5 border border-emerald-100/50">
            <div className="absolute top-2 left-2 w-14 h-14 bg-emerald-100/20 rounded-full blur-2xl"></div>
            <div className="relative">
              <label htmlFor="category_id" className="block text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
                <div className="w-5 h-5 rounded bg-gradient-to-br from-emerald-400 to-teal-400 flex items-center justify-center">
                  <span className="text-white text-xs font-bold">2</span>
                </div>
                Kategorie
              </label>
              <div className="relative">
                <select
                  id="category_id"
                  name="category_id"
                  value={form.category_id}
                  onChange={handleInputChange}
                  className="block w-full appearance-none rounded-xl border-0 px-4 py-3.5 pr-10 text-base text-gray-900 bg-white/80 backdrop-blur-sm shadow-sm ring-1 ring-inset ring-gray-200/50 focus:ring-2 focus:ring-inset focus:ring-emerald-500 focus:bg-white transition-all duration-200 cursor-pointer"
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
          </div>

          {/* Participants Card */}
          <div className="relative overflow-hidden rounded-2xl bg-gradient-to-br from-amber-50/50 to-orange-50/50 p-5 border border-amber-100/50">
            <div className="absolute bottom-2 right-2 w-20 h-20 bg-amber-100/20 rounded-full blur-2xl"></div>
            <div className="relative">
              <label htmlFor="max_participants" className="block text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
                <div className="w-5 h-5 rounded bg-gradient-to-br from-amber-400 to-orange-400 flex items-center justify-center">
                  <span className="text-white text-xs font-bold">3</span>
                </div>
                Maximale Teilnehmerzahl
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
                  className="absolute left-0 z-10 h-full w-14 flex items-center justify-center text-gray-500 hover:text-gray-700 hover:bg-white/50 rounded-l-xl transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-amber-500 disabled:opacity-30 disabled:cursor-not-allowed"
                  disabled={parseInt(form.max_participants) <= 1}
                  aria-label="Teilnehmer reduzieren"
                >
                  <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
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
                  className="block w-full rounded-xl border-0 px-16 py-3.5 text-center text-lg font-semibold text-gray-900 bg-white/80 backdrop-blur-sm shadow-sm ring-1 ring-inset ring-gray-200/50 focus:ring-2 focus:ring-inset focus:ring-amber-500 focus:bg-white transition-all duration-200 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
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
                  className="absolute right-0 z-10 h-full w-14 flex items-center justify-center text-gray-500 hover:text-gray-700 hover:bg-white/50 rounded-r-xl transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-amber-500 disabled:opacity-30 disabled:cursor-not-allowed"
                  disabled={parseInt(form.max_participants) >= 50}
                  aria-label="Teilnehmer erhöhen"
                >
                  <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                  </svg>
                </button>
              </div>
              <p className="mt-2 text-xs text-gray-500 text-center">Zwischen 1 und 50 Teilnehmern</p>
            </div>
          </div>

          {/* Info Card */}
          <div className="relative overflow-hidden rounded-2xl bg-gradient-to-br from-gray-50/80 to-slate-50/80 backdrop-blur-sm border border-gray-200/50 p-4">
            <div className="absolute top-0 right-0 w-24 h-24 bg-gradient-to-br from-blue-100/10 to-indigo-100/10 rounded-full blur-3xl"></div>
            <div className="relative flex items-start gap-3">
              <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-gray-100 to-slate-100 flex items-center justify-center flex-shrink-0">
                <svg className="w-4 h-4 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </div>
              <div className="flex-1">
                <p className="text-sm font-medium text-gray-700 mb-1">Hinweis</p>
                <p className="text-sm text-gray-600">Die Aktivität ist sofort für NFC-Terminals verfügbar.</p>
              </div>
            </div>
          </div>
        </form>
      )}
    </FormModal>
  );
}