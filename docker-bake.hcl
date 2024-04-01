variable "GO_VERSION" {
    default = "latest"
}

group "default" {
    targets = ["dnist"]
}

target "docker-metadata-action" {}

target "dnist" {
    inherits = ["docker-metadata-action"]
    dockerfile = "cmd/dnist/Dockerfile"
    contexts = {
        base = "docker-image://golang:${GO_VERSION}"
        distroless = "docker-image://gcr.io/distroless/static:latest"
    }
}
