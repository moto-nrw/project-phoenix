"use client";

import { useState, useEffect, useCallback } from "react";
import { getCategories, type ActivityCategory } from "~/lib/activity-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "useActivityForm" });

/**
 * Activity form state shape.
 * Used by both create and edit modals.
 */
export interface ActivityFormState {
  name: string;
  category_id: string;
  max_participants: string;
}

/**
 * Return type for the useActivityForm hook.
 */
interface UseActivityFormReturn {
  /** Current form state */
  form: ActivityFormState;
  /** Update form state */
  setForm: React.Dispatch<React.SetStateAction<ActivityFormState>>;
  /** Available categories */
  categories: ActivityCategory[];
  /** Whether categories are loading */
  loading: boolean;
  /** Current error message */
  error: string | null;
  /** Set error message */
  setError: React.Dispatch<React.SetStateAction<string | null>>;
  /** Handle input change events */
  handleInputChange: (
    e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>,
  ) => void;
  /** Validate form and return error message or null */
  validateForm: () => string | null;
  /** Load categories from API */
  loadCategories: () => Promise<void>;
}

/**
 * Custom hook for managing activity form state and validation.
 *
 * Centralizes form logic shared between ActivityManagementModal
 * and QuickCreateActivityModal to eliminate code duplication.
 *
 * @param initialForm - Initial form values
 * @param isOpen - Whether the modal is open (triggers category loading)
 *
 * @example
 * ```tsx
 * const {
 *   form, setForm, categories, loading, error, setError,
 *   handleInputChange, validateForm, loadCategories
 * } = useActivityForm(initialValues, isOpen);
 * ```
 */
export function useActivityForm(
  initialForm: ActivityFormState,
  isOpen: boolean,
): UseActivityFormReturn {
  const [form, setForm] = useState<ActivityFormState>(initialForm);
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load categories when modal opens
  const loadCategories = useCallback(async () => {
    try {
      setLoading(true);
      const categoriesData = await getCategories();
      setCategories(categoriesData ?? []);
    } catch (err) {
      logger.error("failed to load categories", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError("Failed to load categories");
    } finally {
      setLoading(false);
    }
  }, []);

  // Auto-load categories when modal opens
  useEffect(() => {
    if (isOpen) {
      loadCategories().catch(() => {
        // Error already handled in loadCategories
      });
    }
  }, [isOpen, loadCategories]);

  // Handle input changes and clear errors
  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
      const { name, value } = e.target;
      setForm((prev) => ({
        ...prev,
        [name]: value,
      }));
      // Clear error when user starts typing
      setError(null);
    },
    [],
  );

  // Validate form fields
  const validateForm = useCallback((): string | null => {
    if (!form.name.trim()) {
      return "Activity name is required";
    }
    if (!form.category_id) {
      return "Please select a category";
    }
    const maxParticipants = Number.parseInt(form.max_participants, 10);
    if (Number.isNaN(maxParticipants) || maxParticipants < 1) {
      return "Max participants must be a positive number";
    }
    return null;
  }, [form.name, form.category_id, form.max_participants]);

  return {
    form,
    setForm,
    categories,
    loading,
    error,
    setError,
    handleInputChange,
    validateForm,
    loadCategories,
  };
}
