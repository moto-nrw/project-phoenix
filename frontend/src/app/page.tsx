import Link from "next/link";
import Image from "next/image";

export default function HomePage() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <div className="mx-auto max-w-2xl rounded-2xl bg-white/80 p-10 text-center shadow-xl backdrop-blur-md transition-all duration-300 hover:bg-white/90 hover:shadow-2xl">
        <div className="mb-6 flex justify-center">
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

        <h1 className="bg-gradient-to-r from-teal-600 to-blue-600 bg-clip-text text-5xl font-bold text-transparent">
          Welcome to Project Phoenix
        </h1>
        <p className="mt-5 text-2xl text-gray-700">
          A modern all-day-school solution!
        </p>

        <div className="mt-10 flex flex-col justify-center gap-4 sm:flex-row">
          <Link
            href="/login"
            className="inline-block rounded-md bg-gradient-to-r from-teal-500 to-blue-500 px-8 py-3 text-lg font-medium text-white shadow-md transition-all duration-200 hover:scale-[1.02] hover:from-teal-600 hover:to-blue-600 hover:shadow-lg"
          >
            Sign In
          </Link>
          <Link
            href="/register"
            className="inline-block rounded-md border border-teal-200 bg-white px-8 py-3 text-lg font-medium text-teal-600 shadow-md transition-all duration-200 hover:scale-[1.02] hover:bg-teal-50 hover:shadow-lg"
          >
            Register
          </Link>
        </div>
      </div>
    </div>
  );
}
