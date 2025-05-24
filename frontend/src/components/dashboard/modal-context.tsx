"use client";

import React, { createContext, useContext, useState, useCallback } from 'react';

interface ModalContextType {
  isModalOpen: boolean;
  openModal: () => void;
  closeModal: () => void;
}

const ModalContext = createContext<ModalContextType | undefined>(undefined);

export function ModalProvider({ children }: { children: React.ReactNode }) {
  const [isModalOpen, setIsModalOpen] = useState(false);

  const openModal = useCallback(() => {
    setIsModalOpen(true);
  }, []);

  const closeModal = useCallback(() => {
    setIsModalOpen(false);
  }, []);

  return (
    <ModalContext.Provider value={{ isModalOpen, openModal, closeModal }}>
      {children}
    </ModalContext.Provider>
  );
}

export function useModal() {
  const context = useContext(ModalContext);
  if (context === undefined) {
    // Return a no-op implementation if not within provider
    return {
      isModalOpen: false,
      // eslint-disable-next-line @typescript-eslint/no-empty-function
      openModal: () => {},
      // eslint-disable-next-line @typescript-eslint/no-empty-function
      closeModal: () => {}
    };
  }
  return context;
}