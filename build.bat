:: allinker 跨平台编译脚本
:: 使用纯 Go SQLite 驱动，无需 CGO
set GOPROXY=https://goproxy.cn,direct
set GO111MODULE=on

:: 编译 Windows x64 版本
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w" -trimpath -o allinker_windows_amd64.exe .

:: 编译 Windows x86 版本
set GOOS=windows
set GOARCH=386
set CGO_ENABLED=0
go build -ldflags="-s -w" -trimpath -o allinker_windows_386.exe .

:: 编译 Linux x64 版本
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w" -trimpath -o allinker_linux_amd64 .

:: 编译 Linux ARM64 版本
set GOOS=linux
set GOARCH=arm64
set CGO_ENABLED=0
go build -ldflags="-s -w" -trimpath -o allinker_linux_arm64 .

:: 编译 macOS Intel 版本
set GOOS=darwin
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w" -trimpath -o allinker_darwin_amd64 .

:: 编译 macOS ARM64 版本
set GOOS=darwin
set GOARCH=arm64
set CGO_ENABLED=0
go build -ldflags="-s -w" -trimpath -o allinker_darwin_arm64 .