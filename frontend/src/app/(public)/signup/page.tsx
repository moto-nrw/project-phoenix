"use client";

import { Suspense, useEffect, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { SignupForm } from "~/components/auth/signup-form";
import { useSession } from "~/lib/auth-client";
import { Loading } from "~/components/ui/loading";

function LoadingState() {
  return <Loading fullPage={false} />;
}

function SignupContent() {
  const router = useRouter();
  const { data: session, isPending: isSessionLoading } = useSession();
  const [checkingAuth, setCheckingAuth] = useState(true);

  // Redirect if already logged in
  useEffect(() => {
    if (!isSessionLoading) {
      if (session?.user) {
        // User is already logged in, redirect to dashboard
        router.push("/dashboard");
      } else {
        setCheckingAuth(false);
      }
    }
  }, [isSessionLoading, session, router]);

  // Show loading while checking authentication
  if (checkingAuth || isSessionLoading) {
    return <Loading fullPage={false} />;
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <div className="mx-auto w-full max-w-2xl rounded-2xl bg-white/80 p-10 text-center shadow-xl backdrop-blur-md transition-all duration-300 hover:bg-white/90 hover:shadow-2xl">
        {/* Logo Section */}
        <div className="mb-8 flex justify-center">
          <Link href="/">
            <Image
              src="/images/moto_transparent.png"
              alt="moto Logo"
              width={200}
              height={80}
              priority
            />
          </Link>
        </div>

        {/* Header Text */}
        <h1 className="mb-2 bg-gradient-to-r from-[#5080d8] to-[#83cd2d] bg-clip-text text-4xl font-bold text-transparent md:text-5xl">
          Konto erstellen
        </h1>
        <p className="mb-10 text-xl text-gray-700">
          Registriere dich, um moto zu nutzen.
        </p>

        <SignupForm />
      </div>
    </div>
  );
}

export default function SignupPage() {
  return (
    <Suspense fallback={<LoadingState />}>
      <SignupContent />
    </Suspense>
  );
}
