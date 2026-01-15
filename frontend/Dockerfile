# syntax=docker/dockerfile:1
FROM node:20-alpine AS base

# Install timezone data for proper timezone support
RUN apk add --no-cache tzdata

# Install dependencies needed for all stages
FROM base AS deps
WORKDIR /app
COPY package.json package-lock.json ./
# Note: Removed npm cache mount - it can become stale and skip installing new packages
RUN npm ci --only=production

# Development dependencies
FROM base AS dev-deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

# Development stage
FROM base AS development
WORKDIR /app

# Copy dev dependencies from dev-deps stage
COPY --from=dev-deps /app/node_modules ./node_modules

# Copy source files
COPY . .

# Set development environment
ENV NODE_ENV=development
ENV NEXT_TELEMETRY_DISABLED=1

EXPOSE 3000

CMD ["npm", "run", "dev"]

# Builder stage for production
FROM base AS builder
WORKDIR /app

# Copy dependencies from dev-deps (includes dev dependencies needed for build)
COPY --from=dev-deps /app/node_modules ./node_modules

# Copy source files
COPY . .

# Build the application
ENV NEXT_TELEMETRY_DISABLED=1
ENV NODE_ENV=production
RUN npm run build

# Production stage - minimal image
FROM base AS production
WORKDIR /app

# Set production environment
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

# Copy only production dependencies
COPY --from=deps /app/node_modules ./node_modules

# Copy built application
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/package.json ./package.json
COPY --from=builder /app/package-lock.json ./package-lock.json

# Create non-root user for security
RUN addgroup -g 1001 -S nodejs && \
    adduser -S nextjs -u 1001
USER nextjs

EXPOSE 3000

CMD ["npm", "start"]