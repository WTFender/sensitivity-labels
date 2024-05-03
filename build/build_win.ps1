$env:outFile="./bin/labels.exe"
$env:entryFile="./cmd/labels/main.go"
$env:GOOS="windows"
$env:GOARCH="amd64"
go build -o $env:outFile $env:entryFile