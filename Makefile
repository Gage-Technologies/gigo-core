ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
include .env

protos:
	# maybe don't install latest of these but for now it works
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install storj.io/drpc/cmd/protoc-gen-go-drpc@latest
	mkdir -p ${ROOT_DIR}/src/protos/ws
	cd ${ROOT_DIR}/src/ws-protos && git checkout master && git pull
	protoc --go_out=${ROOT_DIR}/src --go-drpc_out=${ROOT_DIR}/src --proto_path=${ROOT_DIR}/src/ws-protos ${ROOT_DIR}/src/ws-protos/**

docker:
	docker build \
		--build-arg "GH_BUILD_TOKEN=${GITHUB_BUILD_TOKEN}" \
		--build-arg "GH_USER_NAME=${GITHUB_USER_NAME}" \
		-t ${DOCKER_IMAGE} ${ROOT_DIR}/

docker-push:
	docker push ${DOCKER_IMAGE}

gen-dev-keys:
	openssl genpkey -algorithm RSA -out dev/data/minio/initData/keys/private.pem -pkeyopt rsa_keygen_bits:2048
	openssl rsa -in dev/data/minio/initData/keys/private.pem -pubout -out dev/data/minio/initData/keys/public.pem
