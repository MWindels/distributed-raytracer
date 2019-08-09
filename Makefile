build_comms:
	@protoc --go_out=plugins=grpc:. shared/comms/comms.proto

build_master_no_comms:
	@go build -o master.exe master/main.go master/registrar.go

build_worker_no_comms:
	@go build -o worker.exe worker/distributed/main.go

build_master: build_comms build_master_no_comms

build_worker: build_comms build_worker_no_comms

build_sequential:
	@go build -o sequential.exe worker/sequential/main.go