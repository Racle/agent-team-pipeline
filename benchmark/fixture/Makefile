.PHONY: test test-api test-web
test: test-api test-web

test-api:
	cd api && go vet ./... && go test ./...

test-web:
	cd web && npx tsc --noEmit && npx vitest run
