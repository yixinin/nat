$env:GOOS="linux"
go build
$env:GOOS="windows"
go build
scp nat hw:nat