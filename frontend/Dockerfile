FROM node:20-alpine AS base

WORKDIR /app

# Install dependencies
COPY package.json package-lock.json ./
RUN npm ci

# Copy source files
COPY . .

ENV NEXT_TELEMETRY_DISABLED 1
ENV NODE_ENV=development

EXPOSE 3000

CMD ["npm", "run", "dev"]