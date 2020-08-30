pretest:
	@docker run --rm -p 4100:4100 qhenkart/sqs-emulator 

test:
	@go test ./...

