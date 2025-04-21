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
      // Expanded color palette with more soft colors
      const colors = [
        '#FF8080', // red
        '#80D8FF', // blue
        '#A5D6A7', // green
        '#FFA726', // orange
        '#9575CD', // purple
        '#F48FB1', // pink
        '#81DEEA', // cyan
        '#FFD54F', // amber
        '#7986CB', // indigo
        '#4DB6AC', // teal
        '#DCE775', // lime
      ];
      
      ballsRef.current = [];
      
      // Create completely random balls with varied sizes
      const createRandomBalls = (count: number, sizeMultiplierMin: number, sizeMultiplierMax: number) => {
        for (let i = 0; i < count; i++) {
          const randomX = Math.random() * canvas.width;
          const randomY = Math.random() * canvas.height;
          const randomColorIndex = Math.floor(Math.random() * colors.length);
          const sizeMultiplier = sizeMultiplierMin + Math.random() * (sizeMultiplierMax - sizeMultiplierMin);
          
          // Create larger, more random balls
          ballsRef.current.push({
            x: randomX,
            y: randomY,
            radius: Math.min(canvas.width, canvas.height) * sizeMultiplier,
            dx: (Math.random() - 0.5) * 0.15, // More varied movement
            dy: (Math.random() - 0.5) * 0.15,
            color: colors[randomColorIndex] || '#80D8FF', // Provide default color
            blur: 40 + Math.random() * 35 // Much higher blur values
          });
        }
      };
      
      // Create large background bubbles at half the size between current and previous version
      // Previous: ~0.2-0.25, Current: 0.25-0.45 → New: 0.225-0.35
      createRandomBalls(8, 0.225, 0.35);
      
      // Create medium bubbles at half size between versions
      // Previous: ~0.12-0.16, Current: 0.15-0.3 → New: 0.135-0.23
      createRandomBalls(10, 0.135, 0.23);
      
      // Create smaller bubbles at half size between versions
      // Previous: ~0.03-0.09, Current: 0.05-0.15 → New: 0.04-0.12
      createRandomBalls(12, 0.04, 0.12);
    };
    
    // Animation loop
    const animate = () => {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      
      // Apply blur to the whole canvas - middle ground between versions (was 25px, then 40px)
      ctx.filter = 'blur(32px)';
      
      // Draw and update each ball
      ballsRef.current.forEach(ball => {
        // Custom blur based on ball size - middle ground between versions
        const customBlur = Math.min(ball.blur, 60); // Cap at 60px (between 40 and 80)
        
        // Draw ball with enhanced gradient
        const gradient = ctx.createRadialGradient(
          ball.x, ball.y, 0, 
          ball.x, ball.y, ball.radius
        );
        
        // Gradient complexity in between previous and enhanced version
        gradient.addColorStop(0, ball.color);
        gradient.addColorStop(0.5, ball.color + 'A0'); // Semi-transparent midpoint
        gradient.addColorStop(0.8, ball.color + '50'); // More transparent as it expands
        gradient.addColorStop(1, 'rgba(255,255,255,0)'); // Fully transparent at edge
        
        ctx.beginPath();
        ctx.arc(ball.x, ball.y, ball.radius * 1.1, 0, Math.PI * 2); // 1.1x size (between 1.0 and 1.2)
        ctx.fillStyle = gradient;
        ctx.globalAlpha = 0.65; // Between 0.7 and 0.6
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