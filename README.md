# Share File

Simple utility to spawn a small webserver that allows you to share one or multiple files

## Usage

```bash
share-file file1 file2 file3
# then open the browser to ip:8080
```

## Building

```bash
go mod download
CGO_ENABLED=0 go build
```
