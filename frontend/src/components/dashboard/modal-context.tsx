"use client";

import React, {
  createContext,
  useContext,
  useState,
  useCallback,
  useMemo,
} from "react";

interface ModalContextType {
  isModalOpen: boolean;
  openModal: () => void;
  closeModal: () => void;
}

const ModalContext = createContext<ModalContextType | undefined>(undefined);

export function ModalProvider({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  // Counter rather than boolean so that stacked modals (e.g. an edit modal
  // opening a confirmation modal on top) only report `isModalOpen=false` once
  // every modal has unmounted. Otherwise the inner modal's cleanup would flip
  // the flag while the outer modal is still open, which would let the mobile
  // master/detail drawer dismiss itself out from under it.
  const [openCount, setOpenCount] = useState(0);

  const openModal = useCallback(() => {
    setOpenCount((count) => count + 1);
  }, []);

  const closeModal = useCallback(() => {
    setOpenCount((count) => (count > 0 ? count - 1 : 0));
  }, []);

  const contextValue = useMemo(
    () => ({ isModalOpen: openCount > 0, openModal, closeModal }),
    [openCount, openModal, closeModal],
  );

  return (
    <ModalContext.Provider value={contextValue}>
      {children}
    </ModalContext.Provider>
  );
}

// Stable fallback object to prevent re-renders when context is unavailable
const FALLBACK_CONTEXT: ModalContextType = {
  isModalOpen: false,

  openModal: () => {},

  closeModal: () => {},
};

export function useModal() {
  const context = useContext(ModalContext);
  if (context === undefined) {
    // Return stable no-op implementation if not within provider
    return FALLBACK_CONTEXT;
  }
  return context;
}
