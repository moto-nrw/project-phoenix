import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { act } from "react";
import type { ActivityCategory } from "~/lib/activity-api";
import { useActivityForm, type ActivityFormState } from "./useActivityForm";

// Mock the activity API
vi.mock("~/lib/activity-api", () => ({
  getCategories: vi.fn(),
}));

// Import after mock to get the mocked version
import { getCategories } from "~/lib/activity-api";

describe("useActivityForm", () => {
  const mockCategories: ActivityCategory[] = [
    {
      id: "1",
      name: "Sports",
      description: "Sports activities",
      created_at: new Date("2024-01-01"),
      updated_at: new Date("2024-01-01"),
    },
    {
      id: "2",
      name: "Arts",
      description: "Arts and crafts",
      created_at: new Date("2024-01-01"),
      updated_at: new Date("2024-01-01"),
    },
  ];

  const initialForm: ActivityFormState = {
    name: "",
    category_id: "",
    max_participants: "10",
  };

  beforeEach(() => {
    vi.clearAllMocks();
    // Suppress console.error for expected errors in tests
    vi.spyOn(console, "error").mockImplementation(() => undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("Initial state", () => {
    it("should initialize with provided initial form values", () => {
      const customForm: ActivityFormState = {
        name: "Soccer",
        category_id: "1",
        max_participants: "20",
      };

      const { result } = renderHook(() => useActivityForm(customForm, false));

      expect(result.current.form).toEqual(customForm);
      expect(result.current.categories).toEqual([]);
      expect(result.current.loading).toBe(false);
      expect(result.current.error).toBeNull();
    });

    it("should initialize with empty form values", () => {
      const { result } = renderHook(() => useActivityForm(initialForm, false));

      expect(result.current.form).toEqual(initialForm);
      expect(result.current.categories).toEqual([]);
      expect(result.current.loading).toBe(false);
      expect(result.current.error).toBeNull();
    });
  });

  describe("Loading categories", () => {
    it("should load categories when modal opens (isOpen = true)", async () => {
      vi.mocked(getCategories).mockResolvedValue(mockCategories);

      const { result } = renderHook(() => useActivityForm(initialForm, true));

      // Initially loading
      expect(result.current.loading).toBe(true);

      // Wait for categories to load
      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      expect(result.current.categories).toEqual(mockCategories);
      expect(result.current.error).toBeNull();
      expect(getCategories).toHaveBeenCalledTimes(1);
    });

    it("should not load categories when modal is closed (isOpen = false)", () => {
      vi.mocked(getCategories).mockResolvedValue(mockCategories);

      const { result } = renderHook(() => useActivityForm(initialForm, false));

      expect(result.current.loading).toBe(false);
      expect(result.current.categories).toEqual([]);
      expect(getCategories).not.toHaveBeenCalled();
    });

    it("should reload categories when isOpen changes from false to true", async () => {
      vi.mocked(getCategories).mockResolvedValue(mockCategories);

      const { result, rerender } = renderHook(
        ({ isOpen }) => useActivityForm(initialForm, isOpen),
        { initialProps: { isOpen: false } },
      );

      expect(getCategories).not.toHaveBeenCalled();

      // Open the modal
      rerender({ isOpen: true });

      expect(result.current.loading).toBe(true);

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      expect(result.current.categories).toEqual(mockCategories);
      expect(getCategories).toHaveBeenCalledTimes(1);
    });

    it("should handle null response from getCategories", async () => {
      // Testing defensive code: hook uses ?? [] to handle unexpected null
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-argument
      vi.mocked(getCategories).mockResolvedValue(null as any);

      const { result } = renderHook(() => useActivityForm(initialForm, true));

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      expect(result.current.categories).toEqual([]);
      expect(result.current.error).toBeNull();
    });

    it("should handle undefined response from getCategories", async () => {
      // Testing defensive code: hook uses ?? [] to handle unexpected undefined
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-argument
      vi.mocked(getCategories).mockResolvedValue(undefined as any);

      const { result } = renderHook(() => useActivityForm(initialForm, true));

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      expect(result.current.categories).toEqual([]);
      expect(result.current.error).toBeNull();
    });
  });

  describe("Error handling", () => {
    it("should set error when category loading fails", async () => {
      const error = new Error("Network error");
      vi.mocked(getCategories).mockRejectedValue(error);

      const { result } = renderHook(() => useActivityForm(initialForm, true));

      expect(result.current.loading).toBe(true);

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      expect(result.current.error).toBe("Failed to load categories");
      expect(result.current.categories).toEqual([]);
      expect(console.error).toHaveBeenCalledWith(
        "Failed to load categories:",
        error,
      );
    });

    it("should allow manual error setting", () => {
      const { result } = renderHook(() => useActivityForm(initialForm, false));

      expect(result.current.error).toBeNull();

      act(() => {
        result.current.setError("Custom error message");
      });

      expect(result.current.error).toBe("Custom error message");
    });

    it("should allow clearing errors", () => {
      const { result } = renderHook(() => useActivityForm(initialForm, false));

      act(() => {
        result.current.setError("Some error");
      });

      expect(result.current.error).toBe("Some error");

      act(() => {
        result.current.setError(null);
      });

      expect(result.current.error).toBeNull();
    });
  });

  describe("handleInputChange", () => {
    it("should update form field on input change", () => {
      const { result } = renderHook(() => useActivityForm(initialForm, false));

      const event = {
        target: { name: "name", value: "Basketball" },
      } as React.ChangeEvent<HTMLInputElement>;

      act(() => {
        result.current.handleInputChange(event);
      });

      expect(result.current.form.name).toBe("Basketball");
      expect(result.current.form.category_id).toBe("");
      expect(result.current.form.max_participants).toBe("10");
    });

    it("should update category_id field on select change", () => {
      const { result } = renderHook(() => useActivityForm(initialForm, false));

      const event = {
        target: { name: "category_id", value: "2" },
      } as React.ChangeEvent<HTMLSelectElement>;

      act(() => {
        result.current.handleInputChange(event);
      });

      expect(result.current.form.category_id).toBe("2");
    });

    it("should update max_participants field", () => {
      const { result } = renderHook(() => useActivityForm(initialForm, false));

      const event = {
        target: { name: "max_participants", value: "25" },
      } as React.ChangeEvent<HTMLInputElement>;

      act(() => {
        result.current.handleInputChange(event);
      });

      expect(result.current.form.max_participants).toBe("25");
    });

    it("should clear error when user types", () => {
      const { result } = renderHook(() => useActivityForm(initialForm, false));

      // Set an error first
      act(() => {
        result.current.setError("Validation error");
      });

      expect(result.current.error).toBe("Validation error");

      // Type in a field
      const event = {
        target: { name: "name", value: "Test" },
      } as React.ChangeEvent<HTMLInputElement>;

      act(() => {
        result.current.handleInputChange(event);
      });

      expect(result.current.error).toBeNull();
      expect(result.current.form.name).toBe("Test");
    });

    it("should handle multiple field changes", () => {
      const { result } = renderHook(() => useActivityForm(initialForm, false));

      act(() => {
        result.current.handleInputChange({
          target: { name: "name", value: "Chess Club" },
        } as React.ChangeEvent<HTMLInputElement>);
      });

      act(() => {
        result.current.handleInputChange({
          target: { name: "category_id", value: "1" },
        } as React.ChangeEvent<HTMLSelectElement>);
      });

      act(() => {
        result.current.handleInputChange({
          target: { name: "max_participants", value: "15" },
        } as React.ChangeEvent<HTMLInputElement>);
      });

      expect(result.current.form).toEqual({
        name: "Chess Club",
        category_id: "1",
        max_participants: "15",
      });
    });
  });

  describe("validateForm", () => {
    it("should return error when name is empty", () => {
      const { result } = renderHook(() =>
        useActivityForm(
          { name: "", category_id: "1", max_participants: "10" },
          false,
        ),
      );

      const error = result.current.validateForm();

      expect(error).toBe("Activity name is required");
    });

    it("should return error when name is only whitespace", () => {
      const { result } = renderHook(() =>
        useActivityForm(
          { name: "   ", category_id: "1", max_participants: "10" },
          false,
        ),
      );

      const error = result.current.validateForm();

      expect(error).toBe("Activity name is required");
    });

    it("should return error when category_id is empty", () => {
      const { result } = renderHook(() =>
        useActivityForm(
          { name: "Soccer", category_id: "", max_participants: "10" },
          false,
        ),
      );

      const error = result.current.validateForm();

      expect(error).toBe("Please select a category");
    });

    it("should return error when max_participants is not a number", () => {
      const { result } = renderHook(() =>
        useActivityForm(
          { name: "Soccer", category_id: "1", max_participants: "abc" },
          false,
        ),
      );

      const error = result.current.validateForm();

      expect(error).toBe("Max participants must be a positive number");
    });

    it("should return error when max_participants is zero", () => {
      const { result } = renderHook(() =>
        useActivityForm(
          { name: "Soccer", category_id: "1", max_participants: "0" },
          false,
        ),
      );

      const error = result.current.validateForm();

      expect(error).toBe("Max participants must be a positive number");
    });

    it("should return error when max_participants is negative", () => {
      const { result } = renderHook(() =>
        useActivityForm(
          { name: "Soccer", category_id: "1", max_participants: "-5" },
          false,
        ),
      );

      const error = result.current.validateForm();

      expect(error).toBe("Max participants must be a positive number");
    });

    it("should allow decimals (parseInt truncates them)", () => {
      const { result } = renderHook(() =>
        useActivityForm(
          { name: "Soccer", category_id: "1", max_participants: "10.5" },
          false,
        ),
      );

      // parseInt("10.5", 10) returns 10, so validation passes
      const error = result.current.validateForm();

      expect(error).toBeNull();
    });

    it("should return error when max_participants is empty", () => {
      const { result } = renderHook(() =>
        useActivityForm(
          { name: "Soccer", category_id: "1", max_participants: "" },
          false,
        ),
      );

      const error = result.current.validateForm();

      expect(error).toBe("Max participants must be a positive number");
    });

    it("should return null when all fields are valid", () => {
      const { result } = renderHook(() =>
        useActivityForm(
          { name: "Soccer", category_id: "1", max_participants: "20" },
          false,
        ),
      );

      const error = result.current.validateForm();

      expect(error).toBeNull();
    });

    it("should return null when name has leading/trailing spaces but is not empty", () => {
      const { result } = renderHook(() =>
        useActivityForm(
          { name: "  Soccer  ", category_id: "1", max_participants: "20" },
          false,
        ),
      );

      const error = result.current.validateForm();

      expect(error).toBeNull();
    });

    it("should validate correctly after form updates", () => {
      const { result } = renderHook(() => useActivityForm(initialForm, false));

      // Initially invalid (empty name)
      let error = result.current.validateForm();
      expect(error).toBe("Activity name is required");

      // Update name
      act(() => {
        result.current.setForm({
          name: "Chess",
          category_id: "",
          max_participants: "10",
        });
      });

      // Now category is missing
      error = result.current.validateForm();
      expect(error).toBe("Please select a category");

      // Update category
      act(() => {
        result.current.setForm({
          name: "Chess",
          category_id: "1",
          max_participants: "10",
        });
      });

      // Now valid
      error = result.current.validateForm();
      expect(error).toBeNull();
    });
  });

  describe("loadCategories callback", () => {
    it("should manually load categories when called", async () => {
      vi.mocked(getCategories).mockResolvedValue(mockCategories);

      const { result } = renderHook(() => useActivityForm(initialForm, false));

      expect(result.current.categories).toEqual([]);

      await act(async () => {
        await result.current.loadCategories();
      });

      expect(result.current.categories).toEqual(mockCategories);
      expect(getCategories).toHaveBeenCalledTimes(1);
    });

    it("should set loading state during manual load", async () => {
      let resolveCategories: (value: ActivityCategory[]) => void = () =>
        undefined;
      const categoriesPromise = new Promise<ActivityCategory[]>(
        (resolve) => (resolveCategories = resolve),
      );
      vi.mocked(getCategories).mockReturnValue(categoriesPromise);

      const { result } = renderHook(() => useActivityForm(initialForm, false));

      expect(result.current.loading).toBe(false);

      act(() => {
        void result.current.loadCategories();
      });

      expect(result.current.loading).toBe(true);

      await act(async () => {
        resolveCategories(mockCategories);
        await categoriesPromise;
      });

      expect(result.current.loading).toBe(false);
      expect(result.current.categories).toEqual(mockCategories);
    });

    it("should handle errors during manual load", async () => {
      const error = new Error("API error");
      vi.mocked(getCategories).mockRejectedValue(error);

      const { result } = renderHook(() => useActivityForm(initialForm, false));

      await act(async () => {
        await result.current.loadCategories();
      });

      expect(result.current.error).toBe("Failed to load categories");
      expect(result.current.loading).toBe(false);
      expect(console.error).toHaveBeenCalledWith(
        "Failed to load categories:",
        error,
      );
    });

    it("should not reload categories multiple times on rapid isOpen changes", async () => {
      vi.mocked(getCategories).mockResolvedValue(mockCategories);

      const { rerender } = renderHook(
        ({ isOpen }) => useActivityForm(initialForm, isOpen),
        { initialProps: { isOpen: false } },
      );

      // Open and close rapidly
      rerender({ isOpen: true });
      rerender({ isOpen: false });
      rerender({ isOpen: true });

      await waitFor(() => {
        // Should be called twice (once for each true transition)
        expect(getCategories).toHaveBeenCalledTimes(2);
      });
    });
  });

  describe("setForm", () => {
    it("should allow manual form updates", () => {
      const { result } = renderHook(() => useActivityForm(initialForm, false));

      const newForm: ActivityFormState = {
        name: "Updated Activity",
        category_id: "3",
        max_participants: "30",
      };

      act(() => {
        result.current.setForm(newForm);
      });

      expect(result.current.form).toEqual(newForm);
    });

    it("should allow partial form updates using callback", () => {
      const { result } = renderHook(() =>
        useActivityForm(
          { name: "Soccer", category_id: "1", max_participants: "10" },
          false,
        ),
      );

      act(() => {
        result.current.setForm((prev) => ({
          ...prev,
          max_participants: "50",
        }));
      });

      expect(result.current.form).toEqual({
        name: "Soccer",
        category_id: "1",
        max_participants: "50",
      });
    });
  });
});
