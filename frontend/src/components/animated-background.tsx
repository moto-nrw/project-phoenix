'use client';

import { useEffect, useRef } from 'react';

interface Ball {
  x: number;
  y: number;
  radius: number;
  dx: number;
  dy: number;
  color: string;
  blur: number;
}

export function AnimatedBackground() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const animationRef = useRef<number | undefined>(undefined);
  const ballsRef = useRef<Ball[]>([]);
  
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    
    // Set canvas size
    const setCanvasSize = () => {
      canvas.width = window.innerWidth;
      canvas.height = window.innerHeight;
    };
    
    // Initialize balls
    const initBalls = () => {
      // Soft colors similar to the reference
      const colors = [
        '#FF8080', // red
        '#80D8FF', // blue
        '#A5D6A7', // green
        '#FFA726', // orange
      ];
      
      ballsRef.current = [];
      
      // Create bigger balls positioned strategically around the screen
      // Top left
      ballsRef.current.push({
        x: canvas.width * 0.2,
        y: canvas.height * 0.2,
        radius: Math.min(canvas.width, canvas.height) * 0.25,
        dx: 0.1,
        dy: 0.08,
        color: colors[0] ?? '#FF8080',
        blur: 40
      });
      
      // Top right
      ballsRef.current.push({
        x: canvas.width * 0.8,
        y: canvas.height * 0.2,
        radius: Math.min(canvas.width, canvas.height) * 0.2,
        dx: -0.12,
        dy: 0.09,
        color: colors[1] ?? '#80D8FF',
        blur: 35
      });
      
      // Bottom left
      ballsRef.current.push({
        x: canvas.width * 0.25,
        y: canvas.height * 0.8,
        radius: Math.min(canvas.width, canvas.height) * 0.22,
        dx: 0.08,
        dy: -0.1,
        color: colors[2] ?? '#A5D6A7',
        blur: 45
      });
      
      // Bottom right
      ballsRef.current.push({
        x: canvas.width * 0.8,
        y: canvas.height * 0.85,
        radius: Math.min(canvas.width, canvas.height) * 0.28,
        dx: -0.07,
        dy: -0.06,
        color: colors[3] ?? '#FFA726',
        blur: 50
      });
      
      // Add one in the center
      ballsRef.current.push({
        x: canvas.width * 0.5,
        y: canvas.height * 0.5,
        radius: Math.min(canvas.width, canvas.height) * 0.15,
        dx: 0.05,
        dy: -0.04,
        color: '#9575CD', // purple
        blur: 30
      });
    };
    
    // Animation loop
    const animate = () => {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      
      // Apply blur to the whole canvas
      ctx.filter = 'blur(30px)';
      
      // Draw and update each ball
      ballsRef.current.forEach(ball => {
        // Draw ball with gradient
        const gradient = ctx.createRadialGradient(
          ball.x, ball.y, 0, 
          ball.x, ball.y, ball.radius
        );
        
        gradient.addColorStop(0, ball.color);
        gradient.addColorStop(1, 'rgba(255,255,255,0)');
        
        ctx.beginPath();
        ctx.arc(ball.x, ball.y, ball.radius, 0, Math.PI * 2);
        ctx.fillStyle = gradient;
        ctx.globalAlpha = 0.7;
        ctx.fill();
        
        // Bounce off walls with padding
        const padding = ball.radius * 0.2;
        if (ball.x + ball.radius - padding > canvas.width || ball.x - ball.radius + padding < 0) {
          ball.dx = -ball.dx;
        }
        
        if (ball.y + ball.radius - padding > canvas.height || ball.y - ball.radius + padding < 0) {
          ball.dy = -ball.dy;
        }
        
        // Move ball very slowly
        ball.x += ball.dx;
        ball.y += ball.dy;
      });
      
      // Reset filter
      ctx.filter = 'none';
      
      animationRef.current = requestAnimationFrame(animate);
    };
    
    // Initialize and start animation
    setCanvasSize();
    initBalls();
    animate();
    
    // Handle window resize
    window.addEventListener('resize', () => {
      setCanvasSize();
      initBalls();
    });
    
    // Cleanup
    return () => {
      if (animationRef.current) {
        cancelAnimationFrame(animationRef.current);
      }
      window.removeEventListener('resize', setCanvasSize);
    };
  }, []);
  
  return (
    <canvas 
      ref={canvasRef}
      className="fixed inset-0 w-full h-full"
      style={{ zIndex: -10 }}
    />
  );
}