"use client";

import { useState, useCallback, useEffect } from "react";

/**
 * Return type for the useModalAnimation hook.
 */
interface UseModalAnimationReturn {
  /** Whether the modal entrance animation is active */
  isAnimating: boolean;
  /** Whether the modal exit animation is active */
  isExiting: boolean;
  /** Triggers exit animation and calls onClose after delay */
  handleClose: () => void;
}

/**
 * Custom hook for managing modal animation states.
 *
 * Centralizes the enter/exit animation logic used across modal components,
 * eliminating code duplication while maintaining consistent UX.
 *
 * @param isOpen - Whether the modal is currently open
 * @param onClose - Callback to execute after exit animation completes
 * @param animationDelay - Delay in ms before closing (default: 250ms)
 * @param entranceDelay - Delay in ms before triggering entrance animation (default: 10ms)
 *
 * @example
 * ```tsx
 * const { isAnimating, isExiting, handleClose } = useModalAnimation(isOpen, onClose);
 *
 * return (
 *   <ModalWrapper
 *     onClose={handleClose}
 *     isAnimating={isAnimating}
 *     isExiting={isExiting}
 *   >
 *     {children}
 *   </ModalWrapper>
 * );
 * ```
 */
export function useModalAnimation(
  isOpen: boolean,
  onClose: () => void,
  animationDelay = 250,
  entranceDelay = 10,
): UseModalAnimationReturn {
  const [isAnimating, setIsAnimating] = useState(false);
  const [isExiting, setIsExiting] = useState(false);

  // Handle modal close with exit animation
  const handleClose = useCallback(() => {
    setIsExiting(true);
    setIsAnimating(false);

    // Delay actual close to allow exit animation to complete
    setTimeout(() => {
      onClose();
    }, animationDelay);
  }, [onClose, animationDelay]);

  // Trigger entrance animation when modal opens
  useEffect(() => {
    if (isOpen) {
      // Small delay ensures smooth entrance animation after DOM paint
      const timer = setTimeout(() => {
        setIsAnimating(true);
      }, entranceDelay);

      return () => clearTimeout(timer);
    }
  }, [isOpen, entranceDelay]);

  // Reset animation states when modal closes
  useEffect(() => {
    if (!isOpen) {
      setIsAnimating(false);
      setIsExiting(false);
    }
  }, [isOpen]);

  return { isAnimating, isExiting, handleClose };
}
