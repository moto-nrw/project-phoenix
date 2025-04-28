'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { env } from '~/env';
import {
  Card,
  CardHeader,
  CardContent,
  CardFooter,
  Input,
  Button,
  Alert,
  Link
} from '~/components/ui';

export default function RegisterPage() {
  const [formData, setFormData] = useState({
    email: '',
    username: '',
    name: '',
    password: '',
    confirmPassword: '',
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [serverError, setServerError] = useState('');
  const [success, setSuccess] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const router = useRouter();

  const validateForm = () => {
    const newErrors: Record<string, string> = {};
    
    if (!formData.email) {
      newErrors.email = 'Email is required';
    } else if (!/\S+@\S+\.\S+/.test(formData.email)) {
      newErrors.email = 'Email is invalid';
    }
    
    if (!formData.username) {
      newErrors.username = 'Username is required';
    } else if (!/^[a-zA-Z0-9]{3,30}$/.test(formData.username)) {
      newErrors.username = 'Username must be 3-30 alphanumeric characters';
    }
    
    if (!formData.name) {
      newErrors.name = 'Name is required';
    }
    
    if (!formData.password) {
      newErrors.password = 'Password is required';
    } else if (formData.password.length < 8) {
      newErrors.password = 'Password must be at least 8 characters';
    } else if (!/[A-Z]/.test(formData.password)) {
      newErrors.password = 'Password must contain an uppercase letter';
    } else if (!/[a-z]/.test(formData.password)) {
      newErrors.password = 'Password must contain a lowercase letter';
    } else if (!/[0-9]/.test(formData.password)) {
      newErrors.password = 'Password must contain a number';
    } else if (!/[^a-zA-Z0-9]/.test(formData.password)) {
      newErrors.password = 'Password must contain a special character';
    }
    
    if (formData.password !== formData.confirmPassword) {
      newErrors.confirmPassword = 'Passwords do not match';
    }
    
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData((prev) => ({ ...prev, [name]: value }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setServerError('');
    setSuccess('');
    
    if (!validateForm()) return;
    
    setIsLoading(true);
    
    try {
      // Use relative URL when in the browser to avoid CORS issues with Docker hostnames
      const apiUrl = typeof window !== 'undefined' ? '/api/auth/register' : `${env.NEXT_PUBLIC_API_URL}/auth/register`;
      
      const response = await fetch(apiUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email: formData.email,
          username: formData.username,
          name: formData.name,
          password: formData.password,
          confirm_password: formData.confirmPassword,
        }),
      });
      
      const responseData = await response.json() as {
        error?: string;
        message?: string;
        status?: string;
      };
      
      if (!response.ok) {
        // Extract the specific error message from the backend response
        let errorMessage = 'Registration failed';
        
        if (responseData.error) {
          // Handle specific error cases from the backend
          switch(responseData.error) {
            case 'email already exists':
              errorMessage = 'An account with this email already exists';
              break;
            case 'username already exists':
              errorMessage = 'This username is already taken';
              break;
            case 'password is too short (minimum 8 characters)':
              errorMessage = 'Password must be at least 8 characters';
              break;
            case 'password must contain at least one uppercase letter':
              errorMessage = 'Password must include an uppercase letter';
              break;
            case 'password must contain at least one lowercase letter':
              errorMessage = 'Password must include a lowercase letter';
              break;
            case 'password must contain at least one number':
              errorMessage = 'Password must include a number';
              break;
            case 'password must contain at least one special character':
              errorMessage = 'Password must include a special character';
              break;
            case 'passwords do not match':
              errorMessage = 'Passwords do not match';
              break;
            default:
              // If we have a specific error message from the backend, use it
              errorMessage = responseData.error;
          }
        } else if (responseData.message) {
          errorMessage = responseData.message;
        } else if (responseData.status) {
          errorMessage = responseData.status;
        }
        
        setServerError(errorMessage);
      } else {
        setSuccess('Account created successfully! Redirecting to login...');
        setTimeout(() => {
          router.push('/login');
        }, 2000);
      }
    } catch (error) {
      setServerError('An error occurred during registration');
      console.error(error);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <Card>
        <CardHeader 
          title="Create an account" 
          description="Fill in your details to create a new account" 
        />

        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-6">
            {serverError && <Alert type="error" message={serverError} />}
            {success && <Alert type="success" message={success} />}

            <div className="space-y-4">
              <Input
                label="Email address"
                name="email"
                type="email"
                autoComplete="email"
                required
                value={formData.email}
                onChange={handleChange}
                error={errors.email}
              />
              
              <Input
                label="Username"
                name="username"
                type="text"
                autoComplete="username"
                required
                value={formData.username}
                onChange={handleChange}
                error={errors.username}
              />

              <Input
                label="Full Name"
                name="name"
                type="text"
                autoComplete="name"
                required
                value={formData.name}
                onChange={handleChange}
                error={errors.name}
              />

              <Input
                label="Password"
                name="password"
                type="password"
                autoComplete="new-password"
                required
                value={formData.password}
                onChange={handleChange}
                error={errors.password}
              />

              <Input
                label="Confirm Password"
                name="confirmPassword"
                type="password"
                autoComplete="new-password"
                required
                value={formData.confirmPassword}
                onChange={handleChange}
                error={errors.confirmPassword}
              />
            </div>

            <Button
              type="submit"
              isLoading={isLoading}
              loadingText="Creating account..."
            >
              Create account
            </Button>
          </form>
        </CardContent>

        <CardFooter>
          <p>
            Already have an account?{' '}
            <Link href="/login">
              Sign in
            </Link>
          </p>
        </CardFooter>
      </Card>
    </div>
  );
}