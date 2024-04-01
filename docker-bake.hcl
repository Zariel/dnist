variable "GO_VERSION" {
    default = "latest"
}

group "default" {
    targets = ["dnist"]
}

target "dnist" {
    dockerfile = "cmd/dnist/Dockerfile"
    contexts = {
        base = "docker-image://golang:${GO_VERSION}"
        distroless = "docker-image://gcr.io/distroless/static:latest"
    }
}
