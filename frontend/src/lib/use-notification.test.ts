import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useNotification, getDbOperationMessage } from "./use-notification";

describe("useNotification", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe("initial state", () => {
    it("should initialize with null message and success type", () => {
      const { result } = renderHook(() => useNotification());

      expect(result.current.notification).toEqual({
        message: null,
        type: "success",
      });
    });
  });

  describe("showSuccess", () => {
    it("should set success notification", () => {
      const { result } = renderHook(() => useNotification());

      act(() => {
        result.current.showSuccess("Operation successful");
      });

      expect(result.current.notification).toEqual({
        message: "Operation successful",
        type: "success",
      });
    });

    it("should auto-hide after default duration", () => {
      const { result } = renderHook(() => useNotification());

      act(() => {
        result.current.showSuccess("Success message");
      });

      expect(result.current.notification.message).toBe("Success message");

      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(result.current.notification.message).toBeNull();
      expect(result.current.notification.type).toBe("success");
    });

    it("should auto-hide after custom duration", () => {
      const { result } = renderHook(() => useNotification(5000));

      act(() => {
        result.current.showSuccess("Success message");
      });

      expect(result.current.notification.message).toBe("Success message");

      act(() => {
        vi.advanceTimersByTime(4999);
      });

      expect(result.current.notification.message).toBe("Success message");

      act(() => {
        vi.advanceTimersByTime(1);
      });

      expect(result.current.notification.message).toBeNull();
    });

    it("should not auto-hide when duration is 0", () => {
      const { result } = renderHook(() => useNotification(0));

      act(() => {
        result.current.showSuccess("Persistent message");
      });

      expect(result.current.notification.message).toBe("Persistent message");

      act(() => {
        vi.advanceTimersByTime(10000);
      });

      expect(result.current.notification.message).toBe("Persistent message");
    });

    it("should not auto-hide when duration is negative", () => {
      const { result } = renderHook(() => useNotification(-1));

      act(() => {
        result.current.showSuccess("Persistent message");
      });

      expect(result.current.notification.message).toBe("Persistent message");

      act(() => {
        vi.advanceTimersByTime(10000);
      });

      expect(result.current.notification.message).toBe("Persistent message");
    });
  });

  describe("showError", () => {
    it("should set error notification", () => {
      const { result } = renderHook(() => useNotification());

      act(() => {
        result.current.showError("Operation failed");
      });

      expect(result.current.notification).toEqual({
        message: "Operation failed",
        type: "error",
      });
    });

    it("should auto-hide after duration", () => {
      const { result } = renderHook(() => useNotification());

      act(() => {
        result.current.showError("Error message");
      });

      expect(result.current.notification.message).toBe("Error message");

      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(result.current.notification.message).toBeNull();
    });
  });

  describe("showWarning", () => {
    it("should set warning notification", () => {
      const { result } = renderHook(() => useNotification());

      act(() => {
        result.current.showWarning("Please be careful");
      });

      expect(result.current.notification).toEqual({
        message: "Please be careful",
        type: "warning",
      });
    });

    it("should auto-hide after duration", () => {
      const { result } = renderHook(() => useNotification());

      act(() => {
        result.current.showWarning("Warning message");
      });

      expect(result.current.notification.message).toBe("Warning message");

      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(result.current.notification.message).toBeNull();
    });
  });

  describe("showInfo", () => {
    it("should set info notification", () => {
      const { result } = renderHook(() => useNotification());

      act(() => {
        result.current.showInfo("For your information");
      });

      expect(result.current.notification).toEqual({
        message: "For your information",
        type: "info",
      });
    });

    it("should auto-hide after duration", () => {
      const { result } = renderHook(() => useNotification());

      act(() => {
        result.current.showInfo("Info message");
      });

      expect(result.current.notification.message).toBe("Info message");

      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(result.current.notification.message).toBeNull();
    });
  });

  describe("hideNotification", () => {
    it("should manually hide notification", () => {
      const { result } = renderHook(() => useNotification());

      act(() => {
        result.current.showSuccess("Success message");
      });

      expect(result.current.notification.message).toBe("Success message");

      act(() => {
        result.current.hideNotification();
      });

      expect(result.current.notification.message).toBeNull();
      expect(result.current.notification.type).toBe("success");
    });

    it("should preserve notification type when hiding", () => {
      const { result } = renderHook(() => useNotification());

      act(() => {
        result.current.showError("Error message");
      });

      expect(result.current.notification.type).toBe("error");

      act(() => {
        result.current.hideNotification();
      });

      expect(result.current.notification.message).toBeNull();
      expect(result.current.notification.type).toBe("error");
    });
  });

  describe("multiple notifications", () => {
    it("should replace existing notification with new one", () => {
      const { result } = renderHook(() => useNotification());

      act(() => {
        result.current.showSuccess("First message");
      });

      expect(result.current.notification).toEqual({
        message: "First message",
        type: "success",
      });

      act(() => {
        result.current.showError("Second message");
      });

      expect(result.current.notification).toEqual({
        message: "Second message",
        type: "error",
      });
    });

    it("should handle rapid successive notifications", () => {
      const { result } = renderHook(() => useNotification(1000));

      act(() => {
        result.current.showSuccess("Message 1");
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      act(() => {
        result.current.showWarning("Message 2");
      });

      expect(result.current.notification).toEqual({
        message: "Message 2",
        type: "warning",
      });

      act(() => {
        vi.advanceTimersByTime(500);
      });

      // First timer might fire, but should be safe
      expect(result.current.notification.type).toBe("warning");

      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(result.current.notification.message).toBeNull();
    });
  });

  describe("callback stability", () => {
    it("should maintain callback references across re-renders", () => {
      const { result, rerender } = renderHook(() => useNotification(3000));

      const initialCallbacks = {
        showSuccess: result.current.showSuccess,
        showError: result.current.showError,
        showWarning: result.current.showWarning,
        showInfo: result.current.showInfo,
        hideNotification: result.current.hideNotification,
      };

      rerender();

      expect(result.current.showSuccess).toBe(initialCallbacks.showSuccess);
      expect(result.current.showError).toBe(initialCallbacks.showError);
      expect(result.current.showWarning).toBe(initialCallbacks.showWarning);
      expect(result.current.showInfo).toBe(initialCallbacks.showInfo);
      expect(result.current.hideNotification).toBe(
        initialCallbacks.hideNotification,
      );
    });

    it("should update callbacks when autoHideDuration changes", () => {
      const { result, rerender } = renderHook(
        ({ duration }) => useNotification(duration),
        { initialProps: { duration: 1000 } },
      );

      const initialCallbacks = {
        showSuccess: result.current.showSuccess,
      };

      rerender({ duration: 2000 });

      expect(result.current.showSuccess).not.toBe(initialCallbacks.showSuccess);
    });
  });
});

describe("getDbOperationMessage", () => {
  describe("create operation", () => {
    it("should return create message without identifier", () => {
      const message = getDbOperationMessage("create", "Gruppe");
      expect(message).toBe("Gruppe wurde erfolgreich erstellt");
    });

    it("should return create message with identifier", () => {
      const message = getDbOperationMessage("create", "Gruppe", "Klasse 5a");
      expect(message).toBe('Gruppe "Klasse 5a" wurde erfolgreich erstellt');
    });
  });

  describe("update operation", () => {
    it("should return update message without identifier", () => {
      const message = getDbOperationMessage("update", "Raum");
      expect(message).toBe("Raum wurde erfolgreich aktualisiert");
    });

    it("should return update message with identifier", () => {
      const message = getDbOperationMessage("update", "Raum", "Raum 101");
      expect(message).toBe('Raum "Raum 101" wurde erfolgreich aktualisiert');
    });
  });

  describe("delete operation", () => {
    it("should return delete message without identifier", () => {
      const message = getDbOperationMessage("delete", "Student");
      expect(message).toBe("Student wurde erfolgreich gelöscht");
    });

    it("should return delete message with identifier", () => {
      const message = getDbOperationMessage(
        "delete",
        "Student",
        "Max Mustermann",
      );
      expect(message).toBe(
        'Student "Max Mustermann" wurde erfolgreich gelöscht',
      );
    });
  });

  describe("edge cases", () => {
    it("should handle empty identifier as no identifier", () => {
      const message = getDbOperationMessage("create", "Gruppe", "");
      expect(message).toBe("Gruppe wurde erfolgreich erstellt");
    });

    it("should handle entity names with special characters", () => {
      const message = getDbOperationMessage("update", "Gruppe/Klasse", "5a");
      expect(message).toBe('Gruppe/Klasse "5a" wurde erfolgreich aktualisiert');
    });

    it("should handle identifiers with quotes", () => {
      const message = getDbOperationMessage("delete", "Raum", 'Raum "Spezial"');
      expect(message).toBe('Raum "Raum "Spezial"" wurde erfolgreich gelöscht');
    });
  });
});
