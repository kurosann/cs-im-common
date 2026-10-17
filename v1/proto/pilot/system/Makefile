sdkPath ?= ../../../../..
name ?=

gen:
	@if [ -f $(name).proto ]; then \
		protoc --proto_path=. --go_out=${sdkPath} $(name).proto  --go-grpc_out=${sdkPath} $(name).proto; \
	fi
	@if [ -f $(name)_dto.proto ]; then \
		protoc --proto_path=. --go_out=${sdkPath} $(name)_dto.proto  --go-grpc_out=${sdkPath} $(name)_dto.proto; \
	fi
	@if [ -f $(name)_base.proto ]; then \
		protoc --proto_path=. --go_out=${sdkPath} $(name)_base.proto  --go-grpc_out=${sdkPath} $(name)_base.proto; \
	fi
