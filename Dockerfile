FROM docker.io/oven/bun:1.3.11-alpine

WORKDIR /app

COPY package.json bun.lock ./
RUN bun install --frozen-lockfile --production

COPY . .

EXPOSE 7123

CMD ["bun", "run", "start"]
