"use client";

import {
  useState,
  useEffect,
  useCallback,
  useLayoutEffect,
  useRef,
} from "react";
import type { FormError } from "~/components/ui/form-error";
import { useApiLoadError } from "~/contexts/ToastContext";
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

/** A check that failed before sending: the hint and the field it marks. */
interface ActivityFormProblem {
  readonly message: string;
  readonly field: keyof ActivityFormState;
}

export function parseParticipantLimit(value: string): number | null {
  return value ? Number.parseInt(value, 10) : null;
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
  /** Failed category load, with retry, for `LoadErrorAlert` */
  loadError: FormError | null;
  /** Handle input change events */
  handleInputChange: (
    e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>,
  ) => void;
  /** Validate form and return the first problem or null */
  validateForm: () => ActivityFormProblem | null;
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
 *   form, setForm, categories, loading, loadError,
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
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  // „Wiederholen“ lädt die Kategorien erneut.
  const reloadRef = useRef<() => void>(() => undefined);

  // Load categories when modal opens
  const loadCategories = useCallback(async () => {
    clearLoadError();
    try {
      setLoading(true);
      const categoriesData = await getCategories();
      setCategories(categoriesData ?? []);
    } catch (err) {
      logger.error("failed to load categories", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showLoadError(err, {
        object: "die Liste der Kategorien",
        retry: () => reloadRef.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [showLoadError, clearLoadError]);
  useLayoutEffect(() => {
    reloadRef.current = () => void loadCategories();
  });

  // Auto-load categories when modal opens
  useEffect(() => {
    if (isOpen) void loadCategories();
  }, [isOpen, loadCategories]);

  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
      const { name, value } = e.target;
      setForm((prev) => ({
        ...prev,
        [name]: value,
      }));
    },
    [],
  );

  // Validate form fields
  const validateForm = useCallback((): ActivityFormProblem | null => {
    if (!form.name.trim()) {
      return {
        message: "Bitte geben Sie einen Namen für die Aktivität ein.",
        field: "name",
      };
    }
    if (!form.category_id) {
      return {
        message: "Bitte wählen Sie eine Kategorie.",
        field: "category_id",
      };
    }
    if (!form.max_participants) {
      return null;
    }
    const maxParticipants = Number.parseInt(form.max_participants, 10);
    if (Number.isNaN(maxParticipants) || maxParticipants < 1) {
      return {
        message: "Die Teilnehmerzahl muss mindestens 1 sein.",
        field: "max_participants",
      };
    }
    return null;
  }, [form.name, form.category_id, form.max_participants]);

  return {
    form,
    setForm,
    categories,
    loading,
    loadError,
    handleInputChange,
    validateForm,
    loadCategories,
  };
}
