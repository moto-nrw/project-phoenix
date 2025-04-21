import { NextRequest, NextResponse } from 'next/server';
import { env } from '~/env';

export async function POST(request: NextRequest) {
  try {
    // Forward the registration request to the backend
    const body = await request.json();
    
    console.log(`Forwarding registration request to ${env.NEXT_PUBLIC_API_URL}/auth/register`);
    
    const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/auth/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    
    // Log the response status for debugging
    console.log(`Registration response status: ${response.status}`);
    
    // Return the backend response directly
    const data = await response.json();
    return NextResponse.json(data, { status: response.status });
  } catch (error) {
    console.error('Registration error:', error);
    return NextResponse.json(
      { message: 'An error occurred during registration', error: String(error) },
      { status: 500 }
    );
  }
}