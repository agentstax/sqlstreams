# Built from the repo root so the go.work workspace resolves the .bench module's
# import of the unpublished root module.
FROM golang:1.27 AS build
WORKDIR /src
COPY . .
RUN cd .bench && CGO_ENABLED=0 go build -o /reliability .

FROM gcr.io/distroless/static-debian12
COPY --from=build /reliability /reliability
ENTRYPOINT ["/reliability"]
