FROM node:24-alpine AS build
ARG NPM_REGISTRY=https://registry.npmjs.org/
WORKDIR /app
COPY package.json package-lock.json .npmrc ./
RUN npm ci --registry=${NPM_REGISTRY}
COPY . .
ARG VITE_TILE_URL=
ARG VITE_TILE_CRS=gcj02
RUN npm run build

FROM nginx:1.29-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
