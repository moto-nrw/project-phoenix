"use client";

import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
} from "react";

import { pushOverlay, useIsModalOpen } from "./overlay-stack";

/**
 * Read access to "is a dialog open" for surfaces below the dialogs (the
 * mobile master/detail drawer must not dismiss itself under one). The state
 * lives in the overlay stack; `Modal` registers there itself.
 *
 * `openModal`/`closeModal` put a dialog on the stack by hand for a surface
 * that is not a kit `Modal`. Outside `ModalProvider` they do nothing.
 */
interface ModalContextType {
  isModalOpen: boolean;
  openModal: () => void;
  closeModal: () => void;
}

interface ManualModals {
  open: () => void;
  close: () => void;
  releaseAll: () => void;
}

const ModalContext = createContext<ManualModals | undefined>(undefined);

export function ModalProvider({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  // One release per `openModal`, so stacked calls only report the dialog
  // closed once every one of them is closed again.
  const releases = useRef<Array<() => void>>([]);
  const manual = useMemo<ManualModals>(
    () => ({
      open: () => {
        releases.current.push(pushOverlay("modal"));
      },
      close: () => {
        releases.current.pop()?.();
      },
      releaseAll: () => {
        for (const release of releases.current.splice(0)) release();
      },
    }),
    [],
  );

  useEffect(() => manual.releaseAll, [manual]);

  return (
    <ModalContext.Provider value={manual}>{children}</ModalContext.Provider>
  );
}

const noop = () => {};

export function useModal(): ModalContextType {
  const manual = useContext(ModalContext);
  const isModalOpen = useIsModalOpen();
  const openModal = manual?.open ?? noop;
  const closeModal = manual?.close ?? noop;
  return useMemo(
    () => ({ isModalOpen, openModal, closeModal }),
    [isModalOpen, openModal, closeModal],
  );
}
