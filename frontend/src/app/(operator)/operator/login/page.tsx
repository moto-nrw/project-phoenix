"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import Image from "next/image";
import { Input, Alert, HelpButton } from "~/components/ui";
import { Loading } from "~/components/ui/loading";
import { useOperatorAuth } from "~/lib/operator/auth-context";

export default function OperatorLoginPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const router = useRouter();
  const { login, isAuthenticated, isLoading: authLoading } = useOperatorAuth();

  // Redirect if already authenticated
  useEffect(() => {
    if (isAuthenticated && !authLoading) {
      router.push("/operator/suggestions");
    }
  }, [isAuthenticated, authLoading, router]);

  // Show loading while checking auth
  if (authLoading || isAuthenticated) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <Loading />
      </div>
    );
  }

  const launchConfetti = () => {
    const confettiContainer = document.createElement("div");
    confettiContainer.style.position = "fixed";
    confettiContainer.style.width = "100%";
    confettiContainer.style.height = "100%";
    confettiContainer.style.top = "0";
    confettiContainer.style.left = "0";
    confettiContainer.style.pointerEvents = "none";
    confettiContainer.style.zIndex = "9999";
    document.body.appendChild(confettiContainer);

    const colors = ["#FF3130", "#F78C10", "#83DC2D", "#5080D8"];

    const logoElement = document.querySelector(".mb-8.flex.justify-center img");
    const logoRect = logoElement?.getBoundingClientRect();

    const startX = logoRect
      ? logoRect.left + logoRect.width / 2
      : window.innerWidth / 2;
    const startY = logoRect
      ? logoRect.top + logoRect.height / 2
      : window.innerHeight / 2;

    for (let i = 0; i < 100; i++) {
      const delay = i < 50 ? 0 : Math.random() * 100;

      setTimeout(() => {
        if (typeof document === "undefined" || !confettiContainer.isConnected)
          return;

        const confetti = document.createElement("div");
        const color = colors[Math.floor(Math.random() * colors.length)];

        confetti.style.position = "absolute";
        confetti.style.width = `${Math.random() * 8 + 3}px`;
        confetti.style.height = `${Math.random() * 4 + 3}px`;
        confetti.style.backgroundColor = color ?? "#FF3130";
        confetti.style.borderRadius = Math.random() > 0.5 ? "50%" : "0";
        confetti.style.opacity = "0.8";

        confetti.style.left = `${startX}px`;
        confetti.style.top = `${startY}px`;

        let angle = 0;
        const quadrant = Math.floor(Math.random() * 4);
        switch (quadrant) {
          case 0:
            angle = (Math.random() * Math.PI) / 2;
            break;
          case 1:
            angle = Math.PI / 2 + (Math.random() * Math.PI) / 2;
            break;
          case 2:
            angle = Math.PI + (Math.random() * Math.PI) / 2;
            break;
          case 3:
            angle = (3 * Math.PI) / 2 + (Math.random() * Math.PI) / 2;
            break;
        }

        const distance = 150 + Math.random() * 200;
        const endX = Math.cos(angle) * distance;
        const endY = Math.sin(angle) * distance;

        const midDistance = distance * 0.6;
        const midX = Math.cos(angle) * midDistance;
        const midY = Math.sin(angle) * midDistance;

        confettiContainer.appendChild(confetti);

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
            duration: Math.random() * 1500 + 1500,
            easing: "cubic-bezier(0.25, 0.46, 0.45, 0.94)",
          },
        );

        animation.onfinish = () => {
          confetti.remove();
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
      launchConfetti();

      await login(email, password);
      // login() in auth context handles redirect
    } catch (err) {
      // Clear confetti on error
      const existingConfetti = document.querySelector(
        'div[style*="z-index: 9999"]',
      );
      if (existingConfetti) {
        existingConfetti.remove();
      }

      setError(
        err instanceof Error
          ? err.message
          : "Anmeldefehler. Bitte versuchen Sie es erneut.",
      );
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
                  Melden Sie sich mit Ihrem <strong>Operator-Account</strong>{" "}
                  an:
                </p>
                <ul className="mt-3 space-y-2">
                  <li>
                    • <strong>E-Mail:</strong> Ihre Operator E-Mail-Adresse
                  </li>
                  <li>
                    • <strong>Passwort:</strong> Ihr Operator-Passwort
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
        <h1
          className="mb-2 text-4xl font-bold md:text-5xl"
          style={{
            background: "linear-gradient(135deg, #5080d8, #83cd2d)",
            WebkitBackgroundClip: "text",
            backgroundClip: "text",
            WebkitTextFillColor: "transparent",
          }}
        >
          Willkommen bei moto
        </h1>
        <p className="mb-10 text-xl text-gray-700">Operator Dashboard</p>

        {/* Login Form */}
        <form onSubmit={handleSubmit} noValidate className="space-y-6">
          {error && <Alert type="error" message={error} />}

          <div className="space-y-4">
            <div className="text-left">
              <label
                htmlFor="operator-email"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                E-Mail-Adresse
              </label>
              <Input
                id="operator-email"
                name="email"
                type="email"
                autoComplete="username"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full"
                label=""
              />
            </div>

            <div className="text-left">
              <label
                htmlFor="operator-password"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Passwort
              </label>
              <div className="relative">
                <Input
                  id="operator-password"
                  name="password"
                  type={showPassword ? "text" : "password"}
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full pr-10"
                  label=""
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
    </div>
  );
}
