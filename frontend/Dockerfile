# Stage 1: Build
FROM docker.io/library/node:22-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm ci

COPY . .
RUN npm run build

# Stage 2: Runtime
FROM docker.io/library/node:22-alpine

WORKDIR /app

COPY --from=builder /app/dist ./dist
COPY --from=builder /app/package.json \
                    /app/package-lock.json ./
RUN npm ci --omit=dev

COPY --chmod=755 docker-entrypoint.sh /docker-entrypoint.sh
RUN chown -R node:node /app /docker-entrypoint.sh

USER node

ENV NODE_ENV=production
ENV PORT=3000
ENV HOST=0.0.0.0
ENV FRONTEND_CONFIG=/etc/frontend/config.env

EXPOSE 3000

HEALTHCHECK NONE

ENTRYPOINT ["/docker-entrypoint.sh"]
