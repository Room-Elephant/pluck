# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 produces a fully static binary.
# -ldflags="-s -w" strips the symbol table and DWARF info (~30 % size reduction).
RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w" \
      -o pluck \
      ./cmd/pluck

# ── Final stage ───────────────────────────────────────────────────────────────
FROM scratch

COPY --from=builder /app/pluck /pluck
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/rules.example /config/rules.conf

ENTRYPOINT ["/pluck"]
