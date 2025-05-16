"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Image from "next/image";
import { signOut } from "next-auth/react";
import { Card, CardHeader, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { useSession } from "next-auth/react";

export default function LogoutPage() {
  const router = useRouter();
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const { data: session } = useSession();
  const userName = session?.user?.name ?? "Benutzer";

  const launchConfetti = () => {
    // Create a simple CSS-based confetti effect
    const confettiContainer = document.createElement('div');
    confettiContainer.style.position = 'fixed';
    confettiContainer.style.width = '100%';
    confettiContainer.style.height = '100%';
    confettiContainer.style.top = '0';
    confettiContainer.style.left = '0';
    confettiContainer.style.pointerEvents = 'none';
    confettiContainer.style.zIndex = '9999';
    document.body.appendChild(confettiContainer);

    // Colors for the confetti
    const colors = ['#FF3130', '#F78C10', '#83DC2D', '#5080D8'];

    // Get the center position of the screen
    const centerX = window.innerWidth / 2;
    const centerY = window.innerHeight / 2;

    // Create and animate 150 confetti pieces
    for (let i = 0; i < 150; i++) {
      setTimeout(() => {
        const confetti = document.createElement('div');
        const color = colors[Math.floor(Math.random() * colors.length)];

        // Style the confetti piece
        confetti.style.position = 'absolute';
        confetti.style.width = `${Math.random() * 10 + 5}px`;
        confetti.style.height = `${Math.random() * 5 + 5}px`;
        confetti.style.backgroundColor = color ?? '#FF3130';
        confetti.style.borderRadius = Math.random() > 0.5 ? '50%' : '0';
        confetti.style.opacity = '0.8';

        // Position at the center of the screen
        confetti.style.left = `${centerX}px`;
        confetti.style.top = `${centerY}px`;

        // Add to container
        confettiContainer.appendChild(confetti);

        // Animate - slower movement by increasing duration
        const animation = confetti.animate(
            [
              {
                transform: 'translate(-50%, -50%) rotate(0deg)',
                opacity: 0.8
              },
              {
                transform: `translate(${Math.random() * 200 - 100}px, ${Math.random() * 200 - 100}px) rotate(${Math.random() * 360}deg)`,
                opacity: 0.6
              },
              {
                transform: `translate(${Math.random() * 400 - 200}px, ${Math.random() * 400 - 200}px) rotate(${Math.random() * 720}deg)`,
                opacity: 0
              }
            ],
            {
              duration: Math.random() * 4000 + 3000, // Increased duration for slower movement
              easing: 'cubic-bezier(0.2, 0.8, 0.2, 1)'
            }
        );

        // Remove element after animation completes
        animation.onfinish = () => {
          confetti.remove();
          // Remove container when last animation finishes
          if (confettiContainer.children.length === 0) {
            confettiContainer.remove();
          }
        };
      }, Math.random() * 500); // Increased stagger time for a more spread out effect
    }
  };

  const handleConfirmLogout = async () => {
    await signOut({ redirect: false });
    setIsLoggingOut(true);
    launchConfetti();

    // Reduced redirect timer
    setTimeout(() => {
      router.push("/");
    }, 1500);
  };

  const handleCancelLogout = () => {
    router.push("/dashboard");
  };

  return (
      <div className="flex min-h-screen items-center justify-center p-4">
        <Card className="w-full max-w-md">
          {isLoggingOut ? (
              <>
                <CardHeader
                    title="Abgemeldet"
                    description="Sie wurden erfolgreich abgemeldet."
                />

                <CardContent>
                  <div className="flex flex-col items-center justify-center space-y-6">
                    <div className="relative h-32 w-32 animate-bounce">
                      <Image
                          src="/images/moto_transparent.png"
                          alt="Logo"
                          fill
                          className="object-contain transition-all duration-300"
                      />
                    </div>
                    <div className="text-center">
                      <p className="mb-4 text-xl font-medium text-teal-600 animate-pulse">
                        Bis bald, {userName}!
                      </p>
                      <p className="text-gray-600">
                        Sie werden zur Anmeldeseite weitergeleitet...
                      </p>
                    </div>
                  </div>
                </CardContent>
              </>
          ) : (
              <>
                <CardHeader
                    title="Abmelden"
                    description="Sind Sie sicher, dass Sie sich abmelden möchten?"
                />

                <CardContent>
                  <div className="flex flex-col items-center justify-center space-y-6">
                    <div className="relative h-32 w-32">
                      <Image
                          src="/images/moto_transparent.png"
                          alt="Logo"
                          fill
                          className="object-contain transition-all duration-300"
                      />
                    </div>

                    <div className="w-full space-y-2 text-center">
                      <p className="mb-6 text-gray-600">
                        Wenn Sie sich abmelden, werden Sie zur Anmeldeseite
                        weitergeleitet.
                      </p>
                      <div className="flex w-full flex-col gap-4 sm:flex-row">
                        <Button
                            variant="outline"
                            onClick={handleCancelLogout}
                            className="flex-1"
                        >
                          Zurück
                        </Button>
                        <Button onClick={handleConfirmLogout} className="flex-1">
                          Abmelden
                        </Button>
                      </div>
                    </div>
                  </div>
                </CardContent>
              </>
          )}
        </Card>
      </div>
  );
}