import { render, renderHook, act } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import type { ReactNode } from "react";
import { AlertProvider, AlertContext } from "./AlertContext";
import { useContext } from "react";

function wrapper({ children }: { children: ReactNode }) {
  return <AlertProvider>{children}</AlertProvider>;
}

describe("AlertProvider", () => {
  it("renders children", () => {
    const { container } = render(
      <AlertProvider>
        <div data-testid="child">Hello</div>
      </AlertProvider>,
    );

    expect(container.querySelector('[data-testid="child"]')).toBeTruthy();
  });

  it("provides alert context to children", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    expect(result.current).toBeDefined();
    expect(result.current?.alertState).toBeDefined();
    expect(result.current?.showAlert).toBeTypeOf("function");
    expect(result.current?.hideAlert).toBeTypeOf("function");
  });
});

describe("AlertContext initial state", () => {
  it("starts with isShowing false", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    expect(result.current?.alertState.isShowing).toBe(false);
  });

  it("starts with undefined type", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    expect(result.current?.alertState.type).toBeUndefined();
  });

  it("starts with undefined message", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    expect(result.current?.alertState.message).toBeUndefined();
  });
});

describe("showAlert", () => {
  it("sets isShowing to true", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    act(() => {
      result.current?.showAlert("success", "Test message");
    });

    expect(result.current?.alertState.isShowing).toBe(true);
  });

  it("sets type to success", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    act(() => {
      result.current?.showAlert("success", "Test message");
    });

    expect(result.current?.alertState.type).toBe("success");
  });

  it("sets type to error", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    act(() => {
      result.current?.showAlert("error", "Error message");
    });

    expect(result.current?.alertState.type).toBe("error");
  });

  it("sets type to info", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    act(() => {
      result.current?.showAlert("info", "Info message");
    });

    expect(result.current?.alertState.type).toBe("info");
  });

  it("sets type to warning", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    act(() => {
      result.current?.showAlert("warning", "Warning message");
    });

    expect(result.current?.alertState.type).toBe("warning");
  });

  it("sets message correctly", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    act(() => {
      result.current?.showAlert("success", "Operation successful");
    });

    expect(result.current?.alertState.message).toBe("Operation successful");
  });

  it("updates alert when called multiple times", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    act(() => {
      result.current?.showAlert("success", "First message");
    });

    expect(result.current?.alertState.message).toBe("First message");

    act(() => {
      result.current?.showAlert("error", "Second message");
    });

    expect(result.current?.alertState.type).toBe("error");
    expect(result.current?.alertState.message).toBe("Second message");
  });

  it("maintains stable reference across renders", () => {
    const { result, rerender } = renderHook(() => useContext(AlertContext), {
      wrapper,
    });

    const firstShowAlert = result.current?.showAlert;

    rerender();

    expect(result.current?.showAlert).toBe(firstShowAlert);
  });
});

describe("hideAlert", () => {
  it("sets isShowing to false", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    act(() => {
      result.current?.showAlert("success", "Test message");
    });

    act(() => {
      result.current?.hideAlert();
    });

    expect(result.current?.alertState.isShowing).toBe(false);
  });

  it("sets type to undefined", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    act(() => {
      result.current?.showAlert("success", "Test message");
    });

    act(() => {
      result.current?.hideAlert();
    });

    expect(result.current?.alertState.type).toBeUndefined();
  });

  it("sets message to undefined", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    act(() => {
      result.current?.showAlert("success", "Test message");
    });

    act(() => {
      result.current?.hideAlert();
    });

    expect(result.current?.alertState.message).toBeUndefined();
  });

  it("can be called when alert is already hidden", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    expect(() => {
      act(() => {
        result.current?.hideAlert();
      });
    }).not.toThrow();

    expect(result.current?.alertState.isShowing).toBe(false);
  });

  it("maintains stable reference across renders", () => {
    const { result, rerender } = renderHook(() => useContext(AlertContext), {
      wrapper,
    });

    const firstHideAlert = result.current?.hideAlert;

    rerender();

    expect(result.current?.hideAlert).toBe(firstHideAlert);
  });
});

describe("alert lifecycle", () => {
  it("handles show -> hide -> show sequence", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    act(() => {
      result.current?.showAlert("success", "First alert");
    });

    expect(result.current?.alertState.isShowing).toBe(true);
    expect(result.current?.alertState.message).toBe("First alert");

    act(() => {
      result.current?.hideAlert();
    });

    expect(result.current?.alertState.isShowing).toBe(false);

    act(() => {
      result.current?.showAlert("error", "Second alert");
    });

    expect(result.current?.alertState.isShowing).toBe(true);
    expect(result.current?.alertState.type).toBe("error");
    expect(result.current?.alertState.message).toBe("Second alert");
  });

  it("handles rapid show calls without hide", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    act(() => {
      result.current?.showAlert("info", "Message 1");
      result.current?.showAlert("warning", "Message 2");
      result.current?.showAlert("error", "Message 3");
    });

    expect(result.current?.alertState.isShowing).toBe(true);
    expect(result.current?.alertState.type).toBe("error");
    expect(result.current?.alertState.message).toBe("Message 3");
  });
});

describe("context value memoization", () => {
  it("updates context value when alertState changes", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    const initialValue = result.current;

    act(() => {
      result.current?.showAlert("success", "Test");
    });

    const updatedValue = result.current;

    expect(updatedValue).not.toBe(initialValue);
  });

  it("maintains same functions across state changes", () => {
    const { result } = renderHook(() => useContext(AlertContext), { wrapper });

    const initialShowAlert = result.current?.showAlert;
    const initialHideAlert = result.current?.hideAlert;

    act(() => {
      result.current?.showAlert("success", "Test");
    });

    expect(result.current?.showAlert).toBe(initialShowAlert);
    expect(result.current?.hideAlert).toBe(initialHideAlert);
  });
});

describe("AlertContext without provider", () => {
  it("returns undefined when used outside provider", () => {
    const { result } = renderHook(() => useContext(AlertContext));

    expect(result.current).toBeUndefined();
  });
});

describe("multiple consumers", () => {
  it("shares state between multiple consumers", () => {
    function Consumer1() {
      const context = useContext(AlertContext);
      return (
        <button
          onClick={() => context?.showAlert("success", "From Consumer 1")}
        >
          Show Alert
        </button>
      );
    }

    function Consumer2() {
      const context = useContext(AlertContext);
      return <div>{context?.alertState.message}</div>;
    }

    const { container } = render(
      <AlertProvider>
        <Consumer1 />
        <Consumer2 />
      </AlertProvider>,
    );

    const button = container.querySelector("button");

    act(() => {
      button?.click();
    });

    const messageDiv = container.querySelector("div");
    expect(messageDiv?.textContent).toBe("From Consumer 1");
  });
});
