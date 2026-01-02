"use client";

import { useEffect, useRef } from "react";

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

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    // Set canvas size
    const setCanvasSize = () => {
      canvas.width = window.innerWidth;
      canvas.height = window.innerHeight;
    };

    // Initialize balls
    const initBalls = () => {
      // Vibrant colors for dynamic animated background
      const colors = [
        "#FF8080", // red
        "#80D8FF", // blue
        "#A5D6A7", // green
        "#FFA726", // orange
      ];

      // Create bigger balls positioned strategically around the screen
      ballsRef.current = [
        // Top left
        {
          x: canvas.width * 0.2,
          y: canvas.height * 0.2,
          radius: Math.min(canvas.width, canvas.height) * 0.4,
          dx: 0.05,
          dy: 0.04,
          color: colors[0] ?? "#FF8080",
          blur: 40,
        },
        // Top right
        {
          x: canvas.width * 0.8,
          y: canvas.height * 0.2,
          radius: Math.min(canvas.width, canvas.height) * 0.35,
          dx: -0.06,
          dy: 0.045,
          color: colors[1] ?? "#80D8FF",
          blur: 35,
        },
        // Bottom left
        {
          x: canvas.width * 0.25,
          y: canvas.height * 0.8,
          radius: Math.min(canvas.width, canvas.height) * 0.38,
          dx: 0.04,
          dy: -0.05,
          color: colors[2] ?? "#A5D6A7",
          blur: 45,
        },
        // Bottom right
        {
          x: canvas.width * 0.8,
          y: canvas.height * 0.85,
          radius: Math.min(canvas.width, canvas.height) * 0.45,
          dx: -0.035,
          dy: -0.03,
          color: colors[3] ?? "#FFA726",
          blur: 50,
        },
      ];
    };

    // Animation loop
    const animate = () => {
      ctx.clearRect(0, 0, canvas.width, canvas.height);

      // Apply blur to the whole canvas
      ctx.filter = "blur(40px)";

      // Draw and update each ball
      ballsRef.current.forEach((ball) => {
        // Draw ball with gradient
        const gradient = ctx.createRadialGradient(
          ball.x,
          ball.y,
          0,
          ball.x,
          ball.y,
          ball.radius,
        );

        gradient.addColorStop(0, ball.color);
        gradient.addColorStop(1, "rgba(255,255,255,0)");

        ctx.beginPath();
        ctx.arc(ball.x, ball.y, ball.radius, 0, Math.PI * 2);
        ctx.fillStyle = gradient;
        ctx.globalAlpha = 0.35;
        ctx.fill();

        // Bounce off walls with padding
        const padding = ball.radius * 0.2;
        if (
          ball.x + ball.radius - padding > canvas.width ||
          ball.x - ball.radius + padding < 0
        ) {
          ball.dx = -ball.dx;
        }

        if (
          ball.y + ball.radius - padding > canvas.height ||
          ball.y - ball.radius + padding < 0
        ) {
          ball.dy = -ball.dy;
        }

        // Move ball very slowly
        ball.x += ball.dx;
        ball.y += ball.dy;
      });

      // Reset filter
      ctx.filter = "none";

      animationRef.current = requestAnimationFrame(animate);
    };

    // Initialize and start animation
    setCanvasSize();
    initBalls();
    animate();

    // Handle window resize
    window.addEventListener("resize", () => {
      setCanvasSize();
      initBalls();
    });

    // Cleanup
    return () => {
      if (animationRef.current) {
        cancelAnimationFrame(animationRef.current);
      }
      window.removeEventListener("resize", setCanvasSize);
    };
  }, []);

  return (
    <canvas
      ref={canvasRef}
      className="fixed inset-0 h-full w-full"
      style={{ zIndex: -10 }}
    />
  );
}
