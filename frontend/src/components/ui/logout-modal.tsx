"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";
import { signOut } from "next-auth/react";
import { Modal } from "./modal";

interface LogoutModalProps {
  isOpen: boolean;
  onClose: () => void;
}

// Logout Icon als React Component
const LogOutIcon = ({ className }: { className?: string }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    width="48"
    height="48"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
  >
    <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
    <polyline points="16 17 21 12 16 7" />
    <line x1="21" x2="9" y1="12" y2="12" />
  </svg>
);

export function LogoutModal({ isOpen, onClose }: LogoutModalProps) {
  const router = useRouter();
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  // const { data: session } = useSession(); // Currently unused
  // const userName = session?.user?.name ?? "Benutzer"; // Currently unused

  const launchConfetti = () => {
    const confettiContainer = document.createElement('div');
    confettiContainer.style.position = 'fixed';
    confettiContainer.style.width = '100%';
    confettiContainer.style.height = '100%';
    confettiContainer.style.top = '0';
    confettiContainer.style.left = '0';
    confettiContainer.style.pointerEvents = 'none';
    confettiContainer.style.zIndex = '10000';
    document.body.appendChild(confettiContainer);

    const colors = ['#FF3130', '#F78C10', '#83CD2D', '#5080D8'];
    const centerX = window.innerWidth / 2;
    const centerY = window.innerHeight / 2;

    for (let i = 0; i < 100; i++) {
      setTimeout(() => {
        const confetti = document.createElement('div');
        const color = colors[Math.floor(Math.random() * colors.length)];

        confetti.style.position = 'absolute';
        confetti.style.width = `${Math.random() * 10 + 5}px`;
        confetti.style.height = `${Math.random() * 5 + 5}px`;
        confetti.style.backgroundColor = color ?? '#FF3130';
        confetti.style.borderRadius = Math.random() > 0.5 ? '50%' : '0';
        confetti.style.opacity = '0.8';
        confetti.style.left = `${centerX}px`;
        confetti.style.top = `${centerY}px`;

        confettiContainer.appendChild(confetti);

        const animation = confetti.animate(
          [
            {
              transform: 'translate(-50%, -50%) rotate(0deg)',
              opacity: 0.8
            },
            {
              transform: `translate(${Math.random() * 400 - 200}px, ${Math.random() * 400 - 200}px) rotate(${Math.random() * 720}deg)`,
              opacity: 0
            }
          ],
          {
            duration: Math.random() * 2000 + 2000,
            easing: 'cubic-bezier(0.2, 0.8, 0.2, 1)'
          }
        );

        animation.onfinish = () => {
          confetti.remove();
          if (confettiContainer.children.length === 0) {
            confettiContainer.remove();
          }
        };
      }, Math.random() * 300);
    }
  };

  const handleConfirmLogout = async () => {
    setIsLoggingOut(true);
    launchConfetti();
    
    // Immediately redirect to login page
    router.push("/");
    
    // Sign out in the background
    setTimeout(() => {
      void signOut({ redirect: false });
    }, 100);
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={isLoggingOut ? () => undefined : onClose}
      title="" // Leerer String zeigt nur X-Button ohne Titel
      footer={undefined} // Kein Footer
    >
      <div className="text-center">
          {isLoggingOut ? (
            <>
              <div className="mx-auto h-12 w-12 mb-4">
                <svg className="animate-spin text-[#5080d8]" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
              </div>
              <h2 className="text-2xl font-bold text-gray-800 mb-2">
                Abmelden...
              </h2>
              <p className="text-gray-600">
                Sie werden zur Anmeldeseite weitergeleitet
              </p>
            </>
          ) : (
            <>
              <LogOutIcon className="mx-auto h-12 w-12 text-gray-700 mb-4" />
              <h1 className="mt-4 text-3xl font-bold tracking-tight text-gray-800 sm:text-4xl">
                Abmelden
              </h1>
              <p className="mt-4 text-gray-600">
                Möchten Sie sich wirklich von Ihrem Konto abmelden?
              </p>
              <div className="mt-6">
                <button
                  onClick={handleConfirmLogout}
                  className="inline-flex items-center rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white shadow-sm transition-all duration-300 hover:scale-110 hover:shadow-2xl hover:shadow-gray-500/25 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2"
                >
                  <span>Abmelden</span>
                </button>
              </div>
            </>
          )}
        </div>
    </Modal>
  );
}