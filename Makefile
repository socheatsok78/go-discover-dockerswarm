tidy:
	go mod tidy
build:
	@$(MAKE) -C cmd build
clean:
	@$(MAKE) -C cmd clean
