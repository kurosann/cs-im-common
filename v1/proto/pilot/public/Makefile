sdkPath ?= ../../../../..
name ?=

gen:
	@if [ ! -f $(name).proto ]; then \
		echo "$(name).proto文件不存在"; \
		exit 1; \
	fi
	@if [ ! -f $(name)_dto.proto ]; then \
		echo "$(name)_dto.proto文件不存在"; \
		exit 1; \
	fi

	protoc --proto_path=. --go_out=${sdkPath} $(name).proto $(name)_dto.proto
	protoc --proto_path=. --go-grpc_out=${sdkPath} $(name).proto $(name)_dto.proto

	# 	protoc --proto_path=. --go_out=../../../../.. public_server.proto public_server_dto.proto public_server_base.proto
    #	protoc --proto_path=. --go-grpc_out=../../../../.. public_server.proto public_server_dto.proto public_server_base.proto

genpy:

	python3 -m grpc_tools.protoc -I. --python_out=${sdkPath} --pyi_out=${sdkPath} --grpc_python_out=${sdkPath} $(name).proto $(name)_dto.proto

	# protoc --proto_path=. --python_out=${sdkPath} $(name).proto $(name)_dto.proto $(name)_base.proto
	# protoc --proto_path=. --python-grpc_out=${sdkPath} $(name).proto $(name)_dto.proto $(name)_base.proto
