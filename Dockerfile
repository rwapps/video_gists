# Start from a Debian image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
FROM golang

# Copy the local package files to the container's workspace.
ADD . /go/src/github.com/rwapps/video_gists

# Build the twitterbot command and dependencies inside the container.
RUN go get "github.com/google/go-github/github"
RUN go install github.com/rwapps/video_gists

ENTRYPOINT /go/bin/video_gists
