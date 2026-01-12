"use client";

import { useRef, useEffect } from "react";
import { useModal } from "~/components/dashboard/modal-context";

/**
 * Hook to manage modal blur overlay effect.
 * Registers/unregisters with ModalContext to trigger background blur when modal is open.
 * Uses refs to avoid StrictMode double-effect issues.
 *
 * @param isOpen - Whether the modal is currently open
 */
export function useModalBlurEffect(isOpen: boolean): void {
  const { openModal, closeModal } = useModal();

  // Use refs to store stable references to the functions
  // This avoids issues with React StrictMode double-invocation
  const openModalRef = useRef(openModal);
  const closeModalRef = useRef(closeModal);

  // Keep refs in sync with latest values
  openModalRef.current = openModal;
  closeModalRef.current = closeModal;

  useEffect(() => {
    if (isOpen) {
      openModalRef.current();
      return () => {
        closeModalRef.current();
      };
    }
  }, [isOpen]);
}
