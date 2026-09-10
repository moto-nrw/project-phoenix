import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { formErrorAttempt, formErrorMessage, useFormError } from "./form-error";

describe("useFormError", () => {
  it("counts every failed attempt, also with an unchanged message", () => {
    const { result } = renderHook(() => useFormError());
    expect(result.current[0]).toBeNull();

    act(() => result.current[1]("Bitte einen Titel eintragen."));
    expect(result.current[0]).toEqual({
      message: "Bitte einen Titel eintragen.",
      attempt: 1,
    });

    act(() => result.current[1]("Bitte einen Titel eintragen."));
    expect(result.current[0]?.attempt).toBe(2);

    act(() => result.current[1]("Speichern fehlgeschlagen."));
    expect(result.current[0]).toEqual({
      message: "Speichern fehlgeschlagen.",
      attempt: 3,
    });
  });

  it("clears on null and on an empty string", () => {
    const { result } = renderHook(() => useFormError());

    act(() => result.current[1]("Fehler"));
    act(() => result.current[1](null));
    expect(result.current[0]).toBeNull();

    act(() => result.current[1]("Fehler"));
    act(() => result.current[1](""));
    expect(result.current[0]).toBeNull();
  });

  it("keeps the setter stable across renders", () => {
    const { result, rerender } = renderHook(() => useFormError());
    const setError = result.current[1];
    rerender();
    expect(result.current[1]).toBe(setError);
  });
});

describe("formErrorMessage / formErrorAttempt", () => {
  it("reads strings and counted errors alike", () => {
    expect(formErrorMessage(null)).toBeNull();
    expect(formErrorMessage("")).toBeNull();
    expect(formErrorMessage("Text")).toBe("Text");
    expect(formErrorMessage({ message: "Text", attempt: 4 })).toBe("Text");
    expect(formErrorAttempt("Text")).toBe(0);
    expect(formErrorAttempt({ message: "Text", attempt: 4 })).toBe(4);
  });
});
