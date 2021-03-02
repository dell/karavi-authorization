docker:
	-mkdir -p ./bin
	cp Dockerfile ./bin/Dockerfile
	CGO_ENABLED=0 go build -o ./bin/stress-tester main.go
	docker build -t 10.247.66.155:5000/stress-tester:aaron --build-arg APP=stress-tester ./bin/.
	docker push 10.247.66.155:5000/stress-tester:aaron 