# syntax=docker/dockerfile:1

FROM base AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /dnist /app/cmd/dnist/

FROM distroless

COPY --from=build /dnist /

ENTRYPOINT ["/dnist"]
