'use client';

import { useEffect, useRef } from 'react';

interface Ball {
  x: number;
  y: number;
  radius: number;
  dx: number;
  dy: number;
  color: string;
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
      const colors = ['#FF6B6B', '#4ECDC4', '#FFD166', '#6A0572'];
      ballsRef.current = [];
      
      for (let i = 0; i < 4; i++) {
        const radius = Math.random() * 50 + 30;
        ballsRef.current.push({
          x: Math.random() * canvas.width,
          y: Math.random() * canvas.height,
          radius,
          dx: (Math.random() - 0.5) * 0.2,
          dy: (Math.random() - 0.5) * 0.2,
          color: colors[i] ?? '#FF6B6B',
        });
      }
    };
    
    // Animation loop
    const animate = () => {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      
      // Draw and update each ball
      ballsRef.current.forEach(ball => {
        // Draw ball
        ctx.beginPath();
        ctx.arc(ball.x, ball.y, ball.radius, 0, Math.PI * 2);
        ctx.fillStyle = ball.color;
        ctx.globalAlpha = 0.5;
        ctx.fill();
        
        // Bounce off walls
        if (ball.x + ball.radius > canvas.width || ball.x - ball.radius < 0) {
          ball.dx = -ball.dx;
        }
        
        if (ball.y + ball.radius > canvas.height || ball.y - ball.radius < 0) {
          ball.dy = -ball.dy;
        }
        
        // Move ball
        ball.x += ball.dx;
        ball.y += ball.dy;
      });
      
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