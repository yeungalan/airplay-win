.PHONY: build frontend backend clean run test

build: frontend backend

frontend:
	cd frontend && npm install && npx next build
	rm -rf server/internal/frontend/dist
	cp -r frontend/out server/internal/frontend/dist

backend: frontend
	cd server && go build -o ../bin/airplay-server ./cmd/

run: build
	./bin/airplay-server $(ARGS)

test:
	cd server && go test ./... -v
	cd frontend && npx jest

clean:
	rm -rf bin/ server/internal/frontend/dist frontend/out frontend/.next
