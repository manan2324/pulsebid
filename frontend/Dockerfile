FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
FROM deps AS build
COPY src ./src
COPY public ./public
COPY next.config.ts tsconfig.json eslint.config.mjs postcss.config.mjs components.json ./
COPY scripts/check-no-secrets.mjs scripts/check-bundles.mjs ./scripts/
ENV NEXT_TELEMETRY_DISABLED=1
# Build-only values; all pages render dynamically, real secrets are runtime-only.
RUN export PULSEBID_API_KEY=build-placeholder ADMIN_PASSWORD=build-placeholder SESSION_SECRET=build-placeholder-secret-32-characters \
    && npm run build && npm run check:secrets && npm run check:bundles
FROM node:22-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production PORT=3001 HOSTNAME=0.0.0.0 NEXT_TELEMETRY_DISABLED=1
RUN addgroup -S -g 1001 nodejs && adduser -S -u 1001 nextjs -G nodejs
COPY --from=build --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=build --chown=nextjs:nodejs /app/.next/static ./.next/static
COPY --from=build --chown=nextjs:nodejs /app/public ./public
COPY --chown=nextjs:nodejs scripts/healthcheck.mjs ./healthcheck.mjs
USER nextjs
EXPOSE 3001
HEALTHCHECK --interval=10s --timeout=5s --start-period=20s --retries=5 CMD ["node", "healthcheck.mjs"]
CMD ["node", "server.js"]
