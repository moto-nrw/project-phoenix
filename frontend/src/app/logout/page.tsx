'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import Image from 'next/image';
import Link from 'next/link';
import { signOut } from 'next-auth/react';
import { Card, CardHeader, CardContent, CardFooter } from '../../components/ui/card';
import { Button } from '../../components/ui/button';
import { BackgroundWrapper } from '../../components/background-wrapper';

export default function LogoutPage() {
  const router = useRouter();
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [countdown, setCountdown] = useState(3);

  useEffect(() => {
    let timer: NodeJS.Timeout;
    
    if (isLoggingOut && countdown > 0) {
      timer = setTimeout(() => {
        setCountdown(countdown - 1);
      }, 1000);
    } else if (isLoggingOut && countdown === 0) {
      handleConfirmLogout();
    }
    
    return () => {
      if (timer) clearTimeout(timer);
    };
  }, [isLoggingOut, countdown]);

  const handleConfirmLogout = async () => {
    await signOut({ redirect: false });
    router.push('/login');
  };

  const handleCancelLogout = () => {
    router.back();
  };

  const handleLogout = () => {
    setIsLoggingOut(true);
  };

  return (
    <BackgroundWrapper>
      <div className="flex min-h-screen items-center justify-center p-4">
        <Card className="max-w-md w-full">
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
              
              {isLoggingOut ? (
                <div className="text-center space-y-4">
                  <div className="relative h-24 w-24 mx-auto">
                    <svg 
                      className="animate-spin h-full w-full text-teal-500" 
                      xmlns="http://www.w3.org/2000/svg" 
                      fill="none" 
                      viewBox="0 0 24 24"
                    >
                      <circle 
                        className="opacity-25" 
                        cx="12" 
                        cy="12" 
                        r="10" 
                        stroke="currentColor" 
                        strokeWidth="4"
                      />
                      <path 
                        className="opacity-75" 
                        fill="currentColor" 
                        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                      />
                    </svg>
                    <div className="absolute inset-0 flex items-center justify-center">
                      <span className="text-2xl font-bold text-teal-600">{countdown}</span>
                    </div>
                  </div>
                  <p className="text-gray-600">Sie werden in {countdown} Sekunden abgemeldet...</p>
                  <Button 
                    variant="outline" 
                    onClick={handleCancelLogout}
                    className="mt-4"
                  >
                    Abbrechen
                  </Button>
                </div>
              ) : (
                <div className="text-center space-y-2 w-full">
                  <p className="text-gray-600 mb-6">
                    Wenn Sie sich abmelden, werden Sie zur Anmeldeseite weitergeleitet.
                  </p>
                  <div className="flex flex-col sm:flex-row gap-4 w-full">
                    <Button
                      variant="outline"
                      onClick={handleCancelLogout}
                      className="flex-1"
                    >
                      Zurück
                    </Button>
                    <Button
                      onClick={handleLogout}
                      className="flex-1"
                    >
                      Abmelden
                    </Button>
                  </div>
                </div>
              )}
            </div>
          </CardContent>
          
          <CardFooter>
            <div className="text-center w-full">
              <Link 
                href="/"
                className="text-teal-600 hover:text-teal-700 text-sm transition-colors"
              >
                Zurück zur Startseite
              </Link>
            </div>
          </CardFooter>
        </Card>
      </div>
    </BackgroundWrapper>
  );
}