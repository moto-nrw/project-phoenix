"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import Image from "next/image";
import { signOut } from "next-auth/react";
import { Card, CardHeader, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";

export default function LogoutPage() {
  const router = useRouter();
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [countdown, setCountdown] = useState(3);

  useEffect(() => {
    let timer: NodeJS.Timeout;

    if (isLoggingOut && countdown > 0) {
      timer = setTimeout(() => {
        setCountdown((prev) => prev - 1);
      }, 1000);
    } else if (isLoggingOut && countdown === 0) {
      router.push("/login");
    }

    return () => {
      if (timer) clearTimeout(timer);
    };
  }, [isLoggingOut, countdown, router]);

  const handleConfirmLogout = async () => {
    await signOut({ redirect: false });
    setIsLoggingOut(true);
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
                <div className="relative h-32 w-32">
                  <Image
                    src="/images/moto_transparent.png"
                    alt="Logo"
                    fill
                    className="object-contain transition-all duration-300"
                  />
                </div>
                <div className="text-center">
                  <p className="mb-2 text-gray-600">
                    Sie werden zur Anmeldeseite weitergeleitet...
                  </p>
                  <div className="inline-flex items-center justify-center">
                    <span className="flex h-8 w-8 items-center justify-center rounded-full bg-teal-50 text-lg font-semibold text-teal-600">
                      {countdown}
                    </span>
                  </div>
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
