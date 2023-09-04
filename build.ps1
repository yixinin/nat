$env:GOOS="linux"
$env:GOARCh="amd64"
go build
$env:GOOS="windows"
$env:GOARCh="amd64"
go build
scp nat hw:nat