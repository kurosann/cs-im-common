tidy:
	export GOPROXY=https://goproxy.cn; \
	go mod tidy

lint:
	command -v golangci-lint || (WORK=$(shell pwd) && cd /tmp && GO111MODULE=on go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && cd $(WORK))
	golangci-lint run --timeout=5m