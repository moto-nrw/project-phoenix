// app/page.tsx
"use client";

import { useState, useEffect } from "react";
import { signIn, useSession } from "next-auth/react";
import { useRouter, useSearchParams } from "next/navigation";
import Image from "next/image";
import { Input, Alert, HelpButton } from "~/components/ui";
import { Suspense } from "react";
import { refreshToken } from "~/lib/auth-api";
import { SmartRedirect } from "~/components/auth/smart-redirect";
import { SupervisionProvider } from "~/lib/supervision-context";
import { PasswordResetModal } from "~/components/ui/password-reset-modal";

import { Loading } from "~/components/ui/loading";
function LoginForm() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [showPassword, setShowPassword] = useState(false);
  const [awaitingRedirect, setAwaitingRedirect] = useState(false);
  const [isResetModalOpen, setIsResetModalOpen] = useState(false);
  const router = useRouter();
  const searchParams = useSearchParams();
  const { data: session, status } = useSession();

  // Check for valid session
  useEffect(() => {
    const checkAndRedirect = async () => {
      // If we have a valid session with access token, set up for redirect
      if (status === "authenticated" && session?.user?.token) {
        console.log("Valid session found, preparing smart redirect");
        setAwaitingRedirect(true);
        setCheckingAuth(false);
        return;
      }

      // If session is expired but we have a refresh token, try to refresh
      if (
        status === "authenticated" &&
        session?.user?.refreshToken &&
        !session?.user?.token
      ) {
        console.log(
          "Session expired but refresh token available, attempting refresh",
        );
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
              console.log("Token refreshed successfully");
              setAwaitingRedirect(true);
              setCheckingAuth(false);
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
  }, [status, session]);

  // Check for session errors in URL
  useEffect(() => {
    const urlError = searchParams.get("error");
    if (urlError === "SessionRequired") {
      setError("Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.");
    } else if (urlError === "SessionExpired") {
      setError("Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.");
    }
  }, [searchParams]);

  // Show loading while checking authentication or awaiting redirect
  if (
    checkingAuth ||
    status === "loading" ||
    (awaitingRedirect && status === "authenticated")
  ) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <Loading />
        {/* Smart redirect component when awaiting redirect */}
        {awaitingRedirect &&
          status === "authenticated" &&
          session?.user?.token && (
            <SmartRedirect
              onRedirect={(path) => {
                console.log(`Redirecting to ${path} based on user permissions`);
                router.push(path);
              }}
            />
          )}
      </div>
    );
  }

  const launchConfetti = () => {
    // Create a simple CSS-based confetti effect
    const confettiContainer = document.createElement("div");
    confettiContainer.style.position = "fixed";
    confettiContainer.style.width = "100%";
    confettiContainer.style.height = "100%";
    confettiContainer.style.top = "0";
    confettiContainer.style.left = "0";
    confettiContainer.style.pointerEvents = "none";
    confettiContainer.style.zIndex = "9999";
    document.body.appendChild(confettiContainer);

    // Colors for the confetti
    const colors = ["#FF3130", "#F78C10", "#83DC2D", "#5080D8"];

    // Get the logo position (instead of the center of the screen)
    const logoElement = document.querySelector(".mb-8.flex.justify-center img");
    const logoRect = logoElement?.getBoundingClientRect();

    // Use logo position or fallback to center if logo not found
    const startX = logoRect
      ? logoRect.left + logoRect.width / 2
      : window.innerWidth / 2;
    const startY = logoRect
      ? logoRect.top + logoRect.height / 2
      : window.innerHeight / 2;

    // Create and animate 100 confetti pieces
    for (let i = 0; i < 100; i++) {
      // No delay for first 50 pieces, small delay for others
      const delay = i < 50 ? 0 : Math.random() * 100;

      setTimeout(() => {
        const confetti = document.createElement("div");
        const color = colors[Math.floor(Math.random() * colors.length)];

        // Style the confetti piece
        confetti.style.position = "absolute";
        confetti.style.width = `${Math.random() * 8 + 3}px`;
        confetti.style.height = `${Math.random() * 4 + 3}px`;
        confetti.style.backgroundColor = color ?? "#FF3130";
        confetti.style.borderRadius = Math.random() > 0.5 ? "50%" : "0";
        confetti.style.opacity = "0.8";

        // Position at the logo
        confetti.style.left = `${startX}px`;
        confetti.style.top = `${startY}px`;

        // Calculate a direction that avoids the logo center
        // This ensures confetti moves outward in all directions and doesn't fly back inward
        let angle = 0;
        // Divide the circle into 4 quadrants and pick a random angle within each quadrant
        const quadrant = Math.floor(Math.random() * 4);
        switch (quadrant) {
          case 0:
            angle = (Math.random() * Math.PI) / 2;
            break; // Top-right quadrant
          case 1:
            angle = Math.PI / 2 + (Math.random() * Math.PI) / 2;
            break; // Bottom-right quadrant
          case 2:
            angle = Math.PI + (Math.random() * Math.PI) / 2;
            break; // Bottom-left quadrant
          case 3:
            angle = (3 * Math.PI) / 2 + (Math.random() * Math.PI) / 2;
            break; // Top-left quadrant
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
              transform: "translate(-50%, -50%) rotate(0deg)",
              opacity: 0.8,
            },
            {
              transform: `translate(${midX}px, ${midY}px) rotate(${Math.random() * 360}deg)`,
              opacity: 0.6,
            },
            {
              transform: `translate(${endX}px, ${endY}px) rotate(${Math.random() * 720}deg)`,
              opacity: 0,
            },
          ],
          {
            duration: Math.random() * 1500 + 1500, // 1.5-3 seconds, slightly slower
            easing: "cubic-bezier(0.25, 0.46, 0.45, 0.94)", // More natural easing
          },
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
        const existingConfetti = document.querySelector(
          'div[style*="z-index: 9999"]',
        );
        if (existingConfetti) {
          existingConfetti.remove();
        }
        setError("Ungültige E-Mail oder Passwort");
      } else {
        // Set flag to indicate we're awaiting redirect
        setAwaitingRedirect(true);
        // Refresh the router to update session state
        router.refresh();
      }
    } catch (error) {
      // Clear confetti if there's an error
      const existingConfetti = document.querySelector(
        'div[style*="z-index: 9999"]',
      );
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
      <div className="mx-auto w-full max-w-2xl rounded-2xl bg-white/80 p-10 text-center shadow-xl backdrop-blur-md transition-all duration-300 hover:bg-white/90 hover:shadow-2xl">
        {/* Help Button */}
        <div className="absolute top-4 right-4">
          <HelpButton
            title="Hilfe"
            content={
              <div>
                <p>
                  Melden Sie sich mit Ihrem <strong>moto-Account</strong> an:
                </p>
                <ul className="mt-3 space-y-2">
                  <li>
                    • <strong>E-Mail:</strong> Ihre registrierte E-Mail-Adresse
                  </li>
                  <li>
                    • <strong>Passwort:</strong> Ihr persönliches Passwort
                  </li>
                </ul>
                <p className="mt-4">
                  <strong>Probleme beim Anmelden?</strong>
                </p>
                <ul className="mt-2 space-y-1 text-sm">
                  <li>
                    • Überprüfen Sie Ihre <strong>Internetverbindung</strong>
                  </li>
                  <li>
                    • Stellen Sie sicher, dass <strong>Caps Lock</strong>{" "}
                    deaktiviert ist
                  </li>
                  <li>
                    • Kontaktieren Sie den <strong>Support</strong> bei
                    anhaltenden Problemen
                  </li>
                </ul>
              </div>
            }
          />
        </div>

        {/* Logo Section */}
        <div className="mb-8 flex justify-center">
          <Image
            src="/images/moto_transparent.png"
            alt="MOTO Logo"
            width={200}
            height={80}
            priority
          />
        </div>

        {/* Welcome Text */}
        <h1 className="mb-2 bg-gradient-to-r from-[#5080d8] to-[#83cd2d] bg-clip-text text-4xl font-bold text-transparent md:text-5xl">
          Willkommen bei moto!
        </h1>
        <p className="mb-10 text-xl text-gray-700">Ganztag. Digital.</p>

        {/* Login Form */}
        <form onSubmit={handleSubmit} className="space-y-6">
          {error && <Alert type="error" message={error} />}

          <div className="space-y-4">
            <div className="text-left">
              <label
                htmlFor="email"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
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
              <label
                htmlFor="password"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Passwort
              </label>
              <div className="relative">
                <Input
                  id="password"
                  name="password"
                  type={showPassword ? "text" : "password"}
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full pr-10"
                  label={""}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute top-1/2 right-3 -translate-y-1/2 text-gray-500 transition-colors hover:text-gray-700"
                  aria-label={
                    showPassword ? "Passwort verbergen" : "Passwort anzeigen"
                  }
                >
                  {showPassword ? (
                    <svg
                      className="h-5 w-5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"
                      />
                    </svg>
                  ) : (
                    <svg
                      className="h-5 w-5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                      />
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                      />
                    </svg>
                  )}
                </button>
              </div>
            </div>

            {/* Forgot Password Link */}
            <div className="text-center">
              <button
                type="button"
                onClick={() => setIsResetModalOpen(true)}
                className="text-sm text-gray-600 transition-colors hover:text-gray-800 hover:underline focus:underline focus:outline-none"
              >
                Passwort vergessen?
              </button>
            </div>
          </div>

          <div className="mt-2 flex justify-center">
            <button
              type="submit"
              disabled={isLoading}
              className="group relative overflow-hidden rounded-xl bg-gray-900 px-8 py-2.5 text-sm font-semibold text-white transition-all duration-200 hover:bg-gray-800 focus:ring-2 focus:ring-gray-900 focus:ring-offset-2 focus:outline-none active:scale-95 disabled:cursor-not-allowed disabled:opacity-50"
            >
              <span className="relative z-10">
                {isLoading ? "Anmeldung läuft..." : "Anmelden"}
              </span>
            </button>
          </div>
        </form>
      </div>

      {/* Password Reset Modal */}
      <PasswordResetModal
        isOpen={isResetModalOpen}
        onClose={() => setIsResetModalOpen(false)}
      />
    </div>
  );
}

export default function HomePage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen flex-col items-center justify-center p-4">
          Loading...
        </div>
      }
    >
      <SupervisionProvider>
        <LoginForm />
      </SupervisionProvider>
    </Suspense>
  );
}
