# Use the official Golang image as the base image
FROM golang:1.21.4

# Set the working directory inside the container
WORKDIR /app
COPY go.mod .
COPY go.sum .
# Only download updates if modules files have changed
RUN go mod download
# Copy all files from the top-level repository to /app
COPY . .

# Build the Golang application
RUN go build -o workflow ./workers/workflow/

# Expose a port for the application to listen on
# EXPOSE 8080

# Command to run the executable
CMD ["./workflow"]