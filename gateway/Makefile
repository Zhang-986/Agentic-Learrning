.PHONY: run build test vet clean

# 运行开发服务器
run:
	go run cmd/server/main.go

# 编译
build:
	go build -o bin/gateway cmd/server/main.go

# 运行测试
test:
	go test ./... -v

# 静态分析
vet:
	go vet ./...

# 清理构建产物
clean:
	rm -rf bin/

# 格式化代码
fmt:
	go fmt ./...

# 安装依赖
deps:
	go mod tidy
