.PHONY: genNewProtofile check genbase gendto genproto

servername ?=
modulename ?=
protoname ?=
reponame ?= tlg-common

genNewProtofile: check
	@mkdir -p $(servername)/$(modulename)
	@if [ ! -f "$(servername)/$(modulename)/$(modulename)_base.proto" ]; then \
		make genbase; \
	fi

	@if [ ! -f "$(servername)/$(modulename)/$(protoname).proto" ]; then \
		make genproto; \
	fi

	@if [ ! -f "$(servername)/$(modulename)/$(protoname)_dto.proto" ]; then \
		make gendto; \
	fi

	@if [ ! -f "$(servername)/$(modulename)/Makefile" ]; then \
		cp init-makefile $(servername)/$(modulename)/Makefile; \
	fi

check:
	@if [ -z "$(modulename)" ]; then \
		echo "modulename 参数不能为空"; \
		exit 1; \
	fi
	@if [ -z "$(protoname)" ]; then \
		echo "protoname 参数不能为空"; \
		exit 1; \
	fi
	@if [ -z "$(servername)" ]; then \
		echo "servername 参数不能为空"; \
		exit 1; \
	fi

genbase: check
	@mkdir -p $(servername)/$(modulename)
	@if [ ! -f "$(servername)/$(modulename)/$(modulename)_base.proto" ]; then \
		touch $(servername)/$(modulename)/$(modulename)_base.proto; \
		echo 'syntax = "proto3";' > $(servername)/$(modulename)/$(modulename)_base.proto; \
		echo "package $(modulename);" >> $(servername)/$(modulename)/$(modulename)_base.proto; \
		echo 'option go_package = "$(reponame)/v1/$(servername)/$(modulename)";' >> $(servername)/$(modulename)/$(modulename)_base.proto; \
		cat init-base.proto >> $(servername)/$(modulename)/$(modulename)_base.proto; \
	fi

gendto: check
	@mkdir -p $(servername)/$(modulename)
	@if [ ! -f "$(servername)/$(modulename)/$(protoname)_dto.proto" ]; then \
		touch $(servername)/$(modulename)/$(protoname)_dto.proto; \
		echo 'syntax = "proto3";' > $(servername)/$(modulename)/$(protoname)_dto.proto; \
		echo "package $(modulename);" >> $(servername)/$(modulename)/$(protoname)_dto.proto; \
		echo 'option go_package = "$(reponame)/v1/$(servername)/$(modulename)";' >> $(servername)/$(modulename)/$(protoname)_dto.proto; \
		echo 'import "base.proto";' >> $(servername)/$(modulename)/$(protoname)_dto.proto; \
	fi

genproto: check
	@mkdir -p $(servername)/$(modulename)
	@if [ ! -f "$(servername)/$(modulename)/$(protoname).proto" ]; then \
		touch $(servername)/$(modulename)/$(protoname).proto; \
		echo 'syntax = "proto3";' > $(servername)/$(modulename)/$(protoname).proto; \
		echo "package $(modulename);" >> $(servername)/$(modulename)/$(protoname).proto; \
		echo 'option go_package = "$(reponame)/v1/$(servername)/$(modulename)";' >> $(servername)/$(modulename)/$(protoname).proto; \
		echo 'import "$(protoname)_dto.proto";' >> $(servername)/$(modulename)/$(protoname).proto; \
	fi
