// app/page.tsx
"use client";

import { useState } from "react";
import { signIn } from "next-auth/react";
import { useRouter } from "next/navigation";
import Image from "next/image";
import {
  Input,
  Button,
  Alert,
  Link,
} from "~/components/ui";

export default function HomePage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const router = useRouter();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    try {
      const result = await signIn("credentials", {
        email,
        password,
        redirect: false,
      });

      if (result?.error) {
        setError("Ungültige E-Mail oder Passwort");
      } else {
        router.push("/dashboard");
        router.refresh();
      }
    } catch (error) {
      setError("Anmeldefehler. Bitte versuchen Sie es erneut.");
      console.error(error);
    } finally {
      setIsLoading(false);
    }
  };

  return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="mx-auto max-w-2xl w-full rounded-2xl bg-white/80 p-10 text-center shadow-xl backdrop-blur-md transition-all duration-300 hover:bg-white/90 hover:shadow-2xl">
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
            Willkommen bei MOTO!
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
                    autoComplete="email"
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    className="w-full" label={""}                />
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
                    className="w-full" label={""}                />
              </div>
            </div>

            <Button
                type="submit"
                isLoading={isLoading}
                loadingText="Anmeldung läuft..."
            >
              Anmelden
            </Button>
          </form>

          {/* Register Link */}
          <div className="mt-8 text-center text-sm text-gray-600">
            <p>
              Noch kein Account?{" "}
              <Link href="/register" className="font-medium text-teal-600 hover:text-teal-500">
                Jetzt registrieren
              </Link>
            </p>
          </div>
        </div>
      </div>
  );
}