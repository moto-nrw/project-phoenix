import { describe, it, expect } from "vitest";
import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { useModalBlurEffect } from "./useModalBlurEffect";
import { ModalProvider, useModal } from "~/components/dashboard/modal-context";

// Wrapper component to provide modal context
function ModalProviderWrapper({ children }: { children: ReactNode }) {
  return <ModalProvider>{children}</ModalProvider>;
}

// Helper hook to capture modal state
function useModalState() {
  const { isModalOpen, openModal, closeModal } = useModal();
  return { isModalOpen, openModal, closeModal };
}

describe("useModalBlurEffect", () => {
  it("should call openModal when isOpen changes to true", () => {
    // First, verify the context starts closed
    const { result: contextResult } = renderHook(() => useModalState(), {
      wrapper: ModalProviderWrapper,
    });

    expect(contextResult.current.isModalOpen).toBe(false);

    // Now render the hook with isOpen = true
    const { result: blurResult } = renderHook(
      ({ isOpen }) => {
        useModalBlurEffect(isOpen);
        return useModal();
      },
      {
        wrapper: ModalProviderWrapper,
        initialProps: { isOpen: true },
      },
    );

    // The modal context should now be open
    expect(blurResult.current.isModalOpen).toBe(true);
  });

  it("should call closeModal when isOpen changes to false", () => {
    const { result, rerender } = renderHook(
      ({ isOpen }) => {
        useModalBlurEffect(isOpen);
        return useModal();
      },
      {
        wrapper: ModalProviderWrapper,
        initialProps: { isOpen: true },
      },
    );

    // Initially open
    expect(result.current.isModalOpen).toBe(true);

    // Change to closed
    rerender({ isOpen: false });

    // Should now be closed
    expect(result.current.isModalOpen).toBe(false);
  });

  it("should not call openModal when isOpen is initially false", () => {
    const { result } = renderHook(
      ({ isOpen }) => {
        useModalBlurEffect(isOpen);
        return useModal();
      },
      {
        wrapper: ModalProviderWrapper,
        initialProps: { isOpen: false },
      },
    );

    expect(result.current.isModalOpen).toBe(false);
  });

  it("should handle multiple open/close cycles", () => {
    const { result, rerender } = renderHook(
      ({ isOpen }) => {
        useModalBlurEffect(isOpen);
        return useModal();
      },
      {
        wrapper: ModalProviderWrapper,
        initialProps: { isOpen: false },
      },
    );

    expect(result.current.isModalOpen).toBe(false);

    // Open
    rerender({ isOpen: true });
    expect(result.current.isModalOpen).toBe(true);

    // Close
    rerender({ isOpen: false });
    expect(result.current.isModalOpen).toBe(false);

    // Open again
    rerender({ isOpen: true });
    expect(result.current.isModalOpen).toBe(true);
  });

  it("should work without ModalProvider (uses fallback)", () => {
    // This should not throw even without provider
    const { result } = renderHook(({ isOpen }) => useModalBlurEffect(isOpen), {
      initialProps: { isOpen: true },
    });

    // Hook returns void, so just verify it doesn't crash
    expect(result.current).toBeUndefined();
  });

  it("should cleanup on unmount when modal is open", () => {
    const { result, unmount } = renderHook(
      ({ isOpen }) => {
        useModalBlurEffect(isOpen);
        return useModal();
      },
      {
        wrapper: ModalProviderWrapper,
        initialProps: { isOpen: true },
      },
    );

    expect(result.current.isModalOpen).toBe(true);

    // Unmount - cleanup should be called
    unmount();

    // After unmount, we can't check the context state directly,
    // but the cleanup effect should have run without error
  });
});
