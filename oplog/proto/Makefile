sdkPath ?= .
name ?=

gen:
	@if [ -f $(name).proto ]; then \
		protoc --proto_path=. --go_out=${sdkPath} $(name).proto  --go-grpc_out=${sdkPath} $(name).proto; \
		mv vane-common/oplog/proto/oplog.pb.go oplog.pb.go; \
		mv vane-common/oplog/proto/oplog_grpc.pb.go oplog_grpc.pb.go; \
	fi
	@if [ -f $(name)_dto.proto ]; then \
		protoc --proto_path=. --go_out=${sdkPath} $(name)_dto.proto  --go-grpc_out=${sdkPath} $(name)_dto.proto; \
		mv vane-common/oplog/proto/oplog_dto.pb.go oplog_dto.pb.go; \
	fi
	@rm -rf tlg-common

genpy:
	python3 -m grpc_tools.protoc -I. --python_out=${sdkPath} --pyi_out=${sdkPath} --grpc_python_out=${sdkPath} $(name).proto $(name)_dto.proto $(name)_base.proto

	# protoc --proto_path=. --python_out=${sdkPath} $(name).proto $(name)_dto.proto $(name)_base.proto
	# protoc --proto_path=. --python-grpc_out=${sdkPath} $(name).proto $(name)_dto.proto $(name)_base.proto
