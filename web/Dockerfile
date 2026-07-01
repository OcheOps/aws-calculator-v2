# syntax=docker/dockerfile:1.7
FROM node:22-bookworm AS build
WORKDIR /src
COPY package.json package-lock.json* ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY . .
RUN npm run build

FROM node:22-bookworm-slim
WORKDIR /app
ENV NODE_ENV=production
COPY --from=build /src/build ./build
COPY --from=build /src/package.json ./package.json
COPY --from=build /src/node_modules ./node_modules
EXPOSE 3000
CMD ["node", "build"]
