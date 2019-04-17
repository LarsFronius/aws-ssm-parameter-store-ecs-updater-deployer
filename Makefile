.PHONY: deps clean build

deps:
	go get -u ./...

clean: 
	rm -rf ./parameter-store-ecs-updater-deployer/parameter-store-ecs-updater-deployer
	
build:
	GOOS=linux GOARCH=amd64 go build -o parameter-store-ecs-updater-deployer/parameter-store-ecs-updater-deployer ./parameter-store-ecs-updater-deployer

zip: build
	zip -j ./parameter-store-ecs-updater-deployer/parameter-store-ecs-updater-deployer.zip parameter-store-ecs-updater-deployer/parameter-store-ecs-updater-deployer

test:
	go test ./...
