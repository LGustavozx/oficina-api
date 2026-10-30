# Estágio 1: compila. Esta imagem tem o compilador Go e é pesada (~800 MB),
# mas ela não vai para produção.
FROM golang:1.26-alpine AS build
WORKDIR /src

# Copiar go.mod/go.sum antes do código: o Docker reaproveita o cache das
# dependências enquanto elas não mudarem, e o build fica bem mais rápido.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 gera um binário estático, que roda em imagem sem libc.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# Estágio 2: imagem final mínima, sem shell nem gerenciador de pacotes
# (menos superfície de ataque) e rodando como usuário sem privilégios.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
