.PHONY: dev build test lint

dev:
	$(MAKE) -C server dev

build:
	$(MAKE) -C server build

test:
	$(MAKE) -C server test

lint:
	$(MAKE) -C server lint
