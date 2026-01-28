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
  const [isModalOpen, setIsModalOpen] = useState(false);

  const openModal = useCallback(() => {
    setIsModalOpen(true);
  }, []);

  const closeModal = useCallback(() => {
    setIsModalOpen(false);
  }, []);

  const contextValue = useMemo(
    () => ({ isModalOpen, openModal, closeModal }),
    [isModalOpen, openModal, closeModal],
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
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  openModal: () => {},
  // eslint-disable-next-line @typescript-eslint/no-empty-function
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
