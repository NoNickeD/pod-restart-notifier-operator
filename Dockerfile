# Start from the official Golang base image
FROM golang:1.21 AS build

# Set the current working directory inside the container
WORKDIR /app

# Copy the Go mod and sum files to download dependencies
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o pod-restart-notifier .

# Start a new stage from scratch
FROM alpine:latest

WORKDIR /root/

# Copy the binary from the previous stage
COPY --from=build /app/pod-restart-notifier .

# Command to run the application
CMD ["./pod-restart-notifier"]