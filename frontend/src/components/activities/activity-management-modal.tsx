"use client";

import React, { useState, useEffect, useCallback } from "react";
import { createPortal } from "react-dom";
import { getCategories, updateActivity, deleteActivity, type ActivityCategory, type Activity } from "~/lib/activity-api";
import { SimpleAlert } from "~/components/simple/SimpleAlert";
import { getDbOperationMessage } from "~/lib/use-notification";

interface ActivityManagementModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: () => void;
  activity: Activity;
  currentStaffId?: string | null;
  readOnly?: boolean;
}

interface EditForm {
  name: string;
  category_id: string;
  max_participants: string;
}

export function ActivityManagementModal({
  isOpen,
  onClose,
  onSuccess,
  activity,
  currentStaffId: _currentStaffId,
  readOnly = false
}: ActivityManagementModalProps) {
  const [showSuccessAlert, setShowSuccessAlert] = useState(false);
  const [successMessage, setSuccessMessage] = useState("");
  const [form, setForm] = useState<EditForm>({
    name: activity.name,
    category_id: activity.ag_category_id || "",
    max_participants: activity.max_participant?.toString() || "15"
  });
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isAnimating, setIsAnimating] = useState(false);
  const [isExiting, setIsExiting] = useState(false);

  // Handle modal close with animation
  const handleClose = useCallback(() => {
    setIsExiting(true);
    setIsAnimating(false);
    
    // Delay actual close to allow exit animation
    setTimeout(() => {
      onClose();
    }, 250);
  }, [onClose]);

  // Load categories and reset form when modal opens or activity changes
  useEffect(() => {
    if (isOpen) {
      // Trigger entrance animation with slight delay for smooth effect
      setTimeout(() => {
        setIsAnimating(true);
      }, 10);
      void loadCategories();
      // Reset form with current activity values
      setForm({
        name: activity.name,
        category_id: activity.ag_category_id || "",
        max_participants: activity.max_participant?.toString() || "15"
      });
      setError(null);
      setShowDeleteConfirm(false);
    }
  }, [isOpen, activity]);

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
      // Prepare the update data
      const updateData = {
        name: form.name.trim(),
        category_id: parseInt(form.category_id),
        max_participants: parseInt(form.max_participants),
        // Include existing values that might be required
        is_open: activity.is_open_ags || false,
        supervisor_ids: activity.supervisor_id ? [parseInt(activity.supervisor_id)] : []
      };

      // Call the update API
      await updateActivity(activity.id, updateData);
      
      // Show success notification
      setSuccessMessage(getDbOperationMessage('update', 'Aktivität', form.name.trim()));
      setShowSuccessAlert(true);
      
      // Handle success
      if (onSuccess) {
        onSuccess();
      }
      
      // Close modal with animation
      handleClose();
    } catch (err) {
      console.error("Error updating activity:", err);
      
      // Extract meaningful error message from API response
      let errorMessage = "Failed to update activity";
      
      if (err instanceof Error) {
        const message = err.message;
        
        // Handle specific error cases with user-friendly messages
        if (message.includes("user is not a teacher")) {
          errorMessage = "Nur pädagogische Fachkräfte können Aktivitäten bearbeiten. Bitte wenden Sie sich an eine pädagogische Fachkraft oder einen Administrator.";
        } else if (message.includes("401")) {
          errorMessage = "Sie haben keine Berechtigung, diese Aktivität zu bearbeiten.";
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

  const handleDelete = async () => {
    setIsDeleting(true);
    setError(null);

    try {
      await deleteActivity(activity.id);
      
      // Show success notification
      setSuccessMessage(getDbOperationMessage('delete', 'Aktivität', activity.name));
      setShowSuccessAlert(true);
      
      // Handle success
      if (onSuccess) {
        onSuccess();
      }
      
      // Close modal with animation
      handleClose();
    } catch (err) {
      console.error("Error deleting activity:", err);
      
      let errorMessage = "Fehler beim Löschen der Aktivität";
      
      if (err instanceof Error) {
        const message = err.message;
        
        if (message.includes("students enrolled")) {
          errorMessage = "Diese Aktivität kann nicht gelöscht werden, da noch Schüler eingeschrieben sind. Bitte entfernen Sie zuerst alle Schüler aus der Aktivität.";
        } else if (message.includes("401") || message.includes("403")) {
          errorMessage = "Sie haben keine Berechtigung, diese Aktivität zu löschen.";
        } else {
          errorMessage = message;
        }
      }
      
      setError(errorMessage);
      setShowDeleteConfirm(false);
    } finally {
      setIsDeleting(false);
    }
  };

  const footer = (
    <>
      <div className="flex flex-col-reverse md:flex-row items-stretch md:items-center justify-between w-full gap-3">
        <div className="flex-1 md:flex-initial">
          {!readOnly && !showDeleteConfirm && (
            <button
              type="button"
              onClick={() => setShowDeleteConfirm(true)}
              className="w-full md:w-auto px-4 py-2.5 md:py-2 rounded-lg text-sm font-medium text-red-600 hover:text-red-700 hover:bg-red-50 transition-colors duration-150 disabled:opacity-50"
              disabled={isSubmitting || isDeleting}
            >
              <svg className="w-4 h-4 inline-block mr-1.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
              Löschen
            </button>
          )}
          {!readOnly && showDeleteConfirm && (
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2">
              <span className="text-sm text-red-600 text-center sm:text-left">Wirklich löschen?</span>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={handleDelete}
                  disabled={isDeleting}
                  className="flex-1 sm:flex-initial px-3 py-2 sm:py-1.5 rounded-lg text-sm font-medium text-white bg-red-600 hover:bg-red-700 transition-colors duration-150 disabled:opacity-50"
                >
                  {isDeleting ? "Wird gelöscht..." : "Ja, löschen"}
                </button>
                <button
                  type="button"
                  onClick={() => setShowDeleteConfirm(false)}
                  disabled={isDeleting}
                  className="flex-1 sm:flex-initial px-3 py-2 sm:py-1.5 rounded-lg text-sm font-medium text-gray-600 hover:text-gray-700 hover:bg-gray-100 transition-colors duration-150"
                >
                  Abbrechen
                </button>
              </div>
            </div>
          )}
        </div>
        
        <div className="flex items-center gap-2 md:gap-3">
          <button
            type="button"
            onClick={handleClose}
            className="flex-1 md:flex-initial px-4 py-2.5 md:py-2 rounded-lg text-sm font-medium text-gray-700 bg-white border border-gray-200 hover:bg-gray-50 hover:border-gray-300 transition-colors duration-150 disabled:opacity-50"
            disabled={isSubmitting || isDeleting}
          >
            Schließen
          </button>
          
          {!readOnly && (
            <button
              type="submit"
              form="activity-management-form"
              disabled={isSubmitting || loading || isDeleting}
              className="relative group overflow-hidden px-5 py-2.5 rounded-full text-sm font-medium disabled:opacity-50 disabled:cursor-not-allowed disabled:transform-none flex items-center justify-center gap-2 flex-1 md:flex-initial
              bg-blue-600 hover:bg-blue-700 md:hover:bg-blue-700 active:bg-blue-700 backdrop-blur-md
              text-white
              shadow-[0_4px_6px_-1px_rgba(0,0,0,0.1),0_2px_4px_-1px_rgba(0,0,0,0.06)] md:hover:shadow-[0_10px_15px_-3px_rgba(59,130,246,0.5),0_4px_6px_-2px_rgba(59,130,246,0.25)]
              border border-blue-200/50 md:hover:border-blue-200/60
              ring-1 ring-white/20 md:hover:ring-blue-200/60
              focus:outline-none focus:ring-2 focus:ring-blue-200/60 focus:ring-offset-2"
              style={{ 
                transform: 'translateY(0px)',
                transition: 'box-shadow 50ms ease-out, transform 800ms cubic-bezier(0.4, 0, 0.2, 1), background-color 300ms ease-out, border-color 300ms ease-out' 
              }}
              onMouseEnter={(e) => {
                if (window.innerWidth >= 768) {
                  (e.currentTarget as HTMLButtonElement).style.transform = 'translateY(-4px)';
                }
              }}
              onMouseLeave={(e) => {
                if (window.innerWidth >= 768) {
                  (e.currentTarget as HTMLButtonElement).style.transform = 'translateY(0px)';
                }
              }}
            >
            
            {/* Content */}
            <div className="relative flex items-center gap-2">
              {isSubmitting ? (
                <>
                  <svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                  <span className="font-semibold">Wird gespeichert...</span>
                </>
              ) : (
                <>
                  <div className="relative">
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" 
                        className="group-hover:stroke-white/90 group-hover:drop-shadow-[0_0_8px_rgba(255,255,255,0.8)] transition-all duration-300" />
                    </svg>
                    {/* Subtle glow behind icon */}
                    <div className="absolute inset-0 bg-gradient-radial from-white/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500 blur-sm"></div>
                  </div>
                  <span className="font-semibold">Speichern</span>
                </>
              )}
            </div>
            
            {/* Animated background shine effect */}
            <div className="absolute inset-0 -top-2 h-full w-full rotate-12 bg-gradient-to-r from-transparent via-white/10 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 group-hover:animate-[shine_1s_ease-in-out]"></div>
            </button>
          )}
        </div>
      </div>
    </>
  );

  // Handle escape key
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        handleClose();
      }
    };

    if (isOpen) {
      document.addEventListener('keydown', handleEscape);
      document.body.style.overflow = 'hidden';
    }

    return () => {
      document.removeEventListener('keydown', handleEscape);
      document.body.style.overflow = 'unset';
    };
  }, [isOpen, handleClose]);

  // Handle backdrop click
  const handleBackdropClick = useCallback((e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      handleClose();
    }
  }, [handleClose]);

  if (!isOpen) return null;

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
            Aktivität: {activity.name}
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
        <div className="overflow-y-auto max-h-[calc(100vh-12rem)] sm:max-h-[calc(90vh-8rem)] scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-gray-100">
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
        <form id="activity-management-form" onSubmit={handleSubmit} className="space-y-4">
          {/* Creator info - positioned at top */}
          <div className="-mt-2 -mx-2 px-2 pb-3 mb-4 border-b border-gray-100 md:-mx-2 md:px-2">
            <p className="text-sm text-gray-500">
              Erstellt von: {activity.supervisors && activity.supervisors.length > 0 && activity.supervisors[0] ? (activity.supervisors[0].full_name ?? 'Unbekannt') : 'Unbekannt'}
            </p>
          </div>
          
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

          {/* Activity Name Card - Compact */}
          <div className="relative overflow-hidden rounded-xl bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-3 md:p-4 border border-gray-200/50">
            <div className="absolute top-1 right-1 w-12 h-12 bg-gray-100/20 rounded-full blur-xl"></div>
            <div className="relative">
              <label htmlFor="name" className="block text-xs font-semibold text-gray-700 mb-2 flex items-center gap-1.5">
                <div className="w-4 h-4 rounded bg-gradient-to-br from-gray-600 to-gray-700 flex items-center justify-center flex-shrink-0">
                  <span className="text-white text-[10px] font-bold">1</span>
                </div>
                Aktivitätsname
              </label>
              <input
                id="name"
                name="name"
                value={form.name}
                onChange={handleInputChange}
                placeholder="z.B. Hausaufgaben, Malen, Basteln..."
                className="block w-full rounded-lg border-0 px-3 py-3 md:py-2.5 text-base md:text-sm text-gray-900 bg-white/80 backdrop-blur-sm shadow-sm ring-1 ring-inset ring-gray-200/50 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-[#5080D8] focus:bg-white transition-all duration-200 disabled:bg-gray-50 disabled:cursor-not-allowed"
                required
                disabled={readOnly}
              />
            </div>
          </div>

          {/* Category Card - Compact */}
          <div className="relative overflow-hidden rounded-xl bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-3 md:p-4 border border-gray-200/50">
            <div className="absolute top-1 left-1 w-10 h-10 bg-gray-100/20 rounded-full blur-xl"></div>
            <div className="relative">
              <label htmlFor="category_id" className="block text-xs font-semibold text-gray-700 mb-2 flex items-center gap-1.5">
                <div className="w-4 h-4 rounded bg-gradient-to-br from-gray-600 to-gray-700 flex items-center justify-center flex-shrink-0">
                  <span className="text-white text-[10px] font-bold">2</span>
                </div>
                Kategorie
              </label>
              <div className="relative">
                <select
                  id="category_id"
                  name="category_id"
                  value={form.category_id}
                  onChange={handleInputChange}
                  className="block w-full appearance-none rounded-lg border-0 px-3 py-3 md:py-2.5 pr-10 text-base md:text-sm text-gray-900 bg-white/80 backdrop-blur-sm shadow-sm ring-1 ring-inset ring-gray-200/50 focus:ring-2 focus:ring-inset focus:ring-[#5080D8] focus:bg-white transition-all duration-200 cursor-pointer disabled:bg-gray-50 disabled:cursor-not-allowed"
                  required
                  disabled={readOnly}
                >
                  <option value="">Kategorie wählen...</option>
                  {categories.map((category) => (
                    <option key={category.id} value={category.id}>
                      {category.name}
                    </option>
                  ))}
                </select>
                <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
                  <svg className="h-5 w-5 md:h-4 md:w-4 text-gray-400" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                    <path fillRule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clipRule="evenodd" />
                  </svg>
                </div>
              </div>
            </div>
          </div>

          {/* Participants Card - Compact */}
          <div className="relative overflow-hidden rounded-xl bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-3 md:p-4 border border-gray-200/50">
            <div className="absolute bottom-1 right-1 w-14 h-14 bg-gray-100/20 rounded-full blur-xl"></div>
            <div className="relative">
              <label htmlFor="max_participants" className="block text-xs font-semibold text-gray-700 mb-2 flex items-center gap-1.5">
                <div className="w-4 h-4 rounded bg-gradient-to-br from-gray-600 to-gray-700 flex items-center justify-center flex-shrink-0">
                  <span className="text-white text-[10px] font-bold">3</span>
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
                  className="absolute left-0 z-10 h-full w-12 md:w-10 flex items-center justify-center text-gray-500 hover:text-gray-700 hover:bg-white/50 rounded-l-lg transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-[#5080D8] disabled:opacity-30 disabled:cursor-not-allowed active:scale-95"
                  disabled={parseInt(form.max_participants) <= 1 || readOnly}
                  aria-label="Teilnehmer reduzieren"
                >
                  <svg className="w-5 h-5 md:w-4 md:h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
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
                  className="block w-full rounded-lg border-0 px-14 md:px-12 py-3 md:py-2.5 text-center text-lg md:text-base font-semibold text-gray-900 bg-white/80 backdrop-blur-sm shadow-sm ring-1 ring-inset ring-gray-200/50 focus:ring-2 focus:ring-inset focus:ring-[#5080D8] focus:bg-white transition-all duration-200 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none disabled:bg-gray-50 disabled:cursor-not-allowed"
                  required
                  disabled={readOnly}
                />
                
                <button
                  type="button"
                  onClick={() => {
                    const current = parseInt(form.max_participants);
                    if (current < 50) {
                      setForm(prev => ({ ...prev, max_participants: (current + 1).toString() }));
                    }
                  }}
                  className="absolute right-0 z-10 h-full w-12 md:w-10 flex items-center justify-center text-gray-500 hover:text-gray-700 hover:bg-white/50 rounded-r-lg transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-[#5080D8] disabled:opacity-30 disabled:cursor-not-allowed active:scale-95"
                  disabled={parseInt(form.max_participants) >= 50 || readOnly}
                  aria-label="Teilnehmer erhöhen"
                >
                  <svg className="w-5 h-5 md:w-4 md:h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                  </svg>
                </button>
              </div>
            </div>
          </div>

          {/* Info Card - Compact */}
          <div className="relative overflow-hidden rounded-lg bg-gradient-to-br from-gray-50/60 to-slate-50/60 backdrop-blur-sm border border-gray-200/30 p-3">
            <div className="relative flex items-center gap-2">
              <svg className="w-3.5 h-3.5 text-gray-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <p className="text-xs text-gray-600">{readOnly ? 'Sie können nur Aktivitäten bearbeiten, die Sie selbst erstellt haben.' : 'Änderungen werden sofort wirksam.'}</p>
            </div>
          </div>
        </form>
      )}
          </div>
        </div>

        {/* Footer */}
        {footer && (
          <div className="border-t border-gray-100 bg-gray-50/50 p-4 md:p-6">
            {footer}
          </div>
        )}
      </div>
    </div>
  );

  // Portal render
  if (typeof document !== 'undefined') {
    return createPortal(
      <>
        <style>{`
          @keyframes shine {
            0% { transform: translateX(-100%) rotate(12deg); }
            100% { transform: translateX(100%) rotate(12deg); }
          }
        `}</style>
        {modalContent}
        {/* Success Alert */}
        {showSuccessAlert && (
          <SimpleAlert
            type="success"
            message={successMessage}
            autoClose
            onClose={() => setShowSuccessAlert(false)}
          />
        )}
      </>,
      document.body
    );
  }

  return null;
}