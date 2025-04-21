import Link from 'next/link';
import { BackgroundWrapper } from '../components/background-wrapper';

export default function HomePage() {
  return (
    <BackgroundWrapper>
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="text-center max-w-2xl mx-auto bg-white/80 backdrop-blur-md rounded-2xl p-10 shadow-xl transition-all duration-300 hover:shadow-2xl hover:bg-white/90">
          <h1 className="text-5xl font-bold bg-gradient-to-r from-teal-600 to-blue-600 bg-clip-text text-transparent">
            Welcome to Project Phoenix
          </h1>
          <p className="mt-5 text-2xl text-gray-700">A modern all-day-school solution!</p>
          
          <div className="mt-10 flex flex-col sm:flex-row gap-4 justify-center">
            <Link 
              href="/login" 
              className="inline-block rounded-md bg-gradient-to-r from-teal-500 to-blue-500 px-8 py-3 text-white text-lg font-medium shadow-md transition-all duration-200 hover:shadow-lg hover:from-teal-600 hover:to-blue-600 hover:scale-[1.02]"
            >
              Sign In
            </Link>
            <Link
              href="/register"
              className="inline-block rounded-md bg-white px-8 py-3 text-teal-600 text-lg font-medium shadow-md border border-teal-200 transition-all duration-200 hover:shadow-lg hover:bg-teal-50 hover:scale-[1.02]"
            >
              Register
            </Link>
          </div>
        </div>
      </div>
    </BackgroundWrapper>
  );
}