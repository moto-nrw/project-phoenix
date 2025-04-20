import Link from 'next/link';

export default function HomePage() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <div className="text-center">
        <h1 className="text-4xl font-bold">Welcome to Project Phoenix</h1>
        <p className="mt-3 text-xl">A modern full-stack application</p>
        
        <div className="mt-8">
          <Link 
            href="/login" 
            className="inline-block rounded-md bg-blue-600 px-6 py-3 text-white shadow-md hover:bg-blue-700"
          >
            Sign in
          </Link>
        </div>
      </div>
    </div>
  );
}