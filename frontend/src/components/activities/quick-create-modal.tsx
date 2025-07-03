"use client";

import React, { useState, useEffect, useCallback } from "react";
import { createPortal } from "react-dom";
import { getCategories, type ActivityCategory } from "~/lib/activity-api";
import { SimpleAlert, alertAnimationStyles } from "~/components/simple/SimpleAlert";
import { getDbOperationMessage } from "~/lib/use-notification";
import { useScrollLock } from "~/hooks/useScrollLock";

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
  const [showSuccessAlert, setShowSuccessAlert] = useState(false);
  const [successMessage, setSuccessMessage] = useState("");
  const [form, setForm] = useState<QuickCreateForm>({
    name: "",
    category_id: "",
    max_participants: "15"
  });
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isAnimating, setIsAnimating] = useState(false);
  const [isExiting, setIsExiting] = useState(false);
  
  // Use scroll lock hook
  useScrollLock(isOpen);

  // Handle modal close with animation
  const handleClose = useCallback(() => {
    setIsExiting(true);
    setIsAnimating(false);
    
    // Delay actual close to allow exit animation
    setTimeout(() => {
      onClose();
    }, 250);
  }, [onClose]);

  // Load categories and manage animations when modal opens
  useEffect(() => {
    if (isOpen) {
      // Trigger entrance animation with slight delay for smooth effect
      setTimeout(() => {
        setIsAnimating(true);
      }, 10);
      void loadCategories();
      // Reset form when modal opens
      setForm({
        name: "",
        category_id: "",
        max_participants: "15"
      });
      setError(null);
      // Don't reset success alert here - it should persist after modal closes
    }
  }, [isOpen]);

  // Reset animation states when modal closes
  useEffect(() => {
    if (!isOpen) {
      setIsAnimating(false);
      setIsExiting(false);
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
      
      // Show success notification
      setSuccessMessage(getDbOperationMessage('create', 'Aktivität', form.name.trim()));
      setShowSuccessAlert(true);
      
      // Handle success
      if (onSuccess) {
        onSuccess();
      }
      
      // Close modal immediately - success alert will persist independently
      handleClose();
    } catch (err) {
      console.error("Error creating activity:", err);
      
      // Extract meaningful error message from API response
      let errorMessage = "Failed to create activity";
      
      if (err instanceof Error) {
        const message = err.message;
        
        // Handle specific error cases with user-friendly messages
        if (message.includes("user is not authenticated")) {
          errorMessage = "Sie müssen angemeldet sein, um Aktivitäten zu erstellen.";
        } else if (message.includes("401")) {
          errorMessage = "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.";
        } else if (message.includes("403")) {
          errorMessage = "Zugriff verweigert. Bitte melden Sie sich erneut an.";
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

  // Handle escape key
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        handleClose();
      }
    };

    if (isOpen) {
      document.addEventListener('keydown', handleEscape);
    }

    return () => {
      document.removeEventListener('keydown', handleEscape);
    };
  }, [isOpen, handleClose]);

  // Handle backdrop click
  const handleBackdropClick = useCallback((e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      handleClose();
    }
  }, [handleClose]);

  // Don't return null here - we need to render the success alert even when modal is closed

  const modalContent = (
    <div
      className={`fixed inset-0 z-[9999] flex items-center justify-center transition-all duration-400 ease-out ${
        isAnimating && !isExiting 
          ? 'bg-black/40' 
          : 'bg-black/0'
      }`}
      onClick={handleBackdropClick}
      style={{ 
        position: 'fixed', 
        top: 0, 
        left: 0, 
        right: 0, 
        bottom: 0,
        animation: isAnimating && !isExiting ? 'backdropEnter 400ms ease-out' : undefined
      }}
    >
      {/* Modal */}
      <div className={`relative w-[calc(100%-2rem)] max-w-md mx-4 rounded-2xl shadow-2xl border border-gray-200/50 overflow-hidden transform ${
          isAnimating && !isExiting
            ? 'animate-modalEnter' 
            : isExiting
            ? 'animate-modalExit'
            : 'scale-75 opacity-0 translate-y-8 -rotate-1'
        }`}
        onClick={(e) => e.stopPropagation()}
        style={{
          background: 'linear-gradient(135deg, rgba(255,255,255,0.95) 0%, rgba(248,250,252,0.98) 100%)',
          backdropFilter: 'blur(20px)',
          boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.25), 0 8px 16px -8px rgba(80, 128, 216, 0.15)',
          animationFillMode: 'both'
        }}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-4 md:p-6 border-b border-gray-100">
          <h3 className="text-lg md:text-xl font-semibold text-gray-900 pr-4">
            Aktivität erstellen
          </h3>
          <button
            onClick={handleClose}
            className="group relative flex-shrink-0 p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-xl transition-all duration-200 hover:scale-105 active:scale-95"
            aria-label="Modal schließen"
          >
            {/* Animated X icon */}
            <svg 
              className="w-5 h-5 transition-transform duration-200 group-hover:rotate-90" 
              fill="none" 
              viewBox="0 0 24 24" 
              stroke="currentColor"
              strokeWidth={2}
            >
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
            
            {/* Subtle hover glow */}
            <div 
              className="absolute inset-0 rounded-xl opacity-0 group-hover:opacity-100 transition-opacity duration-200"
              style={{
                boxShadow: '0 0 12px rgba(80,128,216,0.3)'
              }}
            />
          </button>
        </div>

        {/* Content */}
        <div className="overflow-y-auto max-h-[calc(100vh-12rem)] sm:max-h-[calc(90vh-8rem)] scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-gray-100" data-modal-content="true">
          <div className={`p-4 md:p-6 ${
            isAnimating && !isExiting ? 'sm:animate-contentReveal' : 'sm:opacity-0'
          }`}>
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
          <div className="relative overflow-hidden rounded-2xl bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-5 border border-gray-200/50">
            <div className="absolute top-2 right-2 w-16 h-16 bg-gray-100/20 rounded-full blur-2xl"></div>
            <div className="absolute bottom-2 left-2 w-12 h-12 bg-slate-100/20 rounded-full blur-xl"></div>
            <div className="relative">
              <label htmlFor="name" className="block text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
                <div className="w-5 h-5 rounded bg-gradient-to-br from-gray-600 to-gray-700 flex items-center justify-center">
                  <span className="text-white text-xs font-bold">1</span>
                </div>
                Aktivitätsname
              </label>
              <input
                id="name"
                name="name"
                value={form.name}
                onChange={handleInputChange}
                placeholder="z.B. Hausaufgaben, Malen, Basteln..."
                className="block w-full rounded-xl border-0 px-4 py-3.5 text-base text-gray-900 bg-white/80 backdrop-blur-sm shadow-sm ring-1 ring-inset ring-gray-200/50 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-gray-700 focus:bg-white transition-all duration-200"
                required
              />
            </div>
          </div>

          {/* Category Card */}
          <div className="relative overflow-hidden rounded-2xl bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-5 border border-gray-200/50">
            <div className="absolute top-2 left-2 w-14 h-14 bg-gray-100/20 rounded-full blur-2xl"></div>
            <div className="relative">
              <label htmlFor="category_id" className="block text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
                <div className="w-5 h-5 rounded bg-gradient-to-br from-gray-600 to-gray-700 flex items-center justify-center">
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
                  className="block w-full appearance-none rounded-xl border-0 px-4 py-3.5 pr-10 text-base text-gray-900 bg-white/80 backdrop-blur-sm shadow-sm ring-1 ring-inset ring-gray-200/50 focus:ring-2 focus:ring-inset focus:ring-gray-700 focus:bg-white transition-all duration-200 cursor-pointer"
                  required
                >
                  <option value="">Kategorie wählen...</option>
                  {/* Categories are fetched from backend. Expected values:
                      - Gruppenraum
                      - Hausaufgaben
                      - Kreatives/Musik
                      - Bewegen/Entspannen
                      - Natur
                      - HW/Technik
                      - Spielen
                      - Lernen
                  */}
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
          <div className="relative overflow-hidden rounded-2xl bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-5 border border-gray-200/50">
            <div className="absolute bottom-2 right-2 w-20 h-20 bg-gray-100/20 rounded-full blur-2xl"></div>
            <div className="relative">
              <label htmlFor="max_participants" className="block text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
                <div className="w-5 h-5 rounded bg-gradient-to-br from-gray-600 to-gray-700 flex items-center justify-center">
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
                  className="absolute left-0 z-10 h-full w-14 flex items-center justify-center text-gray-500 hover:text-gray-700 hover:bg-white/50 rounded-l-xl transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-gray-700 disabled:opacity-30 disabled:cursor-not-allowed"
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
                  className="block w-full rounded-xl border-0 px-16 py-3.5 text-center text-lg font-semibold text-gray-900 bg-white/80 backdrop-blur-sm shadow-sm ring-1 ring-inset ring-gray-200/50 focus:ring-2 focus:ring-inset focus:ring-gray-700 focus:bg-white transition-all duration-200 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
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
                  className="absolute right-0 z-10 h-full w-14 flex items-center justify-center text-gray-500 hover:text-gray-700 hover:bg-white/50 rounded-r-xl transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-gray-700 disabled:opacity-30 disabled:cursor-not-allowed"
                  disabled={parseInt(form.max_participants) >= 50}
                  aria-label="Teilnehmer erhöhen"
                >
                  <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                  </svg>
                </button>
              </div>
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
          </div>
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-3 p-4 md:p-6 border-t border-gray-100 bg-gray-50/50">
          <button
            type="button"
            onClick={handleClose}
            className="px-4 py-2 rounded-lg text-sm font-medium text-gray-700 bg-white border border-gray-200 hover:bg-gray-50 hover:border-gray-300 transition-colors duration-150 disabled:opacity-50"
            disabled={isSubmitting}
          >
            Abbrechen
          </button>
          
          <button
            type="submit"
            form="quick-create-form"
            disabled={isSubmitting || loading}
            className="relative px-5 py-2.5 rounded-lg text-sm font-medium text-white transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed disabled:transform-none flex items-center gap-2
            bg-[#83CD2D] hover:bg-[#75BC28] focus:bg-[#75BC28]
            shadow-sm hover:shadow-md focus:shadow-md
            transform hover:-translate-y-0.5 active:translate-y-0
            focus:outline-none focus:ring-2 focus:ring-[#83CD2D] focus:ring-offset-2"
          >
            {isSubmitting ? (
              <>
                <svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <span>Wird erstellt...</span>
              </>
            ) : (
              <>
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                </svg>
                <span>Aktivität erstellen</span>
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );

  // Portal render
  if (typeof document !== 'undefined') {
    return (
      <>
        {/* Render modal only when open */}
        {isOpen && createPortal(modalContent, document.body)}
        {/* Success Alert - rendered independently of modal state */}
        {showSuccessAlert && createPortal(
          <>
            {alertAnimationStyles}
            <SimpleAlert
              type="success"
              message={successMessage}
              autoClose
              duration={3000}
              onClose={() => setShowSuccessAlert(false)}
            />
          </>,
          document.body
        )}
      </>
    );
  }

  return null;
}