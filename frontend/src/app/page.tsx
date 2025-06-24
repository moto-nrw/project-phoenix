// app/page.tsx
"use client";

import { useState, useEffect } from "react";
import { signIn, useSession } from "next-auth/react";
import { useRouter, useSearchParams } from "next/navigation";
import Image from "next/image";
import {
  Input,
  Button,
  Alert,
  HelpButton,
} from "~/components/ui";
import { Suspense } from "react";
import { refreshToken } from "~/lib/auth-api";

function LoginForm() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const router = useRouter();
  const searchParams = useSearchParams();
  const { data: session, status } = useSession();
  
  // Check for valid session and redirect to dashboard
  useEffect(() => {
    const checkAndRedirect = async () => {
      // If we have a valid session with access token, redirect immediately
      if (status === "authenticated" && session?.user?.token) {
        console.log("Valid session found, redirecting to dashboard");
        router.push("/dashboard");
        return;
      }
      
      // If session is expired but we have a refresh token, try to refresh
      if (status === "authenticated" && session?.user?.refreshToken && !session?.user?.token) {
        console.log("Session expired but refresh token available, attempting refresh");
        try {
          const newTokens = await refreshToken();
          if (newTokens) {
            // Update session with new tokens
            const result = await signIn("credentials", {
              redirect: false,
              internalRefresh: true,
              token: newTokens.access_token,
              refreshToken: newTokens.refresh_token,
            });
            
            if (!result?.error) {
              console.log("Token refreshed successfully, redirecting to dashboard");
              router.push("/dashboard");
              return;
            }
          }
        } catch (error) {
          console.error("Failed to refresh token:", error);
        }
      }
      
      // Only show login form if not authenticated
      if (status !== "loading") {
        setCheckingAuth(false);
      }
    };
    
    void checkAndRedirect();
  }, [status, session, router]);
  
  // Check for session errors in URL
  useEffect(() => {
    const urlError = searchParams.get("error");
    if (urlError === "SessionRequired") {
      setError("Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.");
    } else if (urlError === "SessionExpired") {
      setError("Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.");
    }
  }, [searchParams]);
  
  // Show loading while checking authentication
  if (checkingAuth || status === "loading") {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-teal-600 mx-auto mb-4"></div>
          <p className="text-gray-600">Überprüfe Anmeldestatus...</p>
        </div>
      </div>
    );
  }

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

    // Get the logo position (instead of the center of the screen)
    const logoElement = document.querySelector('.mb-8.flex.justify-center img');
    const logoRect = logoElement?.getBoundingClientRect();

    // Use logo position or fallback to center if logo not found
    const startX = logoRect ? logoRect.left + logoRect.width / 2 : window.innerWidth / 2;
    const startY = logoRect ? logoRect.top + logoRect.height / 2 : window.innerHeight / 2;

    // Create and animate 100 confetti pieces
    for (let i = 0; i < 100; i++) {
      // No delay for first 50 pieces, small delay for others
      const delay = i < 50 ? 0 : Math.random() * 100;

      setTimeout(() => {
        const confetti = document.createElement('div');
        const color = colors[Math.floor(Math.random() * colors.length)];

        // Style the confetti piece
        confetti.style.position = 'absolute';
        confetti.style.width = `${Math.random() * 8 + 3}px`;
        confetti.style.height = `${Math.random() * 4 + 3}px`;
        confetti.style.backgroundColor = color ?? '#FF3130';
        confetti.style.borderRadius = Math.random() > 0.5 ? '50%' : '0';
        confetti.style.opacity = '0.8';

        // Position at the logo
        confetti.style.left = `${startX}px`;
        confetti.style.top = `${startY}px`;

        // Calculate a direction that avoids the logo center
        // This ensures confetti moves outward in all directions and doesn't fly back inward
        let angle = 0;
        // Divide the circle into 4 quadrants and pick a random angle within each quadrant
        const quadrant = Math.floor(Math.random() * 4);
        switch (quadrant) {
          case 0: angle = Math.random() * Math.PI / 2; break;              // Top-right quadrant
          case 1: angle = Math.PI / 2 + Math.random() * Math.PI / 2; break;  // Bottom-right quadrant
          case 2: angle = Math.PI + Math.random() * Math.PI / 2; break;    // Bottom-left quadrant
          case 3: angle = 3 * Math.PI / 2 + Math.random() * Math.PI / 2; break;// Top-left quadrant
        }

        // Calculate end position using the angle - guarantee outward motion
        const distance = 150 + Math.random() * 200; // Between 150-350px from center
        const endX = Math.cos(angle) * distance;
        const endY = Math.sin(angle) * distance;

        // Calculate a mid-point that's also moving outward
        const midDistance = distance * 0.6;
        const midX = Math.cos(angle) * midDistance;
        const midY = Math.sin(angle) * midDistance;

        // Add to container
        confettiContainer.appendChild(confetti);

        // Animate with more controlled outward trajectory
        const animation = confetti.animate(
          [
            {
              transform: 'translate(-50%, -50%) rotate(0deg)',
              opacity: 0.8
            },
            {
              transform: `translate(${midX}px, ${midY}px) rotate(${Math.random() * 360}deg)`,
              opacity: 0.6
            },
            {
              transform: `translate(${endX}px, ${endY}px) rotate(${Math.random() * 720}deg)`,
              opacity: 0
            }
          ],
          {
            duration: Math.random() * 1500 + 1500, // 1.5-3 seconds, slightly slower
            easing: 'cubic-bezier(0.25, 0.46, 0.45, 0.94)' // More natural easing
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
      }, delay);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    try {
      // Start confetti immediately when button is clicked
      // This creates a perception of instant response
      launchConfetti();

      const result = await signIn("credentials", {
        email,
        password,
        redirect: false,
      });

      if (result?.error) {
        // If there's an error, clear existing confetti
        const existingConfetti = document.querySelector('div[style*="z-index: 9999"]');
        if (existingConfetti) {
          existingConfetti.remove();
        }
        setError("Ungültige E-Mail oder Passwort");
      } else {
        // Shorter redirect time - just enough to see confetti
        setTimeout(() => {
          router.push("/dashboard");
          router.refresh();
        }, 0);
      }
    } catch (error) {
      // Clear confetti if there's an error
      const existingConfetti = document.querySelector('div[style*="z-index: 9999"]');
      if (existingConfetti) {
        existingConfetti.remove();
      }

      setError("Anmeldefehler. Bitte versuchen Sie es erneut.");
      console.error(error);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <div className="mx-auto max-w-2xl w-full rounded-2xl bg-white/80 p-10 text-center shadow-xl backdrop-blur-md transition-all duration-300 hover:bg-white/90 hover:shadow-2xl">

        {/* Help Button */}
        <div className="absolute top-4 right-4">
          <HelpButton
            title="Hilfe"
            content={
              <div>
                <p>Melden Sie sich mit Ihrem <strong>moto-Account</strong> an:</p>
                <ul className="mt-3 space-y-2">
                  <li>• <strong>E-Mail:</strong> Ihre registrierte E-Mail-Adresse</li>
                  <li>• <strong>Passwort:</strong> Ihr persönliches Passwort</li>
                </ul>
                <p className="mt-4"><strong>Probleme beim Anmelden?</strong></p>
                <ul className="mt-2 space-y-1 text-sm">
                  <li>• Überprüfen Sie Ihre <strong>Internetverbindung</strong></li>
                  <li>• Stellen Sie sicher, dass <strong>Caps Lock</strong> deaktiviert ist</li>
                  <li>• Kontaktieren Sie den <strong>Support</strong> bei anhaltenden Problemen</li>
                </ul>
              </div>
            }
          />
        </div>

        {/* Logo Section */}
        <div className="mb-8 flex justify-center">
          <a
            href="https://www.moto.nrw"
            target="_blank"
            rel="noopener noreferrer"
            className="cursor-pointer transition-all duration-300 hover:scale-105"
          >
            <Image
              src="/images/moto_transparent.png"
              alt="MOTO Logo"
              width={200}
              height={80}
              priority
            />
          </a>
        </div>

        {/* Welcome Text */}
        <h1 className="bg-gradient-to-r from-teal-600 to-blue-600 bg-clip-text text-4xl md:text-5xl font-bold text-transparent mb-2">
          Willkommen bei moto!
        </h1>
        <p className="text-xl text-gray-700 mb-10">
          Ganztag. Digital.
        </p>

        {/* Login Form */}
        <form onSubmit={handleSubmit} className="space-y-6">
          {error && <Alert type="error" message={error} />}

          <div className="space-y-4">
            <div className="text-left">
              <label htmlFor="email" className="block text-sm font-medium text-gray-700 mb-1">
                E-Mail-Adresse
              </label>
              <Input
                id="email"
                name="email"
                type="email"
                autoComplete="username"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full"
                label={""}
              />
            </div>

            <div className="text-left">
              <label htmlFor="password" className="block text-sm font-medium text-gray-700 mb-1">
                Passwort
              </label>
              <Input
                id="password"
                name="password"
                type="password"
                autoComplete="current-password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="w-full"
                label={""}
              />
            </div>
          </div>

          <Button
            type="submit"
            isLoading={isLoading}
            loadingText="Anmeldung läuft..."
            size="lg"
          >
            Anmelden
          </Button>
        </form>

      </div>
    </div>
  );
}

export default function HomePage() {
  return (
    <Suspense fallback={<div className="flex min-h-screen flex-col items-center justify-center p-4">Loading...</div>}>
      <LoginForm />
    </Suspense>
  );
}
