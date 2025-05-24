# syntax=docker/dockerfile:1
FROM node:20-alpine AS base

WORKDIR /app

# Install dependencies with cache mount
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --cache /root/.npm

# Copy source files
COPY . .
ENV NEXT_TELEMETRY_DISABLED=1
ENV NODE_ENV=development

EXPOSE 3000

CMD ["npm", "run", "dev"]