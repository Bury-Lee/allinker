:: 设置 Go 模块代理
set GOPROXY=https://goproxy.cn,direct
set GO111MODULE=on

:: 编译 Windows x64 版本
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w" -trimpath -o ALLinker_windows_amd64.exe .

:: 编译 Windows x86 版本
set GOOS=windows
set GOARCH=386
set CGO_ENABLED=0
go build -ldflags="-s -w" -trimpath -o ALLinker_windows_386.exe .

:: 编译 Linux x64 版本
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w" -trimpath -o ALLinker_linux_amd64 .

:: 编译 Linux ARM64 版本
set GOOS=linux
set GOARCH=arm64
set CGO_ENABLED=0
go build -ldflags="-s -w" -trimpath -o ALLinker_linux_arm64 .

:: 编译 macOS Intel 版本
set GOOS=darwin
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w" -trimpath -o ALLinker_darwin_amd64 .

:: 编译 macOS ARM64 版本
set GOOS=darwin
set GOARCH=arm64
set CGO_ENABLED=0
go build -ldflags="-s -w" -trimpath -o ALLinker_darwin_arm64 .