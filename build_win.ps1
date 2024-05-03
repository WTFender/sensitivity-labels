$env:GOOS="windows"
$env:GOARCH="amd64"
go build -o "./bin/labels.exe" .\main.go